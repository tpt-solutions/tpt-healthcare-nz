package rbac

import (
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
)

// RequirePermission returns an http.Handler middleware that calls
// checker.CanAccess and returns 403 Forbidden if the check fails.
// It sits alongside the existing auth.RequireRole middleware and should
// be applied on a per-route basis for fine-grained resource control.
//
// Example:
//
//	mux.Handle("GET /api/v1/encounters/{id}",
//	    rbac.RequirePermission(checker, "encounter", "read")(encounterHandler))
func RequirePermission(checker *Checker, resource, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if !checker.CanAccess(r.Context(), principal, resource, action) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
