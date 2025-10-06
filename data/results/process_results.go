package results

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func processNewCleanFile(filename string, db *sql.DB) error {
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("erro ao abrir arquivo limpo %s: %w", filename, err)
	}
	defer inputFile.Close()

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	parts := strings.Split(baseName, "_clean_")
	if len(parts) != 2 {
		return fmt.Errorf("formato de nome de arquivo inválido. Esperado 'ferramenta_clean_template.txt'")
	}

	tableName := strings.ReplaceAll(baseName, "_clean_", "_")
	fmt.Printf("Processando dados para a tabela: %s\n", tableName)

	scanner := bufio.NewScanner(inputFile)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}

	insertCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, "|")
		if tableName == "nmap_open_ports" && len(fields) == 1 {
			port := fields[0]
			scopeID := "xyz_temp_scope"
			query := fmt.Sprintf("INSERT INTO %s (port, scope_id) VALUES ($1, $2)", tableName)
			_, err := tx.Exec(query, port, scopeID)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao inserir porta %s na tabela %s: %w", port, tableName, err)
			}
			insertCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao commitar transação: %w", err)
	}

	fmt.Printf("Processamento concluído. %d registros inseridos na tabela %s.\n", insertCount, tableName)
	return nil
}
