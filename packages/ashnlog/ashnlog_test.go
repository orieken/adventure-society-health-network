package ashnlog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"ashn/packages/requestmeta"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfoWritesStructuredJSON(t *testing.T) {
	var buffer bytes.Buffer
	previous := logger
	logger = slog.New(slog.NewJSONHandler(&buffer, nil))
	defer func() { logger = previous }()

	Info("demo_event", "service", "api-gateway", "count", 2)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(buffer.Bytes(), &payload))
	assert.Equal(t, "INFO", payload["level"])
	assert.Equal(t, "demo_event", payload["msg"])
	assert.Equal(t, "api-gateway", payload["service"])
	assert.Equal(t, float64(2), payload["count"])
}

func TestErrorWritesStructuredJSONWithError(t *testing.T) {
	var buffer bytes.Buffer
	previous := logger
	logger = slog.New(slog.NewJSONHandler(&buffer, nil))
	defer func() { logger = previous }()

	Error("demo_failed", assert.AnError, "service", "payer-core")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(buffer.Bytes(), &payload))
	assert.Equal(t, "ERROR", payload["level"])
	assert.Equal(t, "demo_failed", payload["msg"])
	assert.Equal(t, "payer-core", payload["service"])
	assert.Contains(t, payload["error"], "assert.AnError")
}

func TestRequestIncludesTraceFields(t *testing.T) {
	var buffer bytes.Buffer
	previous := logger
	logger = slog.New(slog.NewJSONHandler(&buffer, nil))
	defer func() { logger = previous }()

	request := httptest.NewRequest(http.MethodPost, "/v1/x12/xml", nil)
	request.Header.Set(requestmeta.RequestIDHeader, "req-1")
	request.Header.Set(requestmeta.CorrelationIDHeader, "corr-1")
	handler := requestmeta.Middleware("", http.HandlerFunc(func(_ http.ResponseWriter, tracedRequest *http.Request) {
		Request("api-gateway", tracedRequest)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(buffer.Bytes(), &payload))
	assert.Equal(t, "http_request", payload["msg"])
	assert.Equal(t, "api-gateway", payload["service"])
	assert.Equal(t, "POST", payload["method"])
	assert.Equal(t, "/v1/x12/xml", payload["path"])
	assert.Equal(t, "req-1", payload["requestId"])
	assert.Equal(t, "corr-1", payload["correlationId"])
	assert.NotEmpty(t, payload["traceId"])
	assert.NotEmpty(t, payload["spanId"])
}
