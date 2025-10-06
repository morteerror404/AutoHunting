package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"data_Manager/cleaner_main"
	"data_Manager/db_manager_main"
	"data_Manager/runner"
	"config/request_api_main"
	"utils"
)

type Config struct {
	APIRawResultsPath    string `json:"api_raw_results_path"`
	AIProcessedScopesPath string `json:"ai_processed_scopes_path"`
	WordlistDir          string `json:"wordlist_dir"`
}

type Commands struct {
	Nmap map[string]string `json:"nmap"`
	Ffuf map[string]string `json:"ffuf"`
}

func main() {
	// 1. Carregar configurações
	config, err := loadConfig()
	if err != nil {
		fmt.Printf("Erro ao carregar env.json: %v\n", err)
		os.Exit(1)
	}

	commands, err := loadCommands()
	if err != nil {
		fmt.Printf("Erro ao carregar commands.json: %v\n", err)
		os.Exit(1)
	}

	// 2. Executar request_api para coletar alvos
	fmt.Println("Coletando alvos via APIs de bug bounty...")
	if err := runRequestAPI(config.APIRawResultsPath); err != nil {
		fmt.Printf("Erro ao executar request_api: %v\n", err)
		os.Exit(1)
	}

	// 3. Executar runner para varredura
	fmt.Println("Executando varreduras com nmap...")
	nmapArgs := commands.Nmap["nmap_slow"]
	targetsFile := config.APIRawResultsPath
	outDir := "output"
	if err := runRunner(targetsFile, nmapArgs, outDir); err != nil {
		fmt.Printf("Erro ao executar runner: %v\n", err)
		os.Exit(1)
	}

	// 4. Executar cleaner para processar resultados
	fmt.Println("Limpando resultados...")
	files, err := os.ReadDir(outDir)
	if err != nil {
		fmt.Printf("Erro ao ler diretório de saída: %v\n", err)
		os.Exit(1)
	}
	for _, f := range files {
		if f.IsDir() || !strings.HasPrefix(f.Name(), "nmap_") {
			continue
		}
		filename := filepath.Join(outDir, f.Name())
		if err := cleaner_main.cleanFile(filename, "open_ports"); err != nil {
			fmt.Printf("Erro ao limpar arquivo %s: %v\n", filename, err)
		}
	}

	// 5. Executar db_manager para inserir no banco
	fmt.Println("Inserindo resultados no banco de dados...")
	for _, f := range files {
		if f.IsDir() || !strings.Contains(f.Name(), "_clean_") {
			continue
		}
		filename := filepath.Join(outDir, f.Name())
		db, err := db_manager_main.connectDB()
		if err != nil {
			fmt.Printf("Erro ao conectar ao DB: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()
		if err := db_manager_main.processCleanFile(filename, db); err != nil {
			fmt.Printf("Erro ao processar arquivo limpo %s: %v\n", filename, err)
		}
	}

	fmt.Println("Processamento concluído!")
}

// loadConfig carrega o arquivo env.json
func loadConfig() (Config, error) {
	var config Config
	if err := utils.LoadJSON("env.json", &config); err != nil {
		return config, err
	}
	return config, nil
}

// loadCommands carrega o arquivo commands.json
func loadCommands() (Commands, error) {
	var commands Commands
	if err := utils.LoadJSON("commands.json", &commands); err != nil {
		return commands, err
	}
	return commands, nil
}

// runRequestAPI executa a coleta de alvos via request_api
func runRequestAPI(outFile string) error {
	fmt.Printf("Executando request_api, salvando em %s\n", outFile)
	return nil // Substitua por chamada real se necessário
}

// runRunner executa varreduras com runner
func runRunner(targetsFile, nmapArgs, outDir string) error {
	fmt.Printf("Executando runner com alvos de %s e args %s\n", targetsFile, nmapArgs)
	return nil // Substitua por chamada real se necessário
}