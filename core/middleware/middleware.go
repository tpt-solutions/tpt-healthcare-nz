// Package middleware provides HTTP middleware for a NZ healthcare API.
package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// contextKey is an unexported type for context keys in this package.
type contextKey int

// TenantKey is the context key used to store the tenant UUID.
const TenantKey contextKey = iota

// auditTrailer is implemented by any value that can record an audit Event.
type auditTrailer interface {
	Record(ctx context.Context, e audit.Event) error
}

// TenantFromContext retrieves the tenant UUID stored by TenantExtraction middleware.
// The second return value is false when no tenant UUID is present in the context.
func TenantFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(TenantKey).(uuid.UUID)
	return v, ok
}

// PrincipalFromContext is a compatibility shim that delegates to auth.PrincipalFromContext.
func PrincipalFromContext(ctx context.Context) (*auth.Principal, bool) {
	return auth.PrincipalFromContext(ctx)
}

// RateLimit returns middleware that enforces a token-bucket rate limit of rps
// requests per second with a maximum burst of burst tokens.
// Requests that exceed the limit receive a 429 Too Many Requests response.
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Periodically evict stale limiter entries to avoid unbounded memory growth.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > 10*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		c, ok := clients[ip]
		if !ok {
			c = &client{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
			clients[ip] = c
		}
		c.lastSeen = time.Now()
		return c.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := realIP(r)
			if !getLimiter(ip).Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CORS returns middleware that adds CORS headers. allowedOrigins is the list of
// permitted origins; use ["*"] to allow all origins. Preflight OPTIONS requests
// are handled and responded to immediately with a 204 No Content.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	wildcard := false
	for _, o := range allowedOrigins {
		if o == "*" {
			wildcard = true
			break
		}
		originSet[o] = struct{}{}
	}

	isAllowed := func(origin string) bool {
		if wildcard {
			return true
		}
		_, ok := originSet[origin]
		return ok
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID, X-Request-ID")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			// Handle preflight.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TenantExtraction returns middleware that reads the X-Tenant-ID header, parses it
// as a UUID, and stores it in the request context under TenantKey. Requests with a
// missing or malformed header receive a 400 Bad Request response.
func TenantExtraction() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get("X-Tenant-ID")
			if raw == "" {
				http.Error(w, "missing X-Tenant-ID header", http.StatusBadRequest)
				return
			}
			tenantID, err := uuid.Parse(raw)
			if err != nil {
				http.Error(w, "invalid X-Tenant-ID: must be a valid UUID", http.StatusBadRequest)
				return
			}
			ctx := context.WithValue(r.Context(), TenantKey, tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// responseRecorder is a minimal http.ResponseWriter wrapper that captures the
// status code so it can be reported to the audit trail.
type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

// AuditWrap returns middleware that records an audit.Event after each response is
// written. The tenant UUID and principal are sourced from the request context when
// available; fields that cannot be determined are left as zero values.
func AuditWrap(trail auditTrailer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()

			next.ServeHTTP(rr, r)

			tenantID, _ := TenantFromContext(r.Context())

			e := audit.Event{
				TenantID:     tenantID,
				Action:       actionFromMethod(r.Method),
				ResourceType: r.URL.Path,
				IPAddress:    realIP(r),
				UserAgent:    r.UserAgent(),
				OccurredAt:   start,
				Details: map[string]any{
					"method":      r.Method,
					"path":        r.URL.Path,
					"status_code": rr.status,
					"duration_ms": time.Since(start).Milliseconds(),
				},
			}

			if err := trail.Record(r.Context(), e); err != nil {
				log.Printf("audit: failed to record event: %v", err)
			}
		})
	}
}

// RecoveryMiddleware returns middleware that recovers from panics in downstream
// handlers and responds with 500 Internal Server Error.
func RecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("recovered from panic: %v", rec)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// realIP extracts the best available client IP from a request, consulting
// X-Forwarded-For and X-Real-IP before falling back to RemoteAddr.
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For may be a comma-separated list; the leftmost is the client.
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// actionFromMethod maps an HTTP method to an audit action string.
func actionFromMethod(method string) string {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		return "read"
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return strings.ToLower(method)
	}
}
