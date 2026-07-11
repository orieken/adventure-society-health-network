package requestmeta

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	RequestIDHeader     = "X-Request-ID"
	CorrelationIDHeader = "X-Correlation-ID"
	TraceparentHeader   = "traceparent"
	TracestateHeader    = "tracestate"
)

type contextKey string

const metaContextKey contextKey = "ashn-request-meta"

type Meta struct {
	RequestID     string
	CorrelationID string
	TraceID       string
	SpanID        string
	ParentSpanID  string
	Traceparent   string
	Tracestate    string
}

func Middleware(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID, parentSpanID, ok := parseTraceparent(r.Header.Get(TraceparentHeader))
		if !ok {
			traceID = newHexID(16)
		}
		spanID := newHexID(8)
		meta := Meta{
			RequestID:     firstNonEmpty(r.Header.Get(RequestIDHeader), newID("req")),
			CorrelationID: firstNonEmpty(r.Header.Get(CorrelationIDHeader), r.Header.Get(RequestIDHeader), newID("corr")),
			TraceID:       traceID,
			SpanID:        spanID,
			ParentSpanID:  parentSpanID,
			Traceparent:   formatTraceparent(traceID, spanID),
			Tracestate:    strings.TrimSpace(r.Header.Get(TracestateHeader)),
		}
		w.Header().Set(RequestIDHeader, meta.RequestID)
		w.Header().Set(CorrelationIDHeader, meta.CorrelationID)
		w.Header().Set(TraceparentHeader, meta.Traceparent)
		if meta.Tracestate != "" {
			w.Header().Set(TracestateHeader, meta.Tracestate)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), metaContextKey, meta)))
	})
}

func FromRequest(r *http.Request) Meta {
	if r == nil {
		return Meta{}
	}
	if meta, ok := r.Context().Value(metaContextKey).(Meta); ok {
		return meta
	}
	return Meta{
		RequestID:     strings.TrimSpace(r.Header.Get(RequestIDHeader)),
		CorrelationID: strings.TrimSpace(r.Header.Get(CorrelationIDHeader)),
		Traceparent:   strings.TrimSpace(r.Header.Get(TraceparentHeader)),
		Tracestate:    strings.TrimSpace(r.Header.Get(TracestateHeader)),
	}
}

func Propagate(inbound *http.Request, outbound *http.Request) {
	if inbound == nil || outbound == nil {
		return
	}
	meta := FromRequest(inbound)
	if meta.RequestID != "" {
		outbound.Header.Set(RequestIDHeader, meta.RequestID)
	}
	if meta.CorrelationID != "" {
		outbound.Header.Set(CorrelationIDHeader, meta.CorrelationID)
	}
	if meta.TraceID != "" && meta.SpanID != "" {
		outbound.Header.Set(TraceparentHeader, formatTraceparent(meta.TraceID, meta.SpanID))
	} else if meta.Traceparent != "" {
		outbound.Header.Set(TraceparentHeader, meta.Traceparent)
	}
	if meta.Tracestate != "" {
		outbound.Header.Set(TracestateHeader, meta.Tracestate)
	}
}

func LogFields(r *http.Request) string {
	meta := FromRequest(r)
	fields := []string{}
	if meta.RequestID != "" {
		fields = append(fields, "requestId="+meta.RequestID)
	}
	if meta.CorrelationID != "" {
		fields = append(fields, "correlationId="+meta.CorrelationID)
	}
	if meta.TraceID != "" {
		fields = append(fields, "traceId="+meta.TraceID)
	}
	if meta.SpanID != "" {
		fields = append(fields, "spanId="+meta.SpanID)
	}
	return strings.Join(fields, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func newID(prefix string) string {
	return prefix + "-" + newHexID(16)
}

func newHexID(byteCount int) string {
	bytes := make([]byte, byteCount)
	if _, err := rand.Read(bytes); err != nil {
		return strings.Repeat("0", byteCount*2)
	}
	return hex.EncodeToString(bytes)
}

func parseTraceparent(value string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 4 || parts[0] != "00" || len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return "", "", false
	}
	if !isLowerHex(parts[1]) || !isLowerHex(parts[2]) || !isLowerHex(parts[3]) || allZero(parts[1]) || allZero(parts[2]) {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func formatTraceparent(traceID, spanID string) string {
	if traceID == "" || spanID == "" {
		return ""
	}
	return "00-" + traceID + "-" + spanID + "-01"
}

func isLowerHex(value string) bool {
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}

func allZero(value string) bool {
	for _, char := range value {
		if char != '0' {
			return false
		}
	}
	return true
}
