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

// MaestroContext contains all the configurations and state for a maestro execution.
type MaestroContext struct {
	Start    time.Time
	Logs     []ExecutionLog
	Config   *utils.EnvConfig
	Commands *Commands
	Tokens   *Tokens
	Order    *MaestroOrder
	LogFile  *os.File
}

// ExecutionLog represents a log entry for a step in the execution.
type ExecutionLog struct {
	Timestamp time.Time `json:"timestamp"`
	Step      string    `json:"step"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
}

// MaestroOrder represents the structure of the execution order.
type MaestroOrder struct {
	Platform string              `json:"platform"`
	Task     string              `json:"task"`
	Steps    []utils.MaestroTask `json:"steps"`
	Data     map[string]string   `json:"data,omitempty"` // Generic field for extra data
}

func main() {
	log.Println("Starting maestro default execution flow from order...")

	if err := runMaestro(); err != nil {
		// The error has already been logged inside runMaestro, just exit with error status.
		os.Exit(1)
	}
	log.Println("Maestro finished execution successfully.")
}

func runMaestro() error {
	ctx := &MaestroContext{
		Start: time.Now(),
		Logs:  []ExecutionLog{},
	}

	// 1. Setup Logging
	if err := setupLogging(ctx); err != nil {
		fmt.Printf("Critical error setting up logging: %v\n", err)
		return err // Fatal error, we cannot continue without logging.
	}
	defer ctx.LogFile.Close()
	defer saveExecutionLog(ctx) // Ensures logs are saved at the end.

	// 2. Load all configurations
	if err := loadAllConfigs(ctx); err != nil {
		logAndExit(ctx, "LoadAllConfigs", err)
		return err
	}

	// 3. Iterate and execute each step of the order
	for _, step := range ctx.Order.Steps {
		log.Printf("Starting step: %s", step.Description)
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
		case "insertScope":
			stepErr = stepInsertScope(ctx)
		case "listScopes":
			stepErr = stepListScopes(ctx)
		default:
			stepErr = fmt.Errorf("unknown step in execution order: %s", step.Step)
		}

		if stepErr != nil {
			logAndExit(ctx, step.Step, stepErr)
			return stepErr // Stops execution on error
		}
		ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: step.Step, Status: "Success"})
		log.Printf("Step '%s' completed successfully.", step.Step)
	}

	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "ExecutionCompleted", Status: "Success"})
	log.Printf("Processing finished in %v!\n", time.Since(ctx.Start))
	return nil
}

func stepRequestAPI(ctx *MaestroContext) error {
	apiDirtPath, ok := ctx.Config.Path["api_dirt_results_path"]
	if !ok {
		return fmt.Errorf("api_dirt_results_path not found in env.json")
	}
	return api.RunRequestAPI(apiDirtPath, ctx.Order.Platform, api.Tokens(*ctx.Tokens))
}

func stepRunScanners(ctx *MaestroContext) error {
	outDir, ok := ctx.Config.Path["tool_dirt_dir"]
	if !ok {
		return fmt.Errorf("tool_dirt_dir not found in env.json")
	}
	apiDirtPath, ok := ctx.Config.Path["api_dirt_results_path"]
	if !ok {
		return fmt.Errorf("api_dirt_results_path not found in env.json")
	}

	for tool, args := range map[string]string{
		"nmap": ctx.Commands.Nmap["nmap_slow"],
		"ffuf": ctx.Commands.Ffuf["default"],
	} {
		if err := runner.Run(apiDirtPath, args, outDir, tool); err != nil {
			return fmt.Errorf("runner error for %s: %w", tool, err)
		}
	}
	return nil
}

func stepCleanResults(ctx *MaestroContext) error {
	outDir, ok := ctx.Config.Path["tool_dirt_dir"]
	if !ok {
		return fmt.Errorf("tool_dirt_dir not found in env.json")
	}
	files, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("failed to read raw results directory '%s': %w", outDir, err)
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
			log.Printf("WARNING: Failed to clean file %s: %v", filename, err)
			continue
		}
	}
	return nil
}

func stepStoreResults(ctx *MaestroContext) error {
	dbConn, err := db.ConnectDB()
	if err != nil {
		return fmt.Errorf("error connecting to DB: %w", err)
	}
	defer dbConn.Close()

	cleanDir, ok := ctx.Config.Path["tool_cleaned_dir"]
	if !ok {
		return fmt.Errorf("tool_cleaned_dir not found in env.json")
	}
	files, err := os.ReadDir(cleanDir)
	if err != nil {
		return fmt.Errorf("failed to read cleaned results directory '%s': %w", cleanDir, err)
	}

	for _, f := range files {
		if f.IsDir() || !strings.Contains(f.Name(), "_clean_") {
			continue
		}
		filename := filepath.Join(cleanDir, f.Name())
		if err := db.ProcessCleanFile(filename, dbConn); err != nil {
			log.Printf("WARNING: Failed to process cleaned file %s for DB: %v", filename, err)
			continue
		}
	}
	return nil
}

func stepInsertScope(ctx *MaestroContext) error {
	platform := ctx.Order.Platform
	scope, ok := ctx.Order.Data["scope"]
	if !ok || scope == "" {
		return fmt.Errorf("data 'scope' not found or empty in execution order for task 'insertScope'")
	}

	dbConn, err := db.ConnectDB()
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer dbConn.Close()

	query := "INSERT INTO scopes (platform, scope) VALUES ($1, $2)"
	if _, err := dbConn.Exec(query, platform, scope); err != nil {
		return fmt.Errorf("error executing database insertion: %w", err)
	}

	log.Printf("Success! Scope '%s' inserted for platform '%s'.\n", scope, platform)
	return nil
}

func stepListScopes(ctx *MaestroContext) error {
	platform := ctx.Order.Platform
	dbConn, err := db.ConnectDB()
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	defer dbConn.Close()

	return db.ShowScopes(platform, dbConn)
}

func setupLogging(ctx *MaestroContext) error {
	env, err := utils.LoadEnvConfig()
	if err != nil {
		return fmt.Errorf("failed to load env.json to configure logging: %w", err)
	}
	ctx.Config = env

	logDir, ok := env.Path["log_dir"]
	if !ok {
		return fmt.Errorf("log_dir not found in env.json")
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory '%s': %w", logDir, err)
	}

	logPath := filepath.Join(logDir, "maestro_execution.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file '%s': %w", logPath, err)
	}
	ctx.LogFile = f

	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.SetFlags(log.Ldate | log.Ltime)

	log.Println("Logging configured successfully.")
	return nil
}

func loadAllConfigs(ctx *MaestroContext) error {
	var commands Commands
	if err := utils.LoadJSON("commands.json", &commands); err != nil {
		return fmt.Errorf("error loading commands.json: %w", err)
	}
	ctx.Commands = &commands
	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadCommands", Status: "Success"})

	var tokens Tokens
	if err := utils.LoadJSON("tokens.json", &tokens); err != nil {
		return fmt.Errorf("error loading tokens.json: %w", err)
	}
	ctx.Tokens = &tokens
	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadTokens", Status: "Success"})

	var order MaestroOrder
	if err := utils.LoadJSON("order.json", &order); err != nil {
		return fmt.Errorf("error loading maestro order: %w", err)
	}
	ctx.Order = &order
	ctx.Logs = append(ctx.Logs, ExecutionLog{Timestamp: time.Now(), Step: "LoadExecutionOrder", Status: "Success"})

	log.Println("All configurations loaded.")
	return nil
}

func logAndExit(ctx *MaestroContext, step string, err error) {
	log.Printf("ERROR in step '%s': %v", step, err)
	ctx.Logs = append(ctx.Logs, ExecutionLog{
		Timestamp: time.Now(),
		Step:      step,
		Status:    "Failed",
		Error:     err.Error(),
	})
}

func saveExecutionLog(ctx *MaestroContext) {
	if ctx.Config == nil {
		fmt.Println("Could not save JSON log: config not loaded.")
		return
	}
	logDir, ok := ctx.Config.Path["log_dir"]
	if !ok {
		fmt.Println("Could not save JSON log: log_dir not configured.")
		return
	}

	logPath := filepath.Join(logDir, "maestro_summary.json")
	logData, err := json.MarshalIndent(ctx.Logs, "", "  ")
	if err != nil {
		log.Printf("Error marshalling summary log: %v", err)
		return
	}

	if err := os.WriteFile(logPath, logData, 0644); err != nil {
		log.Printf("Error saving summary log to '%s': %v", logPath, err)
	}
}