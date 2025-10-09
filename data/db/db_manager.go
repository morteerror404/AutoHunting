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

// DBConfig structure for the database settings
type DBConfig struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	DBName   string `json:"dbname"`
}

// CommandsConfig structure for the SQL commands
type CommandsConfig struct {
	CreateTable  string `json:"create_table"`
	ExcludeTable string `json:"exclude_table"`
	InsertInfo   string `json:"insert_info"`
	SelectInfo   string `json:"select_info"`
}

// DBInfo general structure of the db_info.json file
type DBInfo struct {
	ConfigDB DBConfig                  `json:"config_db"`
	Commands map[string]CommandsConfig `json:"commands"`
}

// getCommandsConfig loads the command configuration for a given database type.
func getCommandsConfig(dbType string, dbInfo DBInfo) (CommandsConfig, error) {
	commands, ok := dbInfo.Commands[dbType]
	if !ok {
		return CommandsConfig{}, fmt.Errorf("commands for database type '%s' not found", dbType)
	}
	return commands, nil
}

// ConnectDB opens the connection to the PostgreSQL database
func ConnectDB() (*sql.DB, error) {
	var dbInfo DBInfo
	if err := utils.LoadJSON("db_info.json", &dbInfo); err != nil {
		return nil, fmt.Errorf("error loading database configuration: %w", err)
	}

	dbType := dbInfo.ConfigDB.Type

	var connStr string
	switch dbType {
	case "postgres":
		connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			dbInfo.ConfigDB.Host, dbInfo.ConfigDB.Port, dbInfo.ConfigDB.User, dbInfo.ConfigDB.Password, dbInfo.ConfigDB.DBName)
	case "sqlite3":
		// For sqlite, the "dbname" can be the file path.
		connStr = dbInfo.ConfigDB.DBName
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	db, err := sql.Open(dbType, connStr)
	if err != nil {
		// sql.Open does not return an error for malformed connection strings, but on first use.
		return nil, fmt.Errorf("error opening DB connection: %w", err)
	}

	commands, err := getCommandsConfig(dbType, dbInfo)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("error getting configuration commands: %w", err)
	}
	_ = commands // Use 'commands' in some future logic to avoid unused variable error

	// Ping to check the actual connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to the database (%s): %w", dbType, err)
	}

	fmt.Printf("Connection with %s established successfully!\n", dbType)
	return db, nil
}

// ProcessCleanFile processes the cleaned TXT file and inserts the data into the database
func ProcessCleanFile(filename string, db *sql.DB) error {
	inputFile, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening clean file %s: %w", filename, err)
	}
	defer inputFile.Close()

	// Load the settings to determine the DB type and commands
	var dbInfo DBInfo
	if err := utils.LoadJSON("db_info.json", &dbInfo); err != nil {
		return fmt.Errorf("error loading database configuration for processing: %w", err)
	}
	dbType := dbInfo.ConfigDB.Type
	_, err = getCommandsConfig(dbType, dbInfo)
	if err != nil {
		return err
	}

	baseName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	parts := strings.Split(baseName, "_clean_")
	if len(parts) != 2 {
		return fmt.Errorf("invalid filename format: %s. Expected 'tool_target_timestamp_clean_template.txt'", baseName)
	}

	templateName := parts[1]
	toolAndScope := parts[0]

	var tool string
	if idx := strings.Index(toolAndScope, "_"); idx != -1 {
		tool = toolAndScope[:idx]
	} else {
		return fmt.Errorf("could not determine the tool from: %s", toolAndScope)
	}

	tableName := fmt.Sprintf("%s_%s", tool, templateName)
	fmt.Printf("Processing data for table: %s\n", tableName)

	scanner := bufio.NewScanner(inputFile)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	// Ensures the transaction is rolled back in case of an error
	defer tx.Rollback()

	insertCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		fields := strings.Split(line, "|")

		// Builds the query safely and dynamically
		// The logic to add the 'scope' has been removed to simplify,
		// as the extraction was fragile. The insertion now focuses on the data from the clean file.
		// A more robust logic for metadata (like the scope) should be implemented.
		columns := fields
		placeholders := make([]string, len(columns))
		for i := range columns {
			placeholders[i] = fmt.Sprintf("$\%d", i+1)
		}
		// TODO: Abstract the SQL dialect.
		// The use of placeholders like '$1, $2' is specific to PostgreSQL.
		// To support other databases (e.g., MySQL which uses '?'), it would be necessary to adapt the query based on 'db.DriverName()'.
		query := fmt.Sprintf("INSERT INTO %s VALUES (%s)", tableName, strings.Join(placeholders, ", "))

		// Converts columns to []interface{} for Exec
		args := make([]interface{}, len(columns))
		for i, v := range columns {
			args[i] = v
		}

		if _, err := tx.Exec(query, args...); err != nil {
			// The defer tx.Rollback() will take care of the rollback
			return fmt.Errorf("error inserting into table %s: %w", tableName, err)
		}
		insertCount++
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}
	fmt.Printf("Processing complete. %d records inserted into table %s.\n", insertCount, tableName)
	return nil
}

// ShowScopes queries and displays the available scopes for a platform.
// This function is called by the maestro to execute the listing task.
func ShowScopes(platform string, db *sql.DB) error {
	query := "SELECT scope FROM scopes WHERE platform = $1"

rows, err := db.Query(query, platform)
	if err != nil {
		return fmt.Errorf("error executing scope query for %s: %w", platform, err)
	}
	defer rows.Close()

	fmt.Printf("\n=== Available scopes for %s ===\n", platform)
	count := 0
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return fmt.Errorf("error reading scope: %w", err)
		}
		fmt.Printf("- %s\n", scope)
		count++
	}

	if count == 0 {
		fmt.Printf("No scopes found for %s.\n", platform)
	} else {
		fmt.Printf("Total scopes found: %d\n", count)
	}

	return nil
}

// DataBaseStore is a placeholder for storing data in the database.
func DataBaseStore(destiny string) error {
	fmt.Printf("Function 'DataBaseStore' called with destiny: %s\n", destiny)
	// TODO: Implement logic, possibly calling ProcessCleanFile.
	return nil
}

// GetInfoFromDB is a placeholder for retrieving information from the database.
func GetInfoFromDB(destiny string) error {
	fmt.Printf("Function 'GetInfoFromDB' called with destiny: %s\n", destiny)
	// TODO: Implement logic, possibly calling ShowScopes or other queries.
	return nil
}
