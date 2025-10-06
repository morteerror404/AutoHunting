package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/morteerror404/AutoHunting/utils"
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

	var connStr string
	switch dbType {
	case "postgres":
		connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			dbInfo.ConfigDB.Host, dbInfo.ConfigDB.Port, dbInfo.ConfigDB.User, dbInfo.ConfigDB.Password, dbInfo.ConfigDB.DBName)
	case "sqlite3":
		// Para sqlite, o "dbname" pode ser o caminho do arquivo.
		connStr = dbInfo.ConfigDB.DBName
	default:
		return nil, fmt.Errorf("tipo de banco de dados não suportado: %s", dbType)
	}

	db, err := sql.Open(dbType, connStr)
	if err != nil {
		// sql.Open não retorna erro para strings de conexão mal formatadas, mas sim no primeiro uso.
		return nil, fmt.Errorf("erro ao abrir conexão com o DB: %w", err)
	}

	commands, err := getCommandsConfig(dbType, dbInfo)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao obter comandos de configuração: %w", err)
	}
	_ = commands // Usar 'commands' em alguma lógica futura para evitar erro de variável não utilizada

	// Ping para verificar a conexão real
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("falha ao conectar com o banco de dados (%s): %w", dbType, err)
	}

	fmt.Printf("Conexão com %s estabelecida com sucesso!\n", dbType)
	return db, nil
}

// ProcessCleanFile processa o arquivo TXT limpo e insere os dados no banco
func ProcessCleanFile(filename string, db *sql.DB) error {
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("erro ao abrir arquivo limpo %s: %w", filename, err)
	}
	defer inputFile.Close()

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	parts := strings.Split(baseName, "_clean_")
	if len(parts) != 2 {
		return fmt.Errorf("formato de nome de arquivo inválido: %s. Esperado 'ferramenta_alvo_timestamp_clean_template.txt'", baseName)
	}

	// Extrai o nome da tabela e o escopo do nome do arquivo
	// Ex: nmap_example.com_2023_clean_open_ports -> Tabela: nmap_open_ports, Escopo: example.com
	templateName := parts[1]
	toolAndScope := parts[0]

	var tool, scope string
	if idx := strings.Index(toolAndScope, "_"); idx != -1 {
		tool = toolAndScope[:idx]
		// O resto, menos o timestamp, é o escopo.
		// Esta é uma simplificação; uma abordagem mais robusta seria necessária se os escopos contiverem '_'.
		scopeParts := strings.Split(toolAndScope[idx+1:], "_")
		scope = strings.Join(scopeParts[:len(scopeParts)-1], "_") // Remove o timestamp
	} else {
		return fmt.Errorf("não foi possível determinar a ferramenta e o escopo de: %s", toolAndScope)
	}

	tableName := fmt.Sprintf("%s_%s", tool, templateName)
	fmt.Printf("Processando dados para a tabela: %s\n", tableName)

	scanner := bufio.NewScanner(inputFile)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	// Garante que a transação seja desfeita em caso de erro
	defer tx.Rollback()

	insertCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")

		// Constrói a query de forma segura e dinâmica
		columns := append(fields, scope)
		placeholders := make([]string, len(columns))
		for i := range columns {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		query := fmt.Sprintf("INSERT INTO %s VALUES (%s)", tableName, strings.Join(placeholders, ", "))

		// Converte colunas para []interface{} para o Exec
		args := make([]interface{}, len(columns))
		for i, v := range columns {
			args[i] = v
		}

		if _, err := tx.Exec(query, args...); err != nil {
			// O defer tx.Rollback() cuidará do rollback
			return fmt.Errorf("erro ao inserir na tabela %s: %w", tableName, err)
		}
		insertCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao commitar transação: %w", err)
	}
	fmt.Printf("Processamento concluído. %d registros inseridos na tabela %s.\n", insertCount, tableName)
	return nil
}
