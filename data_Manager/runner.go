package runner

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"io/ioutil"
)

// Estruturas mínimas para parse do XML do nmap (apenas campos que usaremos)
type NmapRun struct {
	XMLName xml.Name `xml:"nmaprun"`
	Hosts   []Host   `xml:"host"`
}

type Host struct {
	Addresses []Address `xml:"address"`
	Ports     Ports     `xml:"ports"`
	Status    Status    `xml:"status"`
}

type Address struct {
	Addr string `xml:"addr,attr"`
	Type string `xml:"addrtype,attr"`
}

type Status struct {
	State string `xml:"state,attr"`
}

type Ports struct {
	Port []Port `xml:"port"`
}

type Port struct {
	Protocol string   `xml:"protocol,attr"`
	PortId   int      `xml:"portid,attr"`
	State    PortState `xml:"state"`
	Service  Service   `xml:"service"`
}

type PortState struct {
	State string `xml:"state,attr"`
}

type Service struct {
	Name string `xml:"name,attr"`
	Product string `xml:"product,attr,omitempty"`
	Version string `xml:"version,attr,omitempty"`
}

func runner() {
	// flags / parâmetros CLI
	targetsFlag := flag.String("targets", "", "Lista de targets separados por vírgula (ex: 10.0.0.1,example.com)")
	portsFlag := flag.String("ports", "1-1000", "Portas para passar ao nmap (ex: 22,80,443 ou 1-1000)")
	nmapArgs := flag.String("nmap-args", "-sV -Pn", "Argumentos adicionais para nmap (entre aspas)")
	concurrency := flag.Int("concurrency", 5, "Número de workers concorrentes")
	timeout := flag.Int("timeout", 60, "Timeout em segundos por execução de ferramenta")
	outDir := flag.String("out-dir", "output", "Pasta para salvar outputs (XML e relatórios)")

	flag.Parse()

	if *targetsFlag == "" {
		fmt.Println("Use --targets para indicar alvos (ex: --targets=example.com,10.0.0.5)")
		os.Exit(1)
	}

	// prepara targets slice
	targets := strings.Split(*targetsFlag, ",")
	for i := range targets {
		targets[i] = strings.TrimSpace(targets[i])
	}

	// cria pasta de saída
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Printf("Erro criando out dir: %v\n", err)
		os.Exit(1)
	}

	// worker pool
	tasks := make(chan string, len(targets))
	results := make(chan string, len(targets))
	var wg sync.WaitGroup

	// função worker
	worker := func(id int) {
		for t := range tasks {
			fmt.Printf("[worker %d] processando %s\n", id, t)
			// compõe comando nmap (usa -oX - para voltar XML no stdout)
			args := []string{}
			// split de nmapArgs em array simples (nota: não trata quoting complexo)
			if *nmapArgs != "" {
				extra := strings.Fields(*nmapArgs)
				args = append(args, extra...)
			}
			args = append(args, "-p", *portsFlag, "-oX", "-", t)

			// executa nmap com timeout
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
			defer cancel()
			out, err := runCommandContext(ctx, "nmap", args...)
			if err != nil {
				fmt.Printf("[worker %d] erro nmap %s: %v\n", id, t, err)
				results <- fmt.Sprintf("%s: error: %v", t, err)
				continue
			}

			// salva XML bruto (opcional)
			xmlPath := filepath.Join(*outDir, fmt.Sprintf("%s_nmap.xml", sanitizeFilename(t)))
			if err := ioutil.WriteFile(xmlPath, out, 0o644); err != nil {
				fmt.Printf("[worker %d] erro salvando xml: %v\n", id, err)
			}

			// parse do XML e geração de relatório simples
			report := parseNmapXML(out, t)
			reportPath := filepath.Join(*outDir, fmt.Sprintf("%s_report.txt", sanitizeFilename(t)))
			if err := ioutil.WriteFile(reportPath, []byte(report), 0o644); err != nil {
				fmt.Printf("[worker %d] erro salvando report: %v\n", id, err)
			}
			results <- fmt.Sprintf("%s: done (xml=%s, report=%s)", t, xmlPath, reportPath)
		}
		wg.Done()
	}

	// inicia workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker(i + 1)
	}

	// envia tasks
	for _, t := range targets {
		tasks <- t
	}
	close(tasks)

	// espera e coleta resultados
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		fmt.Println("[result]", r)
	}

	fmt.Println("All done.")
}

// runCommandContext roda um comando com contexto (timeout) e retorna stdout bytes + erro
func runCommandContext(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	// opcional: redirecionar stderr para facilitar debug
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// parseNmapXML faz um parse simples do XML do nmap e retorna relatório legível
func parseNmapXML(xmlBytes []byte, target string) string {
	var n NmapRun
	if err := xml.Unmarshal(xmlBytes, &n); err != nil {
		return fmt.Sprintf("Erro parse XML: %v\n--- RAW XML ---\n%s", err, string(xmlBytes[:min(len(xmlBytes), 2000)]))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Relatório nmap - alvo: %s\n", target))
	b.WriteString("====================================\n")
	if len(n.Hosts) == 0 {
		b.WriteString("Nenhum host retornado pelo nmap (possível filtro/host down).\n")
		return b.String()
	}
	for _, h := range n.Hosts {
		ip := "unknown"
		for _, a := range h.Addresses {
			if a.Type == "ipv4" || a.Type == "ipv6" {
				ip = a.Addr
				break
			}
			// fallback pega primeiro address
			if ip == "unknown" {
				ip = a.Addr
			}
		}
		b.WriteString(fmt.Sprintf("Host: %s (status=%s)\n", ip, h.Status.State))
		if len(h.Ports.Port) == 0 {
			b.WriteString("  sem portas reportadas\n")
		} else {
			for _, p := range h.Ports.Port {
				b.WriteString(fmt.Sprintf("  - %d/%s -> %s (service=%s %s)\n",
					p.PortId, p.Protocol, p.State.State, p.Service.Name, strings.TrimSpace(p.Service.Product+" "+p.Service.Version)))
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

// sanitizeFilename transforma string em nome de arquivo safe
func sanitizeFilename(s string) string {
	repl := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return repl.Replace(s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

runner();