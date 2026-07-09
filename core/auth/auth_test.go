package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	principal *Principal
	err       error
}

func (m *mockProvider) Validate(_ context.Context, _ string) (*Principal, error) {
	return m.principal, m.err
}

func TestPrincipalToContext_PrincipalFromContext(t *testing.T) {
	t.Run("round-trip", func(t *testing.T) {
		p := &Principal{ID: "user-1", TenantID: uuid.New(), Roles: []string{"clinician"}}
		ctx := PrincipalToContext(context.Background(), p)
		got, ok := PrincipalFromContext(ctx)
		require.True(t, ok)
		assert.Equal(t, p, got)
	})

	t.Run("nil principal", func(t *testing.T) {
		ctx := PrincipalToContext(context.Background(), nil)
		got, ok := PrincipalFromContext(ctx)
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("missing key", func(t *testing.T) {
		got, ok := PrincipalFromContext(context.Background())
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

func TestRequireAuth_Middleware(t *testing.T) {
	principal := &Principal{ID: "user-1", TenantID: uuid.New(), Roles: []string{"clinician"}}

	t.Run("valid token", func(t *testing.T) {
		provider := &mockProvider{principal: principal}
		handler := RequireAuth(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFromContext(r.Context())
			require.True(t, ok)
			assert.Equal(t, "user-1", p.ID)
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing Authorization header", func(t *testing.T) {
		provider := &mockProvider{principal: principal}
		handler := RequireAuth(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("non-Bearer scheme", func(t *testing.T) {
		provider := &mockProvider{principal: principal}
		handler := RequireAuth(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("empty token", func(t *testing.T) {
		provider := &mockProvider{principal: principal}
		handler := RequireAuth(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer ")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		provider := &mockProvider{err: assert.AnError}
		handler := RequireAuth(provider)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestRequireRole_Middleware(t *testing.T) {
	t.Run("matching role", func(t *testing.T) {
		handler := RequireRole("clinician")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		p := &Principal{ID: "user-1", Roles: []string{"clinician"}}
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(PrincipalToContext(req.Context(), p))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing role", func(t *testing.T) {
		handler := RequireRole("clinician")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		p := &Principal{ID: "user-1", Roles: []string{"nurse"}}
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(PrincipalToContext(req.Context(), p))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("no principal in context", func(t *testing.T) {
		handler := RequireRole("clinician")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
