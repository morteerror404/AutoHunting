package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/morteerror404/AutoHunting/api"
	"github.com/morteerror404/AutoHunting/data/cleaner"
	"github.com/morteerror404/AutoHunting/data/db"
	"github.com/morteerror404/AutoHunting/data/runner"
	"github.com/morteerror404/AutoHunting/utils"
)

// MaestroContext contém todas as configurações e estado para uma execução do maestro.
type MaestroContext struct {
	Start    time.Time
	Logs     []ExecutionLog
	Config   *Config
	Commands *Commands
	Tokens   *Tokens
	Order    *MaestroOrder
	LogFile  *os.File
}

type Config struct {
	Path struct {
		APIDirtResultsPath string `json:"api_dirt_results_path"`
		ToolDirtDir        string `json:"tool_dirt_dir"`
		ToolCleanedDir     string `json:"tool_cleaned_dir"`
	} `json:"path"`
	Archives struct {
		MaestroExecOrder string `json:"maestro_exec_order"`
		LogDir           string `json:"log_dir"`
	} `json:"archives"`
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
	Timestamp time.Time `json:"timestamp"`
	Step      string    `json:"step"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
}

type MaestroOrder struct {
	Platform string              `json:"platform"`
	Task     string              `json:"task"`
	Steps    []utils.MaestroTask `json:"steps"`
}

func main() {
	if err := runMaestro(); err != nil {
		// O erro já foi logado dentro de runMaestro, apenas saímos com status de erro.
		os.Exit(1)
	}
	fmt.Println("Maestro concluiu a execução com sucesso.")
}

func runMaestro() error {
	ctx := &MaestroContext{
		Start: time.Now(),
		Logs:  []ExecutionLog{},
	}

	// 1. Configurar Log
	if err := setupLogging(ctx); err != nil {
		fmt.Printf("Erro crítico ao configurar o log: %v\n", err)
		return err // Erro fatal, não podemos continuar sem log.
	}
	defer ctx.LogFile.Close()
	defer saveExecutionLog(ctx) // Garante que os logs sejam salvos no final.

	// 2. Carregar todas as configurações
	if err := loadAllConfigs(ctx); err != nil {
		logAndExit(ctx, "LoadAllConfigs", err)
		return err
	}

	// 3. Iterar e executar cada passo da ordem
	for _, step := range ctx.Order.Steps {
		log.Printf("Iniciando passo: %s", step.Description)
		var stepErr error

		switch step.Step {
		case "RequestAPI":
			stepErr = stepRequestAPI(ctx)
		case "RunScanners":
			stepErr = stepRunScanners(ctx)
		case "CleanResults":
			stepErr = stepCleanResults(ctx)
		case "StoreResults":
			stepErr = stepStoreResults(ctx)
		default:
			stepErr = fmt.Errorf("passo desconhecido na ordem de execução: %s", step.Step)
		}

		if stepErr != nil {
			logAndExit(ctx, step.Step, stepErr)
			return stepErr // Interrompe a execução em caso de erro
		}
		ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: step.Step, Status: "Success"})
		log.Printf("Passo '%s' concluído com sucesso.", step.Step)
	}

	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "ExecutionCompleted", Status: "Success"})
	log.Printf("Processamento concluído em %v!\n", time.Since(ctx.Start))
	return nil
}

func stepRequestAPI(ctx *MaestroContext) error {
	return api.RunRequestAPI(ctx.Config.Path.APIDirtResultsPath, ctx.Order.Platform, *ctx.Tokens)
}

func stepRunScanners(ctx *MaestroContext) error {
	outDir := ctx.Config.Path.ToolDirtDir
	for tool, args := range map[string]string{
		"nmap": ctx.Commands.Nmap["nmap_slow"],
		"ffuf": ctx.Commands.Ffuf["default"],
	} {
		if err := runner.Run(ctx.Config.Path.APIDirtResultsPath, args, outDir, tool); err != nil {
			return fmt.Errorf("erro no runner para %s: %w", tool, err)
		}
	}
	return nil
}

func stepCleanResults(ctx *MaestroContext) error {
	outDir := ctx.Config.Path.ToolDirtDir
	files, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("falha ao ler diretório de resultados brutos '%s': %w", outDir, err)
	}

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
			// Não paramos o processo inteiro se um arquivo falhar, apenas logamos.
			log.Printf("AVISO: Falha ao limpar o arquivo %s: %v", filename, err)
			continue
		}
	}
	return nil
}

func stepStoreResults(ctx *MaestroContext) error {
	dbConn, err := db.ConnectDB()
	if err != nil {
		return fmt.Errorf("erro ao conectar ao DB: %w", err)
	}
	defer dbConn.Close()

	cleanDir := ctx.Config.Path.ToolCleanedDir // Usar o caminho correto do env.json
	files, err := os.ReadDir(cleanDir)
	if err != nil {
		return fmt.Errorf("falha ao ler diretório de resultados limpos '%s': %w", cleanDir, err)
	}

	for _, f := range files {
		// Apenas processar arquivos que contêm "_clean_" no nome, ignorando diretórios.
		if f.IsDir() || !strings.Contains(f.Name(), "_clean_") {
			continue
		}
		filename := filepath.Join(cleanDir, f.Name())
		if err := db.ProcessCleanFile(filename, dbConn); err != nil {
			// Não paramos o processo inteiro se um arquivo falhar, apenas logamos.
			log.Printf("AVISO: Falha ao processar o arquivo limpo %s para o DB: %v", filename, err)
			continue
		}
	}
	return nil
}

func setupLogging(ctx *MaestroContext) error {
	// Carrega apenas o env para saber onde salvar o log.
	var envConfig Config
	if err := utils.LoadJSON("env.json", &envConfig); err != nil {
		return fmt.Errorf("falha ao carregar env.json para configurar log: %w", err)
	}

	logDir := envConfig.Archives.LogDir
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório de log '%s': %w", logDir, err)
	}

	logPath := filepath.Join(logDir, "maestro_execution.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("falha ao abrir arquivo de log '%s': %w", logPath, err)
	}
	ctx.LogFile = f

	// Redireciona a saída padrão do log para o arquivo e para o console
	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.SetFlags(log.Ldate | log.Ltime)

	log.Println("Logging configurado com sucesso.")
	return nil
}

func loadAllConfigs(ctx *MaestroContext) error {
	var config Config
	if err := utils.LoadJSON("env.json", &config); err != nil {
		return fmt.Errorf("erro ao carregar env.json: %w", err)
	}
	ctx.Config = &config
	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadConfig", Status: "Success"})

	var commands Commands
	if err := utils.LoadJSON("commands.json", &commands); err != nil {
		return fmt.Errorf("erro ao carregar commands.json: %w", err)
	}
	ctx.Commands = &commands
	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadCommands", Status: "Success"})

	var tokens Tokens
	if err := utils.LoadJSON("tokens.json", &tokens); err != nil {
		return fmt.Errorf("erro ao carregar tokens.json: %w", err)
	}
	ctx.Tokens = &tokens
	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadTokens", Status: "Success"})

	var order MaestroOrder
	orderData, err := os.ReadFile(config.Archives.MaestroExecOrder)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo de ordem do maestro '%s': %w", config.Archives.MaestroExecOrder, err)
	}
	if err := json.Unmarshal(orderData, &order); err != nil {
		return fmt.Errorf("erro ao decodificar a ordem do maestro: %w", err)
	}
	ctx.Order = &order
	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadExecutionOrder", Status: "Success"})

	log.Println("Todas as configurações foram carregadas.")
	return nil
}

// logAndExit registra um erro e prepara para sair.
func logAndExit(ctx *MaestroContext, step string, err error) {
	log.Printf("ERRO no passo '%s': %v", step, err)
	ctx.Logs = append(ctx.Logs, ExecutionLog{
		Timestamp: time.Now(),
		Step:      step,
		Status:    "Failed",
		Error:     err.Error(),
	})
}

// saveExecutionLog salva o log de execução em um arquivo JSON.
func saveExecutionLog(ctx *MaestroContext) {
	if ctx.Config == nil || ctx.Config.Archives.LogDir == "" {
		fmt.Println("Não foi possível salvar o log JSON: caminho do diretório de log não configurado.")
		return
	}

	logPath := filepath.Join(ctx.Config.Archives.LogDir, "maestro_summary.json")
	logData, err := json.MarshalIndent(ctx.Logs, "", "  ")
	if err != nil {
		log.Printf("Erro ao serializar o log de resumo: %v", err)
		return
	}

	if err := os.WriteFile(logPath, logData, 0644); err != nil {
		log.Printf("Erro ao salvar o log de resumo em '%s': %v", logPath, err)
	}
}
