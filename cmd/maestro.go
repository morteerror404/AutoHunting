package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"data/cleaner"
	"data/db"
	"data/runner"
	"utils"
)

type Config struct {
	APIRawResultsPath     string `json:"api_raw_results_path"`
	AIProcessedScopesPath string `json:"ai_processed_scopes_path"`
	WordlistDir           string `json:"wordlist_dir"`
}

type Commands struct {
	Nmap map[string]string `json:"nmap"`
	Ffuf map[string]string `json:"ffuf"`
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

type ExecutionLog struct {
	Timestamp time.Time
	Step      string
	Status    string
	Error     string
}

func main() {
	start := time.Now()
	logs := []ExecutionLog{}

	// Carregar configurações
	config, err := loadConfig()
	if err != nil {
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadConfig", Status: "Failed", Error: err.Error()})
		saveExecutionLog(logs)
		fmt.Printf("Erro ao carregar env.json: %v\n", err)
		os.Exit(1)
	}
	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadConfig", Status: "Success"})

	// Carregar comandos
	commands, err := loadCommands()
	if err != nil {
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadCommands", Status: "Failed", Error: err.Error()})
		saveExecutionLog(logs)
		fmt.Printf("Erro ao carregar commands.json: %v\n", err)
		os.Exit(1)
	}
	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadCommands", Status: "Success"})

	// Carregar tokens
	tokens, err := loadTokens()
	if err != nil {
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadTokens", Status: "Failed", Error: err.Error()})
		saveExecutionLog(logs)
		fmt.Printf("Erro ao carregar tokens.json: %v\n", err)
		os.Exit(1)
	}
	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadTokens", Status: "Success"})

	// Receber plataforma de show_time.go
	platform, err := loadSelectedPlatform()
	if err != nil {
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadSelectedPlatform", Status: "Failed", Error: err.Error()})
		saveExecutionLog(logs)
		fmt.Printf("Erro ao carregar plataforma selecionada: %v\n", err)
		os.Exit(1)
	}
	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadSelectedPlatform", Status: "Success"})

	// Executar request_api para coletar alvos
	fmt.Println("Coletando alvos via APIs de bug bounty...")
	if err := runRequestAPI(config.APIRawResultsPath, platform, tokens); err != nil {
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "RequestAPI", Status: "Failed", Error: err.Error()})
		saveExecutionLog(logs)
		fmt.Printf("Erro ao executar request_api: %v\n", err)
		os.Exit(1)
	}
	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "RequestAPI", Status: "Success"})

	// Executar runner para varreduras
	fmt.Println("Executando varreduras...")
	outDir := "output"
	for tool, args := range map[string]string{
		"nmap": commands.Nmap["nmap_slow"],
		"ffuf": commands.Ffuf["default"],
	} {
		if err := runRunner(config.APIRawResultsPath, args, outDir, tool); err != nil {
			logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: fmt.Sprintf("Runner_%s", tool), Status: "Failed", Error: err.Error()})
			fmt.Printf("Erro ao executar runner para %s: %v\n", tool, err)
			continue
		}
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: fmt.Sprintf("Runner_%s", tool), Status: "Success"})
	}

	// Executar cleaner para processar resultados
	fmt.Println("Limpando resultados...")
	files, err := os.ReadDir(outDir)
	if err != nil {
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "ReadOutputDir", Status: "Failed", Error: err.Error()})
		saveExecutionLog(logs)
		fmt.Printf("Erro ao ler diretório de saída: %v\n", err)
		os.Exit(1)
	}
	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "ReadOutputDir", Status: "Success"})

	for _, f := range files {
		if f.IsDir() || (!strings.HasPrefix(f.Name(), "nmap_") && !strings.HasPrefix(f.Name(), "ffuf_")) {
			continue
		}
		filename := filepath.Join(outDir, f.Name())
		template := "open_ports"
		if strings.HasPrefix(f.Name(), "ffuf_") {
			template = "endpoints"
		}
		if err := cleaner.CleanFile(filename, template); err != nil {
			logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: fmt.Sprintf("CleanFile_%s", f.Name()), Status: "Failed", Error: err.Error()})
			fmt.Printf("Erro ao limpar arquivo %s: %v\n", filename, err)
			continue
		}
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: fmt.Sprintf("CleanFile_%s", f.Name()), Status: "Success"})
	}

	// Executar db_manager para inserir no banco
	fmt.Println("Inserindo resultados no banco de dados...")
	dbConn, err := db.ConnectDB()
	if err != nil {
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "ConnectDB", Status: "Failed", Error: err.Error()})
		saveExecutionLog(logs)
		fmt.Printf("Erro ao conectar ao DB: %v\n", err)
		os.Exit(1)
	}
	defer dbConn.Close()
	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "ConnectDB", Status: "Success"})

	for _, f := range files {
		if f.IsDir() || !strings.Contains(f.Name(), "_clean_") {
			continue
		}
		filename := filepath.Join(outDir, f.Name())
		if err := db.ProcessCleanFile(filename, dbConn); err != nil {
			logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: fmt.Sprintf("ProcessCleanFile_%s", f.Name()), Status: "Failed", Error: err.Error()})
			fmt.Printf("Erro ao processar arquivo limpo %s: %v\n", filename, err)
			continue
		}
		logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: fmt.Sprintf("ProcessCleanFile_%s", f.Name()), Status: "Success"})
	}

	logs = append(logs, ExecutionLog{Timestamp: time.Now(), Step: "ExecutionCompleted", Status: "Success"})
	saveExecutionLog(logs)
	fmt.Printf("Processamento concluído em %v!\n", time.Since(start))
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

