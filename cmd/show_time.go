package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"AutoHunting/data/db"
	"AutoHunting/utils"
)

type Tokens struct {
	HackerOne struct {
		Username string `json:"username"`
		ApiKey   string `json:"api_key"`
	} `json:"hackerone"`
	Bugcrowd struct {
		Token string `json:"token"`
	} `json:"bugcrowd"`
	Intigriti struct {
		Token string `json:"token"`
	} `json:"intigriti"`
	YesWeHack struct {
		Token string `json:"token"`
	} `json:"yeswehack"`
}

func main() {
	for {
		showMainMenu()
	}
}

func showMainMenu() {
	fmt.Println("\n===== AutoHunting - Menu Principal =====")
	fmt.Println("1) Iniciar Caçada (Hunt)")
	fmt.Println("2) Consultar Banco de Dados")
	fmt.Println("3) Verificar Status das APIs")
	fmt.Println("0) Sair")
	fmt.Print("Escolha uma opção: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(input)

	switch choice {
	case "1":
		handleHuntMenu()
	case "2":
		handleDBMenu()
	case "3":
		handleAPIStatusMenu()
	case "0":
		fmt.Println("Saindo...")
		os.Exit(0)
	default:
		fmt.Println("Opção inválida.")
	}
}


	// Carregar tokens
func handleHuntMenu() {
	tokens, err := loadTokens()
	if err != nil {
		fmt.Printf("Erro ao carregar tokens: %v\n", err)
		return
	}

	platform, err := selectPlatform(tokens)
	if err != nil {
		fmt.Printf("Erro: %v\n", err)
		return
	}
	if platform == "" {
		return // Usuário cancelou
	}

	// Cria a ordem e dispara o maestro
	task := "fullHunt" // Deve corresponder à chave em order-templates.json
	if err := utils.CreateExecutionOrder(task, platform); err != nil {
		fmt.Printf("Erro ao criar ordem de execução para o maestro: %v\n", err)
		return
	}
	triggerMaestro()
}

func handleDBMenu() {
	// Lógica para o menu do banco de dados
	fmt.Println("\n--- Menu Banco de Dados ---")
	fmt.Println("1) Listar escopos de uma plataforma")
	fmt.Println("0) Voltar")
	fmt.Print("Escolha: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(input)

	if choice == "1" {
		tokens, err := loadTokens()
		if err != nil {
			fmt.Printf("Erro ao carregar tokens: %v\n", err)
			return
		}
		platform, err := selectPlatform(tokens)
		if err != nil {
			fmt.Printf("Erro ao selecionar plataforma: %v\n", err)
			return
		}
		if platform != "" {
			if err := showScopes(platform); err != nil {
				fmt.Printf("Erro ao buscar escopos: %v\n", err)
			}
	}
}

func handleAPIStatusMenu() {
	tokens, err := loadTokens()
	if err != nil {
		fmt.Printf("Erro ao carregar tokens: %v\n", err)
		return
	}
	platform, err := selectPlatform(tokens)
	if err != nil {
		fmt.Printf("Erro ao selecionar plataforma: %v\n", err)
		return
	}
	if platform != "" {
		fmt.Println("Verificando status...")
		if err = checkAPIStatus(platform); err != nil {
			fmt.Printf("Erro ao verificar API: %v\n", err)
		} else {
			fmt.Printf("API da plataforma '%s' parece estar respondendo corretamente.\n", platform)
		}
	}
}

// loadTokens carrega o arquivo tokens.json
func loadTokens() (Tokens, error) {
	var tokens Tokens
	if err := utils.LoadJSON("tokens.json", &tokens); err != nil {
		return tokens, err
	}
	return tokens, nil
}

// selectPlatform exibe um menu interativo para selecionar a plataforma
func selectPlatform(tokens Tokens) (string, error) {
	platforms := []string{}
	if tokens.HackerOne.ApiKey != "" {
		platforms = append(platforms, "hackerone")
	}
	if tokens.Bugcrowd.Token != "" {
		platforms = append(platforms, "bugcrowd")
	}

	if tokens.Intigriti.Token != "" {
		platforms = append(platforms, "intigriti")
	}
	if tokens.YesWeHack.Token != "" {
		platforms = append(platforms, "yeswehack")
	}

	if len(platforms) == 0 {
		return "", fmt.Errorf("nenhuma plataforma configurada em tokens.json")
	}

	fmt.Println("\n=== Selecionar Plataforma ===")
	for i, p := range platforms {
		fmt.Printf("%d) %s\n", i+1, p)
	}
	fmt.Println("0) Cancelar")
	fmt.Print("Escolha uma opção: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("erro ao ler entrada: %w", err)
	}
	input = strings.TrimSpace(input)

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 0 || choice > len(platforms) {
		return "", fmt.Errorf("opção inválida")
	}
	if choice == 0 {
		return "", nil
	}
	return platforms[choice-1], nil
}

// checkAPIStatus testa a conexão com a API de uma plataforma
func checkAPIStatus(platform string) error {
	switch platform {
	case "hackerone":
		fmt.Println("Testando a conexão com a API do HackerOne...")
		url := "https://api.hackerone.com/reports" // Use um endpoint público e leve
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("erro ao conectar à API do HackerOne: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API do HackerOne retornou status code: %d", resp.StatusCode)
		}
		return nil
	case "bugcrowd":
		fmt.Println("Testando a conexão com a API do Bugcrowd...")
		url := "https://api.bugcrowd.com/programs" // Endpoint público
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("erro ao conectar à API do Bugcrowd: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API do Bugcrowd retornou status code: %d", resp.StatusCode)
		}
		return nil

	case "intigriti":
		fmt.Println("Testando a conexão com a API do Intigriti...")
		url := "https://api.intigriti.com/core/v1/programs" // Endpoint público
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("erro ao conectar à API do Intigriti: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API do Intigriti retornou status code: %d", resp.StatusCode)
		}
		return nil
	case "yeswehack":
		fmt.Println("Testando a conexão com a API do YesWeHack...")
		url := "https://api.yeswehack.com/api/v1/programs"
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("erro ao conectar à API do YesWeHack: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API do YesWeHack retornou status code: %d", resp.StatusCode)
		}

		return nil

	}
	return fmt.Errorf("plataforma %s não suportada", platform)
}

