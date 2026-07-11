package requestmeta

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMiddlewareUsesIncomingIDsAndSetsResponseHeaders(t *testing.T) {
	var captured Meta
	handler := Middleware("test-service", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = FromRequest(r)
		w.WriteHeader(http.StatusNoContent)
	}))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(RequestIDHeader, "req-test")
	request.Header.Set(CorrelationIDHeader, "corr-test")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "req-test", captured.RequestID)
	assert.Equal(t, "corr-test", captured.CorrelationID)
	assert.Equal(t, "req-test", response.Header().Get(RequestIDHeader))
	assert.Equal(t, "corr-test", response.Header().Get(CorrelationIDHeader))
}

func TestMiddlewareGeneratesIDsAndPropagatesThem(t *testing.T) {
	var outbound *http.Request
	handler := Middleware("", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		outbound = httptest.NewRequest(http.MethodPost, "/downstream", nil)
		Propagate(r, outbound)
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/upstream", nil))

	assert.NotEmpty(t, response.Header().Get(RequestIDHeader))
	assert.NotEmpty(t, response.Header().Get(CorrelationIDHeader))
	assert.Equal(t, response.Header().Get(RequestIDHeader), outbound.Header.Get(RequestIDHeader))
	assert.Equal(t, response.Header().Get(CorrelationIDHeader), outbound.Header.Get(CorrelationIDHeader))
	assert.Contains(t, LogFields(outbound), "requestId=")
}
