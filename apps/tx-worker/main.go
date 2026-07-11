package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"time"

	"ashn/packages/ashnlog"
	"ashn/packages/asyncjobs"
	"ashn/packages/requestmeta"

	_ "github.com/lib/pq"
)

func main() {
	db := openDB()
	if db == nil {
		ashnlog.Fatal("database_url_required", nil, "service", "tx-worker")
	}
	defer db.Close()

	interval := envDuration("TX_WORKER_INTERVAL", time.Second)
	batchSize := envInt("TX_WORKER_BATCH_SIZE", 5)
	ashnlog.Info("worker_polling_started", "service", "tx-worker", "interval", interval.String(), "batchSize", batchSize)

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
		ashnlog.Info("service_listening", "service", "tx-worker", "addr", addr)
		if err := http.ListenAndServe(addr, requestmeta.Middleware("tx-worker", mux)); err != nil {
			ashnlog.Error("health_server_stopped", err, "service", "tx-worker")
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
		ashnlog.Error("worker_poll_failed", err, "service", "tx-worker")
		return
	}
	if processed > 0 {
		ashnlog.Info("worker_processed_jobs", "service", "tx-worker", "count", processed)
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
		ashnlog.Error("postgres_open_failed", err, "service", "tx-worker")
		return nil
	}
	if err := db.Ping(); err != nil {
		ashnlog.Error("postgres_ping_failed", err, "service", "tx-worker")
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
		ashnlog.Error("invalid_duration_env_using_fallback", err, "service", "tx-worker", "key", key, "value", value, "fallback", fallback.String())
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
		ashnlog.Error("invalid_int_env_using_fallback", err, "service", "tx-worker", "key", key, "value", value, "fallback", fallback)
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
