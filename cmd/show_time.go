package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail" //  robust tail library
	"github.com/morteerror404/AutoHunting/utils"
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
	fmt.Println("\n===== AutoHunting - Main Menu =====")
	fmt.Println("1) Start Hunt")
	fmt.Println("2) Query Database")
	fmt.Println("3) Check API Status")
	fmt.Println("0) Exit")

	switch getUserInput("Choose an option: ") {
	case "1":
		handleHuntMenu()
	case "2":
		handleDBMenu()
	case "3":
		handleAPIStatusMenu()
	case "0":
		fmt.Println("Exiting...")
		os.Exit(0)
	default:
		fmt.Println("Invalid option.")
	}
}

// handleHuntMenu gerencia a lógica para iniciar uma "caçada completa".
func handleHuntMenu() {
	tokens, err := loadTokens()
	if err != nil {
		fmt.Printf("Error loading tokens: %v\n", err)
		return
	}

	platform, err := selectPlatform(tokens)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	if platform == "" {
		return // User cancelled
	}

	// Create the order and trigger the maestro
	task := "fullHunt" // Must match the key in order-templates.json
	if err := utils.CreateExecutionOrder(task, platform, nil); err != nil {
		fmt.Printf("Error creating execution order for maestro: %v\n", err)
		return
	}
	triggerMaestro()
}

// handleDBMenu exibe o submenu de operações do banco de dados.
func handleDBMenu() {
	fmt.Println("\n--- Database Menu ---")
	fmt.Println("1) List scopes for a platform")
	fmt.Println("2) Insert scope manually")
	fmt.Println("0) Back")

	switch getUserInput("Choose: ") {
	case "1":
		{
			tokens, err := loadTokens()
			if err != nil {
				fmt.Printf("Error loading tokens: %v\n", err)
				return
			}
			platform, err := selectPlatform(tokens)
			if err != nil {
				fmt.Printf("Error selecting platform: %v\n", err)
				return
			}
			if platform != "" {
				// Create the order for the 'listScopes' task and trigger the maestro
				if err := utils.CreateExecutionOrder("listScopes", platform, nil); err != nil {
					fmt.Printf("Error creating list order: %v\n", err)
				} else {
					fmt.Println("\nList order created. Triggering maestro...")
					triggerMaestro()
				}
			}
		}
	case "2":
		if err := handleManualScopeInsertion(); err != nil {
			fmt.Printf("Error inserting scope: %v\n", err)
		}
	case "0":
		return
	default:
		fmt.Println("Invalid option.")
	}
}

// handleAPIStatusMenu gerencia a verificação de status das APIs das plataformas.
func handleAPIStatusMenu() {
	tokens, err := loadTokens()
	if err != nil {
		fmt.Printf("Error loading tokens: %v\n", err)
		return
	}
	platform, err := selectPlatform(tokens)
	if err != nil {
		fmt.Printf("Error selecting platform: %v\n", err)
		return
	}
	if platform != "" {
		fmt.Println("Checking status...")
		if err = checkAPIStatus(platform); err != nil {
			fmt.Printf("Error checking API: %v\n", err)
		} else {
			fmt.Printf("API for platform '%s' seems to be responding correctly.\n", platform)
		}
	}
}

// handleListScopes gerencia a tarefa de listar escopos de uma plataforma.
func handleListScopes() {
	tokens, err := loadTokens()
	if err != nil {
		fmt.Printf("Error loading tokens: %v\n", err)
		return
	}
	platform, err := selectPlatform(tokens)
	if err != nil {
		fmt.Printf("Error selecting platform: %v\n", err)
		return
	}
	if platform != "" {
		if err := utils.CreateExecutionOrder("listScopes", platform, nil); err != nil {
			fmt.Printf("Error creating list order: %v\n", err)
		} else {
			fmt.Println("\nList order created. Triggering maestro...")
			triggerMaestro()
		}
	}
}

// handleManualScopeInsertion gerencia a inserção manual de um escopo.
func handleManualScopeInsertion() error {
	fmt.Println("\n--- Insert Scope Manually ---")

	tokens, err := loadTokens()
	if err != nil {
		return fmt.Errorf("error loading tokens: %w", err)
	}
	platform, err := selectPlatform(tokens)
	if err != nil {
		return fmt.Errorf("error selecting platform: %w", err)
	}
	if platform == "" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	scope := getUserInput(fmt.Sprintf("Enter the scope for platform '%s' (e.g., example.com): ", platform))
	if scope == "" {
		return fmt.Errorf("scope cannot be empty")
	}

	orderData := map[string]string{"scope": scope}
	if err := utils.CreateExecutionOrder("insertScope", platform, orderData); err != nil {
		return fmt.Errorf("error creating insertion order: %w", err)
	}

	fmt.Println("\nInsertion order created. Triggering maestro...")
	triggerMaestro()
	return nil
}

