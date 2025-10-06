package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"AutoHunting/utils"
)

// DBConfig estrutura para as configurações do banco
type DBConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

// CommandsConfig estrutura para os comandos SQL
type CommandsConfig struct {
	CreateTable  string `json:"create_table"`
	ExcludeTable string `json:"exclude_table"`
	InsertInfo   string `json:"insert_info"`
	SelectInfo   string `json:"select_info"`
}

// DBInfo estrutura geral do arquivo db_info.json
type DBInfo struct {
	ConfigDB DBConfig                  `json:"config_db"`
	Commands map[string]CommandsConfig `json:"commands"`
}

// getCommandsConfig carrega a configuração de comandos para um determinado banco de dados.
func getCommandsConfig(dbType string, dbInfo DBInfo) (CommandsConfig, error) {
	commands, ok := dbInfo.Commands[dbType]
	if !ok {
		return CommandsConfig{}, fmt.Errorf("comandos para o tipo de banco de dados '%s' não encontrados", dbType)
	}
	return commands, nil
}

// ConnectDB abre a conexão com o banco de dados PostgreSQL
func ConnectDB() (*sql.DB, error) {
	var dbInfo DBInfo
	if err := utils.LoadJSON("db_info.json", &dbInfo); err != nil {
		return nil, fmt.Errorf("erro ao carregar db_info.json: %w", err)
	}

	dbType := dbInfo.ConfigDB.Type

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbInfo.ConfigDB.Host, dbInfo.ConfigDB.Port, dbInfo.ConfigDB.User, dbInfo.ConfigDB.Password, dbInfo.ConfigDB.DBName)
	db, err := sql.Open(dbType, connStr)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão com o DB: %w", err)
	}

	commands, err := getCommandsConfig(dbType, dbInfo)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao obter comandos de configuração: %w", err)
	}
	_ = commands // Usar 'commands' em alguma lógica futura para evitar erro de variável não utilizada

	fmt.Println("Conexão com PostgreSQL estabelecida com sucesso!")
	return db, nil
}

// ProcessCleanFile processa o arquivo TXT limpo e insere os dados no banco
func ProcessCleanFile(filename string, db *sql.DB) error {
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("erro ao abrir arquivo limpo %s: %w", filename, err)
	}

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
			scopeID := "xyz_temp_scope" // O ID do escopo deve ser lido de alguma outra fonte
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
	defer inputFile.Close()
	return nil
}
