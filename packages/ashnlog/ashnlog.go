package ashnlog

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"ashn/packages/requestmeta"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

func Info(message string, args ...any) {
	logger.Info(message, args...)
}

func Error(message string, err error, args ...any) {
	values := append([]any{}, args...)
	if err != nil {
		values = append(values, "error", err.Error())
	}
	logger.Error(message, values...)
}

func Fatal(message string, err error, args ...any) {
	Error(message, err, args...)
	os.Exit(1)
}

func Request(service string, r *http.Request, args ...any) {
	values := []any{
		"service", service,
		"method", r.Method,
		"path", r.URL.Path,
	}
	meta := requestmeta.FromRequest(r)
	if meta.RequestID != "" {
		values = append(values, "requestId", meta.RequestID)
	}
	if meta.CorrelationID != "" {
		values = append(values, "correlationId", meta.CorrelationID)
	}
	values = append(values, args...)
	logger.InfoContext(context.Background(), "http_request", values...)
}
