package runner

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// NmapRun e estruturas relacionadas (mantidas do original)
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
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr,omitempty"`
	Version string `xml:"version,attr,omitempty"`
}

// Run executa varreduras com a ferramenta especificada (nmap ou ffuf)
func Run(targetsFile, args, outDir string) error {
	// Ler alvos do arquivo
	data, err := os.ReadFile(targetsFile)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo de alvos %s: %w", targetsFile, err)
	}
	targets := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := range targets {
		targets[i] = strings.TrimSpace(targets[i])
	}

	// Criar pasta de saída
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de saída %s: %w", outDir, err)
	}

	// Configurar worker pool
	tasks := make(chan string, len(targets))
	results := make(chan string, len(targets))
	var wg sync.WaitGroup

	// Determinar ferramenta com base nos argumentos
	tool := "nmap"
	if strings.Contains(args, "ffuf") {
		tool = "ffuf"
	}

	// Worker function
	worker := func(id int) {
		defer wg.Done()
		for target := range tasks {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			filename := fmt.Sprintf("%s_%s_%s.txt", tool, sanitizeFilename(target), time.Now().Format("20060102150405"))
			outputPath := filepath.Join(outDir, filename)

			var cmdArgs []string
			if tool == "nmap" {
				cmdArgs = append(strings.Fields(args), "-oX", outputPath, target)
			} else if tool == "ffuf" {
				wordlist := strings.Replace(args, "FUZZ", target, 1)
				cmdArgs = append(strings.Fields(wordlist), "-o", outputPath)
			}

			output, err := runCommandContext(ctx, tool, cmdArgs...)
			if err != nil {
				results <- fmt.Sprintf("Worker %d: erro ao escanear %s com %s: %v", id, target, tool, err)
				continue
			}

			var report string
			if tool == "nmap" {
				report = parseNmapXML(output, target)
			} else {
				report = string(output) // Para ffuf, usar saída bruta
			}
			results <- fmt.Sprintf("Worker %d: %s\n%s", id, target, report)

			// Salvar saída bruta
			if err := os.WriteFile(outputPath, output, 0644); err != nil {
				results <- fmt.Sprintf("Worker %d: erro ao salvar saída para %s: %v", id, outputPath, err)
			}
		}
	}

	// Iniciar workers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go worker(i + 1)
	}

	// Enviar tarefas
	for _, t := range targets {
		tasks <- t
	}
	close(tasks)

	// Esperar e coletar resultados
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		fmt.Println("[result]", r)
	}

	return nil
}

// runCommandContext executa um comando com timeout
func runCommandContext(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// parseNmapXML faz parse do XML do nmap
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

// sanitizeFilename transforma string em nome de arquivo seguro
func sanitizeFilename(s string) string {
	repl := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return repl.Replace(s)
}

// min retorna o menor valor entre dois inteiros
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}