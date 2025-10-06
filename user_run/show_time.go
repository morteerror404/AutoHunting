package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"db_manager_main"
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

func main() {
	start := time.Now()
	metrics := Metrics{}

	// Carregar configurações
	config, err := loadConfig("env.json")
	if err != nil {
		fmt.Printf("Erro ao carregar env.json: %v\n", err)
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
	platform, err := selectPlatform()
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
func loadConfig(filename string) (Config, error) {
	var config Config
	data, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	return config, nil
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
		// Verificar se o arquivo contém erros (ex.: buscar "error" no conteúdo)
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
func selectPlatform() (string, error) {
	platforms := []string{"hackerone", "bugcrowd", "intigriti", "yeswehack"}
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

	switch input {
	case "1":
		return platforms[0], nil
	case "2":
		return platforms[1], nil
	case "3":
		return platforms[2], nil
	case "4":
		return platforms[3], nil
	case "0":
		return "", nil
	default:
		return "", fmt.Errorf("opção inválida")
	}
}

// showScopes consulta e exibe os escopos disponíveis para uma plataforma
func showScopes(platform string) error {
	// Conectar ao banco de dados
	db, err := db_manager_main.connectDB()
	if err != nil {
		return fmt.Errorf("erro ao conectar ao banco de dados: %w", err)
	}
	defer db.Close()

	// Consulta SQL para buscar escopos
	query := "SELECT scope FROM scopes WHERE platform = $1"
	rows, err := db.Query(query, platform)
	if err != nil {
		return fmt.Errorf("erro ao executar consulta de escopos para %s: %w", platform, err)
	}
	defer rows.Close()

	// Exibir resultados
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