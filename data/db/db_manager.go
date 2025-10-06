package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq"
	"github.com/usuario/bug-hunt/utils"
)

type DBConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

func ConnectDB() (*sql.DB, error) {
	var cfg DBConfig
	if err := utils.LoadJSON("db_info.json", &cfg); err != nil {
		return nil, fmt.Errorf("erro ao carregar db_info.json: %w", err)
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão com o DB: %w", err)
	}

	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao conectar ao DB: %w", err)
	}

	fmt.Println("Conexão com PostgreSQL estabelecida com sucesso!")
	return db, nil
}

func ProcessCleanFile(filename string, db *sql.DB) error {
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