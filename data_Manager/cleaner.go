package cleaner_main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Estruturas (As mesmas do exemplo anterior)
type Template struct {
	Regex  string   `json:"regex"`
	Fields []string `json:"fields"`
}
type Templates struct {
	Nmap   map[string]Template `json:"nmap"`
	Nuclei map[string]Template `json:"nuclei"`
}

// O resto da sua estrutura de structs e lógica para cleanFile ...

func cleanFile(filename string, templateName string) error {
	// ... (Código para ler templates.json e selecionar selectedTemplate - SEM ALTERAÇÃO AQUI) ...

	// --- LÓGICA DE LIMPEZA (TRECHO CHAVE) ---

	// 2. IDENTIFICAR FERRAMENTA E SELECIONAR O TEMPLATE CORRETO
	var selectedTemplate Template

	if strings.HasPrefix(filename, "nmap_") {
		t, ok := templates.Nmap[templateName]
		if !ok {
			return fmt.Errorf("template Nmap '%s' não encontrado no JSON", templateName)
		}
		selectedTemplate = t
	} else if strings.HasPrefix(filename, "nuclei_") {
		t, ok := templates.Nuclei[templateName]
		if !ok {
			return fmt.Errorf("template Nuclei '%s' não encontrado no JSON", templateName)
		}
		selectedTemplate = t
	} else {
		return fmt.Errorf("prefixo de ferramenta desconhecido no arquivo: %s", filename)
	}

	// 3. LER O ARQUIVO DE RESULTADOS E APLICAR A LIMPEZA
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

	// 4. SALVAR OS DADOS LIMPOS NO NOVO ARQUIVO TXT
	// Cria o nome do arquivo de saída: 'nmap_clean_escopoXYZ.txt'
	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)) // Remove a extensão
	outputFilename := baseName + "_clean_" + templateName + ".txt"                  // Adiciona _clean_ e o template

	outputFile, err := os.Create(outputFilename)
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

	fmt.Printf("Sucesso! Dados limpos do template '%s' salvos em: %s\n", templateName, outputFilename)
	return nil
}

// func cleaner_main() { ... } (A mesma função cleaner_main de teste)