// showScopes consulta e exibe os escopos disponíveis para uma plataforma
func showScopes(platform string) error {
	db, err := db.ConnectDB()
	if err != nil {
		return fmt.Errorf("erro ao conectar ao banco de dados: %w", err)
	}
	defer db.Close()

	query := "SELECT scope FROM scopes WHERE platform = $1"
	rows, err := db.Query(query, platform)
	if err != nil {
		return fmt.Errorf("erro ao executar consulta de escopos para %s: %w", platform, err)
	}
	defer rows.Close()

	fmt.Printf("\n=== Escopos disponíveis para %s ===\n", platform)
	count := 0
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return fmt.Errorf("erro ao ler escopo: %w", err)
		}
		fmt.Printf("- %s\n", scope)
		count++
	}

	if count == 0 {
		fmt.Printf("Nenhum escopo encontrado para %s.\n", platform)
	} else {
		fmt.Printf("Total de escopos encontrados: %d\n", count)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("erro ao iterar resultados: %w", err)
	}

	return nil
}

// triggerMaestro escreve a ordem e executa o maestro, monitorando o log.
func triggerMaestro() {
	// 1. Executar o maestro em um novo processo
	fmt.Println("\n[+] Disparando o maestro... Acompanhe o progresso abaixo.")
	cmd := exec.Command("go", "run", "./cmd/maestro")
	cmd.Stderr = os.Stderr // Redireciona o Stderr do maestro para o Stderr do show_time

	// 2. Iniciar o monitoramento do log em uma goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan bool)

	go func() {
		defer wg.Done()
		// Carrega env.json para encontrar o caminho do log
		var envConfig struct {
			Archives struct {
				LogDir string `json:"log_dir"`
			} `json:"archives"`
		}
		if err := utils.LoadJSON("env.json", &envConfig); err != nil {
			fmt.Printf("\n[ERROR] Não foi possível encontrar o diretório de log do maestro: %v\n", err)
			return
		}
		logPath := filepath.Join(envConfig.Archives.LogDir, "maestro_execution.log")
		tailLogFile(logPath, done)
	}()

	// Inicia o comando e espera ele terminar
	if err := cmd.Start(); err != nil {
		fmt.Printf("Erro ao iniciar o maestro: %v\n", err)
		close(done)
		return
	}

	err = cmd.Wait()
	close(done) // Sinaliza para a goroutine de tail parar
	wg.Wait()   // Espera a goroutine de tail terminar

	if err != nil {
		fmt.Printf("\n[-] Maestro finalizou com erro.\n")
	} else {
		fmt.Println("\n[+] Maestro concluiu a execução com sucesso.")
	}
}

// tailLogFile monitora um arquivo de log e imprime novas linhas.
func tailLogFile(filepath string, done <-chan bool) {
	// Implementação simplificada de 'tail -f'
	// Em um cenário real, usar uma biblioteca como 'github.com/hpcloud/tail' seria melhor.
	f, err := os.Open(filepath)
	if err != nil {
		// O arquivo pode não existir ainda, espera um pouco.
		time.Sleep(500 * time.Millisecond)
		tailLogFile(filepath, done)
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		select {
		case <-done:
			return
		default:
			line, err := reader.ReadString('\n')
			if err == nil {
				fmt.Print("[Maestro] ", line)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}
