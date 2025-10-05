package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lib/pq" // Driver do PostgreSQL
)

// Estrutura para configurar a conexão com o banco de dados.
const (
	dbHost     = "localhost"
	dbPort     = 5432
	dbUser     = "postgres"
	dbPassword = "sua_senha_secreta" // Troque pela sua senha real
	dbName     = "bug_hunt_db"
)

// connectDB estabelece a conexão com o PostgreSQL.
func connectDB() (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

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

// processCleanFile lê o arquivo TXT limpo e insere os dados no banco.
func processCleanFile(filename string, db *sql.DB) error {
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("erro ao abrir arquivo limpo %s: %w", filename, err)
	}
	defer inputFile.Close()

	// 1. DEDUZIR A TABELA E OS CAMPOS PELO NOME DO ARQUIVO
	// Ex: nmap_clean_open_ports.txt -> Ferramenta: nmap, Template: open_ports
	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	parts := strings.Split(baseName, "_clean_")
	
	if len(parts) != 2 {
		return fmt.Errorf("formato de nome de arquivo inválido. Esperado 'ferramenta_clean_template.txt'")
	}
	
	// A tabela será a combinação da ferramenta e do template, ex: "nmap_open_ports"
	tableName := strings.ReplaceAll(baseName, "_clean_", "_") 
	
	// ATENÇÃO: Você deve garantir que esta tabela existe no seu banco de dados!
	// Exemplo de tabela para o template 'open_ports':
	// CREATE TABLE nmap_open_ports (port INT, scope_id VARCHAR);
	
	fmt.Printf("Processando dados para a tabela: %s\n", tableName)

	scanner := bufio.NewScanner(inputFile)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	
	insertCount := 0
	
	// 2. LER O ARQUIVO LINHA POR LINHA E INSERIR
	for scanner.Scan() {
		line := scanner.Text()
		// O separador que definimos no cleaner.go foi o '|'
		fields := strings.Split(line, "|") 
		
		// ********** LÓGICA DE INSERÇÃO DINÂMICA (Exemplo para 'open_ports') **********
		
		// Este é o ponto onde a lógica de INSERT precisa ser personalizada para cada template.
		// Para simplificar, vou assumir o template 'open_ports' (que tem apenas 'port')
		
		if tableName == "nmap_open_ports" && len(fields) == 1 {
			// Supondo que 'fields[0]' é a porta e o escopo é um valor fixo por enquanto
			port := fields[0]
			scopeID := "xyz_temp_scope" // O ID do escopo deve ser lido de alguma outra fonte

			query := fmt.Sprintf("INSERT INTO %s (port, scope_id) VALUES ($1, $2)", tableName)
			_, err := tx.Exec(query, port, scopeID)
			
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("erro ao inserir porta %s na tabela %s: %w", port, tableName, err)
			}
			insertCount++
			
		} else {
			// Se o template for outro, você adicionaria aqui a lógica do INSERT
			// Ex: if tableName == "nuclei_vulnerabilities" { ... }
		}
		// ********************************************************************************
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao commitar transação: %w", err)
	}

	fmt.Printf("Processamento concluído. %d registros inseridos na tabela %s.\n", insertCount, tableName)
	return nil
}

func main() {
	// 1. Conectar ao Banco de Dados
	db, err := connectDB()
	if err != nil {
		fmt.Println("Erro fatal na conexão:", err)
		return
	}
	defer db.Close()

	// 2. Processar o arquivo limpo gerado pelo Cleaner.go
	// Substitua 'nmap_escopo_xyz.txt_clean_open_ports.txt' pelo nome real do arquivo limpo
	cleanFilename := "nmap_escopo_xyz.txt_clean_open_ports.txt" 
	
	if err := processCleanFile(cleanFilename, db); err != nil {
		fmt.Println("Erro ao processar arquivo:", err)
	}
}