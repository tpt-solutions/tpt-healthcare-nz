package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/auth/jwt"
)

// devAuthHandler implements a dev/test-only local login endpoint that issues
// Ed25519 JWTs for a single fixed practitioner identity, without requiring
// Auth0. It is only wired into the server when Config.DevAuth is true —
// never enable this in production.
type devAuthHandler struct {
	jwtProvider *jwt.Provider
	email       string
	password    string
	tenantID    uuid.UUID
}

// devLoginRequest is the body for POST /api/v1/auth/token.
type devLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// devAuthUser mirrors the AuthUser shape expected by the clinic app's
// AuthContext.
type devAuthUser struct {
	ID     string   `json:"id"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	HPICPN string   `json:"hpiCpn"`
	Roles  []string `json:"roles"`
}

// devLoginResponse is the response body for POST /api/v1/auth/token.
type devLoginResponse struct {
	AccessToken string      `json:"access_token"`
	User        devAuthUser `json:"user"`
	TenantID    string      `json:"tenant_id"`
}

const (
	devPractitionerID     = "dev-practitioner-1"
	devPractitionerName   = "Dev Practitioner"
	devPractitionerHPICPN = "CPN00001"
)

var devPractitionerRoles = []string{"gp"}

// Login handles POST /api/v1/auth/token, issuing a JWT for the fixed
// dev/test practitioner identity when the supplied credentials match.
func (h *devAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req devLoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	if req.Email != h.email || req.Password != h.password {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "INVALID_CREDENTIALS", Message: "invalid credentials"})
		return
	}

	principal := &auth.Principal{
		ID:             devPractitionerID,
		TenantID:       h.tenantID,
		Email:          req.Email,
		Roles:          devPractitionerRoles,
		Practitioner:   true,
		PractitionerID: devPractitionerHPICPN,
	}

	token, err := h.jwtProvider.Issue(r.Context(), principal, 8*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "TOKEN_ERROR", Message: "failed to issue token"})
		return
	}

	writeJSON(w, http.StatusOK, devLoginResponse{
		AccessToken: token,
		User: devAuthUser{
			ID:     devPractitionerID,
			Email:  req.Email,
			Name:   devPractitionerName,
			HPICPN: devPractitionerHPICPN,
			Roles:  devPractitionerRoles,
		},
		TenantID: h.tenantID.String(),
	})
}
