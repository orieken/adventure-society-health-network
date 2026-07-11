package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"ashn/packages/asyncjobs"
	"ashn/packages/requestmeta"

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

	startHealthServer(env("TX_WORKER_ADDR", ":8084"))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		process(db, batchSize)
		<-ticker.C
	}
}

func startHealthServer(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	go func() {
		log.Printf("[ASHN] tx-worker health listening on %s", addr)
		if err := http.ListenAndServe(addr, requestmeta.Middleware("tx-worker", mux)); err != nil {
			log.Printf("[ASHN] tx-worker health server stopped: %v", err)
		}
	}()
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"service": "tx-worker", "status": "ok"})
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

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