// loadTokens loads the tokens.json file
func loadTokens() (Tokens, error) {
	var tokens Tokens
	if err := utils.LoadJSON("tokens.json", &tokens); err != nil {
		return tokens, err
	}
	return tokens, nil
}

// selectPlatform displays an interactive menu to select the platform
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
		return "", fmt.Errorf("no platform configured in tokens.json")
	}

	fmt.Println("\n=== Select Platform ===")
	for i, p := range platforms {
		fmt.Printf("%d) %s\n", i+1, p)
	}
	fmt.Println("0) Cancel")

	input := getUserInput("Choose an option: ")

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 0 || choice > len(platforms) {
		return "", fmt.Errorf("invalid option")
	}
	if choice == 0 {
		return "", nil
	}
	return platforms[choice-1], nil
}

// testEndpoint performs a GET request to a URL and checks for a 200 OK status.
func testEndpoint(platformName, url string) error {
	fmt.Printf("Testing connection to %s API...\n", platformName)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error connecting to %s API: %w", platformName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s API returned status code: %d", platformName, resp.StatusCode)
	}
	return nil
}

// checkAPIStatus testa a conexão com a API de uma plataforma.
func checkAPIStatus(platform string) error {
	endpoints := map[string]string{
		"hackerone": "https://api.hackerone.com/v1/hackers/programs", // Endpoint público e leve
		"bugcrowd":  "https://api.bugcrowd.com/programs",
		"intigriti": "https://api.intigriti.com/core/v1/programs",
		"yeswehack": "https://api.yeswehack.com/api/v1/programs",
	}

	url, ok := endpoints[platform]
	if !ok {
		return fmt.Errorf("platform '%s' not supported for API status check", platform)
	}

	return testEndpoint(platform, url)
}

// getUserInput exibe um prompt e lê uma linha de entrada do usuário.
func getUserInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// triggerMaestro writes the order and executes the maestro, monitoring the log.
func triggerMaestro() {
	// 1. Execute the maestro in a new process
	fmt.Println("\n[+] Triggering maestro... Follow the progress below.")
	// It is more efficient to execute the compiled binary than to use 'go run'
	cmd := exec.Command("./bin/maestro") // Assuming the binary is in ./bin/maestro
	cmd.Stderr = os.Stderr               // Redirects the maestro's Stderr to show_time's Stderr

	// 2. Start monitoring the log in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	// Using context for cancellation is more idiomatic in Go
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensures cancellation is called at the end

	go func() {
		defer wg.Done()
		time.Sleep(500 * time.Millisecond) // Small wait for the maestro to create the log file
		// Load env.json to find the log path
		env, err := utils.LoadEnvConfig()
		if err != nil {
			fmt.Printf("\n[ERROR] Could not find maestro log directory: %v\n", err)
			return
		}
		logDir, ok := env.Path["log_dir"]
		if !ok {
			fmt.Printf("\n[ERROR] log_dir not found in env.json\n")
			return
		}
		logPath := filepath.Join(logDir, "maestro_execution.log")
		tailLogFile(ctx, logPath)
	}()

	// Start the command and wait for it to finish
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting maestro: %v\n", err)
		return
	}

	err := cmd.Wait()
	cancel()  // Signals the tail goroutine to stop
	wg.Wait() // Waits for the tail goroutine to finish

	if err != nil {
		fmt.Printf("\n[-] Maestro finished with an error.\n")
	} else {
		fmt.Println("\n[+] Maestro finished execution successfully.")
	}
}

// tailLogFile monitors a log file and prints new lines.
func tailLogFile(ctx context.Context, filepath string) {
	// Using a tail library for a more robust implementation.
	// It handles file creation and is efficient.
	t, err := tail.TailFile(filepath, tail.Config{
		Follow:    true,  // Follow the file (like tail -f)
		ReOpen:    true,  // Try to reopen the file if it is rotated or recreated
		MustExist: false, // Does not fail if the file does not exist at the beginning
		Poll:      true,  // Uses polling, good for network file systems or when inotify fails
	})
	if err != nil {
		fmt.Printf("\n[ERROR] Failed to start log monitoring: %v\n", err)
		return
	}

	for line := range t.Lines {
		fmt.Print("[Maestro] ", line.Text)
	}

	<-ctx.Done() // Waits for the cancellation signal
}
