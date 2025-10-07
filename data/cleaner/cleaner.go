package cleaner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/morteerror404/AutoHunting/utils"
)

// Estruturas (As mesmas do exemplo anterior)
type Template struct {
	Regex  string   `json:"regex"`
	Fields []string `json:"fields"`
}

// TemplatePaths carrega o arquivo principal que mapeia ferramentas para seus arquivos de template.
type TemplatePaths struct {
	Templates map[string]string `json:"templates"`
}

// ToolTemplates carrega o conteúdo de um arquivo de template específico de uma ferramenta (ex: nmap.json).
type ToolTemplates struct {
	Templates map[string]Template `json:"templates"`
}

// EnvConfig carrega os caminhos necessários do env.json.
type EnvConfig struct {
	Path struct {
		ToolCleanedDir string `json:"tool_cleaned_dir"`
	} `json:"path"`
}

// O resto da sua estrutura de structs e lógica para cleanFile ...

func CleanFile(filename string, templateName string) error {
	// Carrega as configurações de ambiente para obter o diretório de saída.
	var env EnvConfig
	if err := utils.LoadJSON("env.json", &env); err != nil {
		return fmt.Errorf("erro ao carregar env.json: %w", err)
	}

	// 1. Carrega o arquivo principal que contém os caminhos para os templates.
	var templatePaths TemplatePaths
	if err := utils.LoadJSON("cleaner-templates.json", &templatePaths); err != nil {
		return fmt.Errorf("erro ao carregar cleaner-templates.json: %w", err)
	}

	// 2. IDENTIFICA A FERRAMENTA E O CAMINHO DO SEU TEMPLATE
	var templateFilePath string
	var toolName string
	base := filepath.Base(filename)

	for name, path := range templatePaths.Templates {
		if strings.HasPrefix(base, name+"_") {
			toolName = name
			templateFilePath = path
			break
		}
	}

	if templateFilePath == "" {
		return fmt.Errorf("nenhum template encontrado para o arquivo: %s", filename)
	}

	// 3. CARREGA O ARQUIVO DE TEMPLATE ESPECÍFICO DA FERRAMENTA
	var toolTemplates ToolTemplates
	templateData, err := os.ReadFile(templateFilePath)
	if err != nil {
		return fmt.Errorf("erro ao ler o arquivo de template '%s' para a ferramenta '%s': %w", templateFilePath, toolName, err)
	}
	if err := json.Unmarshal(templateData, &toolTemplates); err != nil {
		return fmt.Errorf("erro ao decodificar o JSON do template '%s': %w", templateFilePath, err)
	}

	var selectedTemplate Template
	if t, ok := toolTemplates.Templates[templateName]; ok {
		selectedTemplate = t
	} else {
		return fmt.Errorf("template de limpeza '%s' não encontrado para a ferramenta '%s' em '%s'", templateName, toolName, templateFilePath)
	}

	// 4. LER O ARQUIVO DE RESULTADOS BRUTOS E APLICAR A LIMPEZA
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("erro ao abrir arquivo de resultados %s: %w", filename, err)
	}
	defer inputFile.Close()

	re, err := regexp.Compile(selectedTemplate.Regex)
	if err != nil {
		return fmt.Errorf("erro ao compilar regex do template '%s': %w", templateName, err)
	}

	scanner := bufio.NewScanner(inputFile)
	var cleanedLines []string // Agora armazena strings formatadas (linhas)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)

		if len(matches) > 1 {
			// Prepara os dados extraídos para serem unidos
			extractedData := make([]string, 0, len(selectedTemplate.Fields))
			for i := range selectedTemplate.Fields {
				// i + 1 porque os matches reais começam no índice 1
				if i+1 < len(matches) {
					extractedData = append(extractedData, matches[i+1])
				}
			}
			// UNE os dados extraídos em uma única linha separada por '|'
			cleanedLines = append(cleanedLines, strings.Join(extractedData, "|"))
		}
	}

	// 5. SALVAR OS DADOS LIMPOS NO NOVO ARQUIVO TXT
	baseName := strings.TrimSuffix(base, filepath.Ext(base))
	outputFilename := baseName + "_clean_" + templateName + ".txt"
	outputFilePath := filepath.Join(env.Path.ToolCleanedDir, outputFilename)

	// Garante que o diretório de saída exista.
	if err := os.MkdirAll(env.Path.ToolCleanedDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de saída '%s': %w", env.Path.ToolCleanedDir, err)
	}

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("erro ao criar arquivo de saída: %w", err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	for _, line := range cleanedLines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return fmt.Errorf("erro ao escrever linha no arquivo limpo: %w", err)
		}
	}
	writer.Flush()

	fmt.Printf("Sucesso! Dados limpos do template '%s' salvos em: %s\n", templateName, outputFilePath)
	return nil
}

// func data_manager() { ... } (A mesma função data_manager de teste)
