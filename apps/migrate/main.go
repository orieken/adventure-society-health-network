package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("[ASHN] DATABASE_URL is required for migrations")
	}

	migrationPath := filepath.Join("infra", "migrations", "000001_init.up.sql")
	if value := os.Getenv("ASHN_MIGRATION_PATH"); value != "" {
		migrationPath = value
	}

	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		log.Fatalf("[ASHN] read migration failed: %v", err)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[ASHN] postgres open failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("[ASHN] postgres ping failed: %v", err)
	}

	if _, err := db.Exec(string(migration)); err != nil {
		log.Fatalf("[ASHN] migration failed: %v", err)
	}
	log.Println("[ASHN] migration applied")
}
