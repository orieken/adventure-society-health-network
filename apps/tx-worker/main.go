package main

import (
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"

	"ashn/packages/asyncjobs"

	_ "github.com/lib/pq"
)

func main() {
	db := openDB()
	if db == nil {
		log.Fatal("[ASHN] tx-worker requires DATABASE_URL")
	}
	defer db.Close()

	interval := envDuration("TX_WORKER_INTERVAL", time.Second)
	batchSize := envInt("TX_WORKER_BATCH_SIZE", 5)
	log.Printf("[ASHN] tx-worker polling every %s with batch size %d", interval, batchSize)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		process(db, batchSize)
		<-ticker.C
	}
}

func process(db *sql.DB, batchSize int) {
	processed, err := asyncjobs.ProcessDue(db, batchSize)
	if err != nil {
		log.Printf("[ASHN] tx-worker poll failed: %v", err)
		return
	}
	if processed > 0 {
		log.Printf("[ASHN] tx-worker processed %d job(s)", processed)
	}
}

func openDB() *sql.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil
	}
	return openDBWith(dsn, sql.Open)
}

func openDBWith(dsn string, open func(string, string) (*sql.DB, error)) *sql.DB {
	db, err := open("postgres", dsn)
	if err != nil {
		log.Printf("[ASHN] postgres open failed: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		log.Printf("[ASHN] postgres ping failed: %v", err)
		_ = db.Close()
		return nil
	}
	return db
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("[ASHN] invalid %s=%q; using %s", key, value, fallback)
		return fallback
	}
	return duration
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Printf("[ASHN] invalid %s=%q; using %d", key, value, fallback)
		return fallback
	}
	return parsed
}
