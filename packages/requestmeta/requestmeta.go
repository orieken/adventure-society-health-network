package requestmeta

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
)

const (
	RequestIDHeader     = "X-Request-ID"
	CorrelationIDHeader = "X-Correlation-ID"
)

type contextKey string

const metaContextKey contextKey = "ashn-request-meta"

type Meta struct {
	RequestID     string
	CorrelationID string
}

func Middleware(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		meta := Meta{
			RequestID:     firstNonEmpty(r.Header.Get(RequestIDHeader), newID("req")),
			CorrelationID: firstNonEmpty(r.Header.Get(CorrelationIDHeader), r.Header.Get(RequestIDHeader), newID("corr")),
		}
		w.Header().Set(RequestIDHeader, meta.RequestID)
		w.Header().Set(CorrelationIDHeader, meta.CorrelationID)
		if service != "" {
			log.Printf("[ASHN] service=%s requestId=%s correlationId=%s", service, meta.RequestID, meta.CorrelationID)
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
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return prefix + "-unavailable"
	}
	return prefix + "-" + hex.EncodeToString(bytes[:])
}
