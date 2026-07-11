package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"ashn/packages/ashnlog"

	_ "github.com/lib/pq"
)

func main() {
	if err := runMigrationFromEnv(); err != nil {
		ashnlog.Fatal("migration_failed", err, "service", "migrate")
	}
}

func runMigrationFromEnv() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("[ASHN] DATABASE_URL is required for migrations")
	}

	migrationPath := filepath.Join("infra", "migrations", "000001_init.up.sql")
	if value := os.Getenv("ASHN_MIGRATION_PATH"); value != "" {
		migrationPath = value
	}
	return runMigration(dsn, migrationPath, sql.Open)
}

func runMigration(dsn, migrationPath string, openDB func(string, string) (*sql.DB, error)) error {
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("[ASHN] read migration failed: %w", err)
	}

	db, err := openDB("postgres", dsn)
	if err != nil {
		return fmt.Errorf("[ASHN] postgres open failed: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("[ASHN] postgres ping failed: %w", err)
	}

	if _, err := db.Exec(string(migration)); err != nil {
		return fmt.Errorf("[ASHN] migration failed: %w", err)
	}
	ashnlog.Info("migration_applied", "service", "migrate", "path", migrationPath)
	return nil
}
