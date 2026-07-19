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
	request.Header.Set(TraceparentHeader, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	request.Header.Set(TracestateHeader, "ashn=demo")
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "req-test", captured.RequestID)
	assert.Equal(t, "corr-test", captured.CorrelationID)
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", captured.TraceID)
	assert.NotEmpty(t, captured.SpanID)
	assert.Equal(t, "00f067aa0ba902b7", captured.ParentSpanID)
	assert.Equal(t, "ashn=demo", captured.Tracestate)
	assert.Equal(t, "req-test", response.Header().Get(RequestIDHeader))
	assert.Equal(t, "corr-test", response.Header().Get(CorrelationIDHeader))
	assert.Contains(t, response.Header().Get(TraceparentHeader), "00-4bf92f3577b34da6a3ce929d0e0e4736-")
	assert.Equal(t, "ashn=demo", response.Header().Get(TracestateHeader))
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
	assert.NotEmpty(t, response.Header().Get(TraceparentHeader))
	assert.Equal(t, response.Header().Get(RequestIDHeader), outbound.Header.Get(RequestIDHeader))
	assert.Equal(t, response.Header().Get(CorrelationIDHeader), outbound.Header.Get(CorrelationIDHeader))
	assert.Equal(t, response.Header().Get(TraceparentHeader), outbound.Header.Get(TraceparentHeader))
	assert.Contains(t, LogFields(outbound), "requestId=")
}

func TestInvalidTraceparentStartsNewTrace(t *testing.T) {
	var captured Meta
	handler := Middleware("", http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = FromRequest(r)
	}))

	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set(TraceparentHeader, "bad-trace")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	assert.Len(t, captured.TraceID, 32)
	assert.Len(t, captured.SpanID, 16)
	assert.Empty(t, captured.ParentSpanID)
}

func TestPropagateCopiesExistingTraceparentWithoutMiddleware(t *testing.T) {
	inbound := httptest.NewRequest(http.MethodGet, "/upstream", nil)
	inbound.Header.Set(TraceparentHeader, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	outbound := httptest.NewRequest(http.MethodGet, "/downstream", nil)

	Propagate(inbound, outbound)

	assert.Equal(t, inbound.Header.Get(TraceparentHeader), outbound.Header.Get(TraceparentHeader))
}

func TestFormatTraceparentRequiresTraceAndSpan(t *testing.T) {
	assert.Empty(t, formatTraceparent("", "00f067aa0ba902b7"))
	assert.Empty(t, formatTraceparent("4bf92f3577b34da6a3ce929d0e0e4736", ""))
	assert.Equal(t, "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", formatTraceparent("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7"))
}
