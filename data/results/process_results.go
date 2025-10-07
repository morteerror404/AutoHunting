package results

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/morteerror404/AutoHunting/utils" // To load env.json
)

// EnvConfig structure to load relevant paths from env.json
type EnvConfig struct {
	Path struct {
		APIDirtResultsPath string `json:"api_dirt_results_path"`
		// Add other dirt paths if needed, e.g., ToolDirtDir
	} `json:"path"`
}

// normalizeDirtFiles lê arquivos de um diretório "dirt" especificado.
// Se um arquivo não for um .txt, ele copia seu conteúdo para um novo arquivo temporário .txt
// no mesmo diretório, tornando-o consumível para processamento posterior.
// Ele retorna uma lista de caminhos para todos os arquivos .txt (originais ou recém-criados).
func normalizeDirtFiles(dirtPath string) ([]string, error) {
	var normalizedFilePaths []string

	files, err := os.ReadDir(dirtPath)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler diretório '%s': %w", dirtPath, err)
	}

	for _, fileInfo := range files {
		if fileInfo.IsDir() {
			continue
		}

		fullPath := filepath.Join(dirtPath, fileInfo.Name())
		ext := strings.ToLower(filepath.Ext(fileInfo.Name()))

		if ext == ".txt" {
			normalizedFilePaths = append(normalizedFilePaths, fullPath)
		} else {
			// Cria um arquivo temporário .txt no mesmo diretório
			// Usa os.CreateTemp para criação robusta de arquivos temporários únicos
			tempFile, err := os.CreateTemp(dirtPath, strings.TrimSuffix(fileInfo.Name(), ext)+"_*.tmp.txt")
			if err != nil {
				fmt.Printf("AVISO: não foi possível criar arquivo temporário para '%s': %v\n", fullPath, err)
				continue
			}
			tempFilePath := tempFile.Name()
			tempFile.Close() // Fecha imediatamente após a criação para permitir que io.Copy o abra

			sourceFile, err := os.Open(fullPath)
			if err != nil {
				fmt.Printf("AVISO: não foi possível abrir o arquivo '%s' para normalização: %v\n", fullPath, err)
				os.Remove(tempFilePath) // Limpa o arquivo temporário se a origem não puder ser aberta
				continue
			}

			destFile, err := os.OpenFile(tempFilePath, os.O_WRONLY|os.O_TRUNC, 0644) // Abre para escrita, trunca se existir
			if err != nil {
				fmt.Printf("AVISO: não foi possível abrir arquivo temporário '%s' para escrita: %v\n", tempFilePath, err)
				sourceFile.Close()      // Fecha o arquivo de origem
				os.Remove(tempFilePath) // Limpa o arquivo temporário
				continue
			}

			_, err = io.Copy(destFile, sourceFile)
			if err != nil {
				fmt.Printf("AVISO: erro ao copiar conteúdo de '%s' para '%s': %v\n", fullPath, tempFilePath, err)
				sourceFile.Close()
				destFile.Close()
				os.Remove(tempFilePath) // Limpa o arquivo temporário com falha
				continue
			}
			sourceFile.Close()
			destFile.Close()

			normalizedFilePaths = append(normalizedFilePaths, tempFilePath)
			fmt.Printf("INFO: Arquivo '%s' normalizado para '%s'.\n", fullPath, tempFilePath)
		}
	}

	return normalizedFilePaths, nil
}

// ProcessAndUnifyScopes lê arquivos de escopo de um diretório "dirt", unifica, limpa,
// remove duplicatas e salva o resultado em um único arquivo de saída.
// Esta função é o coração do módulo 'results', preparando os alvos para os scanners.
func ProcessAndUnifyScopes(outputFile string) error {
	// 1. Carrega env.json para obter o caminho do diretório "dirt" da API
	var envConfig EnvConfig
	if err := utils.LoadJSON("env.json", &envConfig); err != nil {
		return fmt.Errorf("erro ao carregar env.json para obter api_dirt_results_path: %w", err)
	}
	apiDirtPath := envConfig.Path.APIDirtResultsPath

	// 2. Normaliza os arquivos no diretório "dirt" da API
	inputFiles, err := normalizeDirtFiles(apiDirtPath)
	if err != nil {
		return fmt.Errorf("erro ao normalizar arquivos do diretório '%s': %w", apiDirtPath, err)
	}

	// Garante que os arquivos temporários sejam limpos após o processamento
	defer func() {
		for _, filePath := range inputFiles {
			// Verifica se é um arquivo temporário criado por normalizeDirtFiles
			if strings.Contains(filepath.Base(filePath), ".tmp.txt") {
				if err := os.Remove(filePath); err != nil {
					fmt.Printf("AVISO: não foi possível remover arquivo temporário '%s': %v\n", filePath, err)
				}
			}
		}
	}()

	// Usar um mapa para armazenar os escopos garante a desduplicação automática.
	uniqueScopes := make(map[string]struct{})

	// Itera sobre cada arquivo de entrada fornecido.
	for _, inputFile := range inputFiles {
		file, err := os.Open(inputFile)
		if err != nil {
			fmt.Printf("AVISO: não foi possível abrir o arquivo de escopo '%s': %v\n", inputFile, err)
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Ignora linhas vazias ou que são apenas comentários.
			if line != "" && !strings.HasPrefix(line, "#") {
				uniqueScopes[line] = struct{}{}
			}
		}
		file.Close() // Fecha imediatamente após a varredura

		if err := scanner.Err(); err != nil {
			fmt.Printf("AVISO: erro ao ler o arquivo '%s': %v\n", inputFile, err)
		}
	}

	// Abre o arquivo de saída para escrita.
	out, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("erro ao criar arquivo de saída '%s': %w", outputFile, err)
	}
	defer out.Close()

	writer := bufio.NewWriter(out)
	count := 0
	for scope := range uniqueScopes {
		if _, err := writer.WriteString(scope + "\n"); err != nil {
			return fmt.Errorf("erro ao escrever no arquivo de saída '%s': %w", outputFile, err)
		}
		count++
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("erro ao fazer flush para o arquivo de saída '%s': %w", outputFile, err)
	}

	fmt.Printf("Processamento de escopo concluído. %d alvos únicos salvos em: %s\n", count, outputFile)
	return nil
}
