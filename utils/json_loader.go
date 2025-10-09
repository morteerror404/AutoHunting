package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// EnvConfig holds the configuration from env.json
type EnvConfig struct {
	Path              map[string]string   `json:"path"`
	Archives          map[string]string   `json:"archives"`
	AccessPermissions map[string][]string `json:"access_permissions"`
}

// MaestroTask represents a step in a Maestro order
type MaestroTask struct {
	Step        string `json:"step"`
	Description string `json:"description"`
}

// MaestroOrder represents a Maestro execution order
type MaestroOrder struct {
	Platform string            `json:"platform"`
	Task     string            `json:"task"`
	Steps    []MaestroTask     `json:"steps"`
	Data     map[string]string `json:"data,omitempty"`
}

var envConfig *EnvConfig

// LoadEnvConfig loads the env.json file and caches it
func LoadEnvConfig() (*EnvConfig, error) {
	if envConfig != nil {
		return envConfig, nil
	}

	// Assuming env.json is in a known relative path from the executable or working directory
	// For this example, let's assume it's in 'config/json/env.json'
	// In a real application, you might want to use an environment variable or a command-line flag
	// to specify the config path.
	envPath := "config/json/env.json"
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("error reading env.json: %w", err)
	}

	var config EnvConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error unmarshalling env.json: %w", err)
	}

	envConfig = &config
	return envConfig, nil
}

// GetEnvPath retrieves a specific path from the "path" section of env.json.
func GetEnvPath(pathKey string) (string, error) {
	config, err := LoadEnvConfig()
	if err != nil {
		return "", err
	}

	path, ok := config.Path[pathKey]
	if !ok {
		return "", fmt.Errorf("path key '%s' not found in env.json 'path' object", pathKey)
	}

	return path, nil
}

// GetAccessPermissions retrieves the access permissions for a given module.
func GetAccessPermissions(moduleName string) ([]string, error) {
	config, err := LoadEnvConfig()
	if err != nil {
		return nil, err
	}

	permissions, ok := config.AccessPermissions[moduleName]
	if !ok {
		return nil, fmt.Errorf("no access permissions found for module '%s'", moduleName)
	}

	return permissions, nil
}

// GetJSONPath constructs the full path for a given JSON file name
func GetJSONPath(jsonName string) (string, error) {
	config, err := LoadEnvConfig()
	if err != nil {
		return "", err
	}

	// Normalize jsonName to have consistent key format (e.g., "commands.json")
	normalizedJSONName := strings.ReplaceAll(jsonName, "-", "_")
	normalizedJSONName = strings.TrimSuffix(normalizedJSONName, ".json")

	var archiveKey string
	for key, value := range config.Archives {
		if strings.Contains(value, jsonName) {
			archiveKey = key
			break
		}
	}

	if archiveKey == "" {
		return "", fmt.Errorf("archive path for '%s' not found in env.json", jsonName)
	}

	relativePath := config.Archives[archiveKey]
	if relativePath == "" {
		return "", fmt.Errorf("archive path for '%s' not found in env.json", jsonName)
	}

	basePath, ok := config.Path["base_path"]
	if !ok {
		return "", fmt.Errorf("'base_path' not found in env.json path config")
	}

	return filepath.Join(basePath, relativePath), nil
}

// LoadJSON loads a JSON file using the path from env.json
func LoadJSON(jsonName string, v interface{}) error {
	absPath, err := GetJSONPath(jsonName)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("error reading file %s: %w", absPath, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("error unmarshalling JSON from %s: %w", absPath, err)
	}

	return nil
}

// CreateExecutionOrder generates the execution order file for the maestro.
func CreateExecutionOrder(task, platform string, data map[string]string) error {
	// 1. Load order templates
	var orderTemplates struct {
		ExecutionPlans map[string][]MaestroTask `json:"execution_plans"`
	}
	if err := LoadJSON("order-templates.json", &orderTemplates); err != nil {
		return fmt.Errorf("error loading order templates: %w", err)
	}

	// 2. Find the execution plan for the requested task
	steps, ok := orderTemplates.ExecutionPlans[task]
	if !ok {
		return fmt.Errorf("execution plan for task '%s' not found", task)
	}

	// 3. Assemble the final order
	finalOrder := MaestroOrder{
		Platform: platform,
		Task:     task,
		Steps:    steps,
		Data:     data,
	}

	// 4. Serialize and save the final order file
	orderData, err := json.MarshalIndent(finalOrder, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling final maestro order: %w", err)
	}

	maestroOrderPath, err := GetJSONPath("order.json")
	if err != nil {
		return fmt.Errorf("error getting path for maestro order file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(maestroOrderPath), 0755); err != nil {
		return fmt.Errorf("error creating directory for maestro order: %w", err)
	}

	if err := ioutil.WriteFile(maestroOrderPath, orderData, 0644); err != nil {
		return fmt.Errorf("error writing maestro order file '%s': %w", maestroOrderPath, err)
	}

	fmt.Printf("Execution order for task '%s' created successfully at: %s\n", task, maestroOrderPath)
	return nil
}

// CheckEnvPath is a placeholder for the function that checks environment paths.
// The specific logic needs to be implemented based on maestro's requirements.
func CheckEnvPath(destiny string) error {
	fmt.Printf("Function 'CheckEnvPath' called with destiny: %s\n", destiny)
	// TODO: Implement the actual logic for checking the environment path.
	// This could involve using the 'destiny' parameter to look up a path
	// in the envConfig and verifying its existence or permissions.
	return nil
}