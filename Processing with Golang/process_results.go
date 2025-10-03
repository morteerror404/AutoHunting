package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var (
	dbPath     string
	dirtyPath  string
	cleanPath  string
)

// ============================
// Função para criar o banco
// ============================
func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		status TEXT NOT NULL
	);`
	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

// ============================
// Registrar arquivo no DB
// ============================
func registerFile(db *sql.DB, filename string, status string) {
	_, err := db.Exec("INSERT INTO files (name, status) VALUES (?, ?)", filename, status)
	if err != nil {
		log.Println("Erro ao registrar arquivo:", err)
	}
}

// ============================
// Listar arquivos do DB
// ============================
func listFiles(db *sql.DB) {
	rows, err := db.Query("SELECT id, name, status FROM files")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Arquivos registrados no DB:")
	for rows.Next() {
		var id int
		var name, status string
		rows.Scan(&id, &name, &status)
		fmt.Printf("[%d] %s - %s\n", id, name, status)
	}
}

// ============================
// Processar arquivos sujos
// ============================
func processFiles(db *sql.DB) {
	files, err := os.ReadDir(dirtyPath)
	if err != nil {
		log.Fatal("Erro ao ler pasta suja:", err)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		oldPath := filepath.Join(dirtyPath, f.Name())
		newPath := filepath.Join(cleanPath, f.Name())

		// Simulação de tratamento -> mover para "limpa"
		err := os.Rename(oldPath, newPath)
		if err != nil {
			log.Println("Erro ao mover arquivo:", err)
			continue
		}

		registerFile(db, f.Name(), "tratado")
		fmt.Println("Arquivo tratado:", f.Name())
	}
}

// ============================
// Menu interativo
// ============================
func menu() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n===== MENU =====")
		fmt.Println("1) Definir pasta suja")
		fmt.Println("2) Definir pasta limpa")
		fmt.Println("3) Definir caminho do DB")
		fmt.Println("4) Processar arquivos")
		fmt.Println("5) Listar arquivos no DB")
		fmt.Println("0) Sair")
		fmt.Print("Escolha uma opção: ")

		input, _ := reader.ReadString('\n')
		switch input[:len(input)-1] {
		case "1":
			fmt.Print("Informe o caminho da pasta suja: ")
			dirtyPath, _ = reader.ReadString('\n')
			dirtyPath = dirtyPath[:len(dirtyPath)-1]
		case "2":
			fmt.Print("Informe o caminho da pasta limpa: ")
			cleanPath, _ = reader.ReadString('\n')
			cleanPath = cleanPath[:len(cleanPath)-1]
		case "3":
			fmt.Print("Informe o caminho do DB (ex: data.db): ")
			dbPath, _ = reader.ReadString('\n')
			dbPath = dbPath[:len(dbPath)-1]
		case "4":
			if dbPath == "" || dirtyPath == "" || cleanPath == "" {
				fmt.Println("⚠️ Configure os caminhos antes de processar!")
				continue
			}
			db := initDB()
			defer db.Close()
			processFiles(db)
		case "5":
			if dbPath == "" {
				fmt.Println("⚠️ Configure o DB primeiro!")
				continue
			}
			db := initDB()
			defer db.Close()
			listFiles(db)
		case "0":
			fmt.Println("Saindo...")
			return
		default:
			fmt.Println("Opção inválida")
		}
	}
}

func main() {
	menu()
}
