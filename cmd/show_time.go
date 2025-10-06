package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/usuario/bug-hunt/data/db"
	"github.com/usuario/bug-hunt/utils"
)

type Metrics struct {
	TotalFilesProcessed int
	TotalErrors         int
	TotalTime           time.Duration
}

type Config struct {
	APIRawResultsPath    string `json:"api_raw_results_path"`
	AIProcessedScopesPath string `json:"ai_processed_scopes_path"`
	WordlistDir          string `json:"wordlist_dir"`
}

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
	start := time.Now()
	metrics := Metrics{}

	// Carregar configurações
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Erro ao carregar env.json: %v\n", err)
		os.Exit(1)
	}

	// Carregar tokens
	tokens, err := loadTokens()
	if err != nil {
		fmt.Printf("Erro ao carregar tokens.json: %v\n", err)
		os.Exit(1)
	}

	// Monitorar diretório de saída (output)
	outputDir := "output"
	metrics, err = monitorDirectory(outputDir, metrics)
	if err != nil {
		fmt.Printf("Erro ao monitorar diretório %s: %v\n", outputDir, err)
	}

	// Monitorar diretório de resultados da API
	apiDir := filepath.Dir(config.APIRawResultsPath)
	metrics, err = monitorDirectory(apiDir, metrics)
	if err != nil {
		fmt.Printf("Erro ao monitorar diretório %s: %v\n", apiDir, err)
	}

	// Exibir menu para selecionar plataforma e mostrar escopos
	platform, err := selectPlatform(tokens)
	if err != nil {
		fmt.Printf("Erro ao selecionar plataforma: %v\n", err)
		metrics.TotalErrors++
	} else if platform != "" {
		if err := showScopes(platform); err != nil {
			fmt.Printf("Erro ao exibir escopos: %v\n", err)
			metrics.TotalErrors++
		}
	}

	// Exibir resumo
	fmt.Println("\n=== Resumo de Execução ===")
	fmt.Printf("Arquivos processados: %d\n", metrics.TotalFilesProcessed)
	fmt.Printf("Erros encontrados: %d\n", metrics.TotalErrors)
	fmt.Printf("Tempo total: %v\n", time.Since(start))
}

// loadConfig carrega o arquivo env.json
func loadConfig() (Config, error) {
	var config Config
	if err := utils.LoadJSON("env.json", &config); err != nil {
		return config, err
	}
	return config, nil
}

// loadTokens carrega o arquivo tokens.json
func loadTokens() (Tokens, error) {
	var tokens Tokens
	if err := utils.LoadJSON("tokens.json", &tokens); err != nil {
		return tokens, err
	}
	return tokens, nil
}

// monitorDirectory analisa arquivos em um diretório e atualiza métricas
func monitorDirectory(dir string, metrics Metrics) (Metrics, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		metrics.TotalErrors++
		return metrics, err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		metrics.TotalFilesProcessed++
		filePath := filepath.Join(dir, f.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			metrics.TotalErrors++
			continue
		}
		if strings.Contains(string(data), "error") || strings.Contains(string(data), "Erro") {
			metrics.TotalErrors++
		}
	}

	return metrics, nil
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