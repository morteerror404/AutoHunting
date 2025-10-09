package cleaner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/morteerror404/AutoHunting/utils"
)

// Template defines the structure for a single cleaning rule.
type Template struct {
	Regex  string   `json:"regex"`
	Fields []string `json:"fields"`
}

// AllToolTemplates loads the cleaner-templates.json file, which now contains all templates.
type AllToolTemplates map[string]map[string]Template

// CleanFile processes a raw tool output file based on a named template.
func CleanFile(filename string, templateName string) error {
	// Load environment settings to get the output directory.
	env, err := utils.LoadEnvConfig()
	if err != nil {
		return fmt.Errorf("error loading env.json: %w", err)
	}
	cleanedDir, ok := env.Archives["tool_cleaned_dir"]
	if !ok {
		return fmt.Errorf("tool_cleaned_dir not found in env.json")
	}

	// 1. Load the centralized file containing all templates.
	var allTemplates AllToolTemplates
	if err := utils.LoadJSON("cleaner-templates.json", &allTemplates); err != nil {
		return fmt.Errorf("error loading cleaner-templates.json: %w", err)
	}

	// 2. IDENTIFY THE TOOL AND SELECT THE CORRECT TEMPLATE
	var toolName string
	var selectedTemplate Template
	var found bool
	base := filepath.Base(filename)

	for name, toolTemplates := range allTemplates {
		if strings.HasPrefix(base, name+"_") {
			oolName = name
			if t, ok := toolTemplates[templateName]; ok {
				selectedTemplate = t
				found = true
			}
			break
		}
	}

	if !found {
		return fmt.Errorf("cleaning template '%s' not found for tool '%s' in 'cleaner-templates.json'", templateName, toolName)
	}

	// 4. READ THE RAW RESULTS FILE AND APPLY CLEANING
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening results file %s: %w", filename, err)
	}
	defer inputFile.Close()

	re, err := regexp.Compile(selectedTemplate.Regex)
	if err != nil {
		return fmt.Errorf("error compiling regex from template '%s': %w", templateName, err)
	}

	scanner := bufio.NewScanner(inputFile)
	var cleanedLines []string // Now stores formatted strings (lines)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)

		if len(matches) > 1 {
			// Prepare the extracted data to be joined
			extractedData := make([]string, 0, len(selectedTemplate.Fields))
			for i := range selectedTemplate.Fields {
				// i + 1 because the actual matches start at index 1
				if i+1 < len(matches) {
					extractedData = append(extractedData, matches[i+1])
				}
			}
			// JOIN the extracted data into a single line separated by '|'
			cleanedLines = append(cleanedLines, strings.Join(extractedData, "|"))
		}
	}

	// 5. SAVE THE CLEANED DATA TO THE NEW TXT FILE
	baseName := strings.TrimSuffix(base, filepath.Ext(base))
	outputFilename := baseName + "_clean_" + templateName + ".txt"
	outputFilePath := filepath.Join(cleanedDir, outputFilename)

	// Ensure the output directory exists.
	if err := os.MkdirAll(cleanedDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory '%s': %w", cleanedDir, err)
	}

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	for _, line := range cleanedLines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("error writing line to cleaned file: %w", err)
		}
	}
	writer.Flush()

	fmt.Printf("Success! Cleaned data from template '%s' saved to: %s\n", templateName, outputFilePath)
	return nil
}

// CleanerApiRawResults is a placeholder for cleaning raw API results.
func CleanerApiRawResults(destiny string) error {
	fmt.Printf("Function 'CleanerApiRawResults' called with destiny: %s\n", destiny)
	// TODO: Implement logic, possibly calling CleanFile.
	return nil
}

// CleanerReadyMenu is a placeholder for formatting a menu.
func CleanerReadyMenu(destiny string) error {
	fmt.Printf("Function 'CleanerReadyMenu' called with destiny: %s\n", destiny)
	// TODO: Implement logic to prepare data for a menu display.
	return nil
}
