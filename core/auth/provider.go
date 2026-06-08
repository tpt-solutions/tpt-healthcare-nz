package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// Principal represents an authenticated identity in the tpt-healthcare system.
// PractitionerID holds the HPI CPN (Health Provider Index Common Person Number)
// when the principal is a practitioner.
type Principal struct {
	ID             string
	TenantID       uuid.UUID
	Email          string
	Roles          []string
	Practitioner   bool
	PractitionerID string    // HPI CPN
	// DepartmentIDs lists the departments this principal has an active role
	// assignment for. Populated at JWT issuance from the role_assignments table.
	// An empty slice means tenant-wide access for all assigned roles.
	DepartmentIDs []uuid.UUID
}

// Provider validates an opaque token and returns the authenticated Principal.
type Provider interface {
	Validate(ctx context.Context, token string) (*Principal, error)
}

// Common errors returned by auth provider implementations.
var (
	ErrInvalidConfig   = errors.New("auth: invalid configuration")
	ErrNotImplemented  = errors.New("auth: not implemented")
)

// contextKey is an unexported type for context keys in this package, preventing
// collisions with keys from other packages.
type contextKey int

const (
	principalContextKey contextKey = iota
)

// PrincipalToContext returns a new context carrying the provided Principal.
func PrincipalToContext(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalContextKey, p)
}

// PrincipalFromContext retrieves the Principal stored in ctx, if any.
// The second return value reports whether a Principal was present.
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalContextKey).(*Principal)
	return p, ok && p != nil
}

// RequireAuth returns HTTP middleware that validates a Bearer token from the
// Authorization header using the supplied Provider. On success the resolved
// Principal is stored in the request context. On failure a 401 is written and
// the chain is short-circuited.
func RequireAuth(provider Provider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "unauthorized: missing Authorization header", http.StatusUnauthorized)
				return
			}

			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				http.Error(w, "unauthorized: Authorization header must use Bearer scheme", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, bearerPrefix)
			if token == "" {
				http.Error(w, "unauthorized: empty token", http.StatusUnauthorized)
				return
			}

			principal, err := provider.Validate(r.Context(), token)
			if err != nil {
				http.Error(w, "unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := PrincipalToContext(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns HTTP middleware that ensures the request's Principal has
// the specified role. It must be used downstream of RequireAuth. Missing
// principal or missing role results in a 403 response.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			principal, ok := PrincipalFromContext(req.Context())
			if !ok {
				http.Error(w, "forbidden: no authenticated principal", http.StatusForbidden)
				return
			}

			for _, assigned := range principal.Roles {
				if assigned == role {
					next.ServeHTTP(w, req)
					return
				}
			}

			http.Error(w, "forbidden: role "+role+" required", http.StatusForbidden)
		})
	}
}
