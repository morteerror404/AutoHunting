package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadJSON carrega um arquivo JSON do diretório json/ e decodifica para a estrutura fornecida
func LoadJSON(filename string, v interface{}) error {
	absPath := filepath.Join("json", filename)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("erro ao ler arquivo %s: %w", absPath, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("erro ao decodificar JSON de %s: %w", absPath, err)
	}

	return nil
}