// loadTokens carrega o arquivo tokens.json
func loadTokens() (Tokens, error) {
	var tokens Tokens
	if err := utils.LoadJSON("tokens.json", &tokens); err != nil {
		return tokens, err
	}
	return tokens, nil
}

// loadSelectedPlatform carrega a plataforma selecionada
func loadSelectedPlatform() (string, error) {
	var selection struct {
		Platform string `json:"platform"`
	}
	if err := utils.LoadJSON("selected_platform.json", &selection); err != nil {
		return "", fmt.Errorf("erro ao carregar selected_platform.json: %w", err)
	}
	return selection.Platform, nil
}

// runRequestAPI executa a coleta de alvos via request_api
func runRequestAPI(outFile, platform string, tokens Tokens) error {
	fmt.Printf("Executando request_api para plataforma %s, salvando em %s\n", platform, outFile)
	args := []string{"-out", outFile}
	if platform != "" {
		switch platform {
		case "hackerone":
			if tokens.HackerOne.ApiKey != "" {
				args = append(args, "-h1-user", tokens.HackerOne.Username, "-h1-key", tokens.HackerOne.ApiKey)
			}
		case "bugcrowd":
			if tokens.Bugcrowd.Token != "" {
				args = append(args, "-bc-token", tokens.Bugcrowd.Token)
			}
		case "intigriti":
			if tokens.Intigriti.Token != "" {
				args = append(args, "-int-token", tokens.Intigriti.Token)
			}
		case "yeswehack":
			if tokens.YesWeHack.Token != "" {
				args = append(args, "-ywh-token", tokens.YesWeHack.Token)
			}
		}
	} else {
		// Adicionar todas as plataformas se nenhuma for especificada
		if tokens.HackerOne.ApiKey != "" {
			args = append(args, "-h1-user", tokens.HackerOne.Username, "-h1-key", tokens.HackerOne.ApiKey)
		}
		if tokens.Bugcrowd.Token != "" {
			args = append(args, "-bc-token", tokens.Bugcrowd.Token)
		}
		if tokens.Intigriti.Token != "" {
			args = append(args, "-int-token", tokens.Intigriti.Token)
		}
		if tokens.YesWeHack.Token != "" {
			args = append(args, "-ywh-token", tokens.YesWeHack.Token)
		}
	}
package api

func Main(args []string) error {
	// Sua lógica aqui
	if err != nil {
		return err // Retorna o erro
	}
	return nil // Retorna nil se tudo correr bem
}
	if err := api.Main(args); err != nil {
		return fmt.Errorf("erro ao executar api.Main: %w", err)
	}
}

// runRunner executa varreduras com runner
func runRunner(targetsFile, args, outDir, tool string) error {
	fmt.Printf("Executando runner com alvos de %s, args %s, ferramenta %s, saída em %s\n", targetsFile, args, tool, outDir)
	return runner.Run(targetsFile, args, outDir)
}

// saveExecutionLog salva o log de execução em maestro_execution.log
func saveExecutionLog(logs []ExecutionLog) {
	f, err := os.OpenFile("maestro_execution.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Erro ao abrir maestro_execution.log: %v\n", err)
		return
	}
	defer f.Close()

	writer := bufio.NewWriter(f)
	for _, log := range logs {
		line := fmt.Sprintf("[%s] Step: %s, Status: %s", log.Timestamp.Format(time.RFC3339), log.Step, log.Status)
		if log.Error != "" {
			line += fmt.Sprintf(", Error: %s", log.Error)
		}
		line += "\n"
		if _, err := writer.WriteString(line); err != nil {
			fmt.Printf("Erro ao escrever no log: %v\n", err)
		}
	}
	writer.Flush()
}
