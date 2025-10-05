package main

import (
    "bufio"
    "database/sql"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "time"
)

var (
    dbPath     string
    dirtyPath  string
    cleanPath  string
)

// ============================
// Abrir conexão com o banco
// ============================
func openDB() *sql.DB {
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        log.Fatal("Erro ao abrir conexão com o DB:", err)
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
        log.Fatal("Erro ao listar arquivos do DB:", err)
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
// Processar arquivos da pasta limpa
// ============================
func processCleanFiles(db *sql.DB) {
    files, err := os.ReadDir(cleanPath)
    if err != nil {
        log.Fatal("Erro ao ler pasta limpa:", err)
    }
    for _, f := range files {
        if f.IsDir() {
            continue
        }
        registerFile(db, f.Name(), "limpo")
        fmt.Println("Arquivo limpo registrado:", f.Name())
    }
}

// ============================
// Criar pastas suja e limpa
// ============================
func createFolders() error {
    if dirtyPath == "" || cleanPath == "" {
        return fmt.Errorf("Configure os caminhos da pasta suja e limpa primeiro!")
    }
    err := os.MkdirAll(dirtyPath, 0755)
    if err != nil {
        return fmt.Errorf("Erro ao criar pasta suja: %v", err)
    }
    err = os.MkdirAll(cleanPath, 0755)
    if err != nil {
        return fmt.Errorf("Erro ao criar pasta limpa: %v", err)
    }
    fmt.Println("Pastas criadas com sucesso:", dirtyPath, "e", cleanPath)
    return nil
}

// ============================
// Verificar conexão com o DB
// ============================
func checkDBConnection() error {
    if dbPath == "" {
        return fmt.Errorf("Configure o caminho do DB primeiro!")
    }
    db, err := sql.Open("sqlite3", dbPath)
    if err != nil {
        return fmt.Errorf("Erro ao abrir conexão com o DB: %v", err)
    }
    defer db.Close()
    err = db.Ping()
    if err != nil {
        return fmt.Errorf("Erro ao verificar conexão com o DB: %v", err)
    }
    fmt.Println("Conexão com o banco de dados bem-sucedida!")
    return nil
}

// ============================
// Verificar extensões na pasta suja
// ============================
func checkDirtyFolderExtensions() error {
    if dirtyPath == "" {
        return fmt.Errorf("Configure o caminho da pasta suja primeiro!")
    }
    files, err := os.ReadDir(dirtyPath)
    if err != nil {
        return fmt.Errorf("Erro ao ler pasta suja: %v", err)
    }
    extensionCount := make(map[string]int)
    for _, f := range files {
        if f.IsDir() {
            continue
        }
        ext := filepath.Ext(f.Name())
        if ext == "" {
            ext = "sem extensão"
        }
        extensionCount[ext]++
    }
    fmt.Println("Extensões encontradas na pasta suja:")
    for ext, count := range extensionCount {
        fmt.Printf("%s: %d arquivo(s)\n", ext, count)
    }
    return nil
}

// ============================
// Salvar informações dos arquivos em um arquivo de texto
// ============================
func saveFileInfoToFile(outputFile string) error {
    if dirtyPath == "" {
        return fmt.Errorf("Configure o caminho da pasta suja primeiro!")
    }
    if outputFile == "" {
        return fmt.Errorf("Informe o caminho do arquivo de saída!")
    }
    files, err := os.ReadDir(dirtyPath)
    if err != nil {
        return fmt.Errorf("Erro ao ler pasta suja: %v", err)
    }
    file, err := os.Create(outputFile)
    if err != nil {
        return fmt.Errorf("Erro ao criar arquivo de saída: %v", err)
    }
    defer file.Close()
    writer := bufio.NewWriter(file)
    for _, f := range files {
        if f.IsDir() {
            continue
        }
        info, err := f.Info()
        if err != nil {
            log.Printf("Erro ao obter informações do arquivo %s: %v\n", f.Name(), err)
            continue
        }
        ext := filepath.Ext(f.Name())
        if ext == "" {
            ext = "sem extensão"
        }
        line := fmt.Sprintf("Nome: %s, Extensão: %s, Tamanho: %d bytes, Modificação: %s\n",
            f.Name(), ext, info.Size(), info.ModTime().Format(time.RFC3339))
        _, err = writer.WriteString(line)
        if err != nil {
            log.Printf("Erro ao escrever no arquivo de saída: %v\n", err)
            continue
        }
    }
    err = writer.Flush()
    if err != nil {
        return fmt.Errorf("Erro ao salvar dados no arquivo: %v", err)
    }
    fmt.Printf("Informações dos arquivos salvas em: %s\n", outputFile)
    return nil
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
        fmt.Println("4) Processar arquivos da pasta suja")
        fmt.Println("5) Listar arquivos no DB")
        fmt.Println("6) Criar pastas suja e limpa")
        fmt.Println("7) Processar arquivos da pasta limpa")
        fmt.Println("8) Verificar conexão com o DB")
        fmt.Println("9) Verificar extensões na pasta suja")
        fmt.Println("10) Salvar informações dos arquivos")
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
                fmt.Println("Configure os caminhos antes de processar!")
                continue
            }
            db := openDB()
            defer db.Close()
            processFiles(db)
        case "5":
            if dbPath == "" {
                fmt.Println("Configure o DB primeiro!")
                continue
            }
            db := openDB()
            defer db.Close()
            listFiles(db)
        case "6":
            err := createFolders()
            if err != nil {
                log.Println(err)
            }
        case "7":
            if dbPath == "" || cleanPath == "" {
                fmt.Println("Configure o caminho do DB e da pasta limpa primeiro!")
                continue
            }
            db := openDB()
            defer db.Close()
            processCleanFiles(db)
        case "8":
            err := checkDBConnection()
            if err != nil {
                log.Println(err)
            }
        case "9":
            err := checkDirtyFolderExtensions()
            if err != nil {
                log.Println(err)
            }
        case "10":
            if dirtyPath == "" {
                fmt.Println("Configure o caminho da pasta suja primeiro!")
                continue
            }
            fmt.Print("Informe o caminho do arquivo de saída (ex: info.txt): ")
            outputFile, _ := reader.ReadString('\n')
            outputFile = outputFile[:len(outputFile)-1]
            err := saveFileInfoToFile(outputFile)
            if err != nil {
                log.Println(err)
            }
        case "0":
            fmt.Println("Saindo...")
            return
        default:
            fmt.Println("Opção inválida")
        }
    }
}
