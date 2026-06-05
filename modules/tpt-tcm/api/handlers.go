package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/auth"
	"github.com/PhillipC05/tpt-healthcare/core/consent"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/core/nhi"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-tcm/internal/diagnosis"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-tcm/internal/herb"
	"github.com/google/uuid"
)

// Prescription wraps herb.Prescription with API-level fields.
// The herb package already defines the core Prescription type; this alias
// avoids re-declaring a conflicting struct in the same package.
type Prescription = herb.Prescription

func hashNHI(n string) string {
	h := sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(n))))
	return "sha256:" + hex.EncodeToString(h[:8])
}

func requirePrincipal(w http.ResponseWriter, r *http.Request) (*auth.Principal, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHORIZED", Message: "no authenticated principal"})
		return nil, false
	}
	return p, true
}

func validateNHIParam(w http.ResponseWriter, r *http.Request, param string) (string, bool) {
	v := r.PathValue(param)
	if v == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patient NHI is required"})
		return "", false
	}
	if !nhi.ValidateNHI(v) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "NHI format is invalid"})
		return "", false
	}
	return strings.ToUpper(strings.TrimSpace(v)), true
}

func (s *Server) checkAPC(w http.ResponseWriter, r *http.Request, principal *auth.Principal) bool {
	if !principal.Practitioner || principal.PractitionerID == "" {
		writeJSON(w, http.StatusForbidden, apiError{Code: "NOT_PRACTITIONER", Message: "clinical actions require a registered practitioner identity"})
		return false
	}
	valid, err := s.hpiClient.ValidateAPC(r.Context(), principal.PractitionerID)
	if err != nil {
		s.logger.Error("HPI APC validation failed", slog.String("cpn", principal.PractitionerID), slog.Any("error", err))
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "HPI_UNAVAILABLE", Message: "unable to verify practitioner APC"})
		return false
	}
	if !valid {
		writeJSON(w, http.StatusForbidden, apiError{Code: "APC_INVALID", Message: "practitioner does not hold a current Annual Practising Certificate"})
		return false
	}
	return true
}

func (s *Server) checkConsent(w http.ResponseWriter, r *http.Request, patientNHI string) bool {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant context required"})
		return false
	}
	granted, err := s.consentStore.Check(r.Context(), tenantID, patientNHI, consent.ConsentTypeAccess)
	if err != nil {
		s.logger.Error("consent check failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "CONSENT_CHECK_ERROR", Message: "unable to verify patient consent"})
		return false
	}
	if !granted {
		writeJSON(w, http.StatusForbidden, apiError{Code: "CONSENT_REQUIRED", Message: "patient has not granted access consent"})
		return false
	}
	return true
}

func (s *Server) recordEvent(r *http.Request, principal *auth.Principal, action, resourceType, resourceID, patientNHI string) {
	tenantID, _ := middleware.TenantFromContext(r.Context())
	if err := s.auditTrail.Record(r.Context(), audit.Event{
		TenantID:     tenantID,
		PrincipalID:  principal.ID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		PatientNHI:   hashNHI(patientNHI),
		IPAddress:    r.RemoteAddr,
		OccurredAt:   time.Now().UTC(),
	}); err != nil {
		s.logger.Error("audit record failed", slog.String("resource_type", resourceType), slog.Any("error", err))
	}
}

// ---- Herb Catalog Handlers ----

func (s *Server) handleListHerbs(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, []herb.Herb{})
}

func (s *Server) handleCreateHerb(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var h herb.Herb
	if err := decodeJSON(r, &h); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	h.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	h.CreatedAt = now
	h.UpdatedAt = now
	s.recordEvent(r, principal, "create", "Herb", h.ID, "")
	writeJSON(w, http.StatusCreated, h)
}

func (s *Server) handleGetHerb(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	herbID := r.PathValue("herbId")
	if herbID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_HERB_ID", Message: "herb ID is required"})
		return
	}
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("herb %s not found", herbID)})
}

func (s *Server) handleUpdateHerb(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	herbID := r.PathValue("herbId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var h herb.Herb
	if err := decodeJSON(r, &h); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	h.ID = herbID
	h.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "Herb", herbID, "")
	writeJSON(w, http.StatusOK, h)
}

// ---- Herbal Prescription Handlers ----

func (s *Server) handleListPrescriptions(w http.ResponseWriter, r *http.Request) {
	_, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkConsent(w, r, patientNHI) {
		return
	}
	writeJSON(w, http.StatusOK, []Prescription{})
}

func (s *Server) handleCreatePrescription(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var rx Prescription
	if err := decodeJSON(r, &rx); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(rx.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	rx.ID = uuid.New().String()
	rx.ClinicianID = principal.PractitionerID
	now := time.Now().UnixMilli()
	rx.CreatedAt = now
	rx.UpdatedAt = now
	s.recordEvent(r, principal, "create", "HerbalPrescription", rx.ID, rx.PatientNHI)
	writeJSON(w, http.StatusCreated, rx)
}

func (s *Server) handleGetPrescription(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkConsent(w, r, patientNHI) {
		return
	}
	rxID := r.PathValue("rxId")
	s.recordEvent(r, principal, "read", "HerbalPrescription", rxID, patientNHI)
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("prescription %s not found", rxID)})
}

func (s *Server) handleUpdatePrescription(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	rxID := r.PathValue("rxId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var rx Prescription
	if err := decodeJSON(r, &rx); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	rx.ID = rxID
	rx.PatientNHI = patientNHI
	rx.ClinicianID = principal.PractitionerID
	rx.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "HerbalPrescription", rxID, patientNHI)
	writeJSON(w, http.StatusOK, rx)
}

// ---- TCM Diagnosis Handlers ----

func (s *Server) handleGetDiagnosis(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	if !s.checkConsent(w, r, patientNHI) {
		return
	}
	s.recordEvent(r, principal, "read", "TCMDiagnosis", patientNHI, patientNHI)
	writeJSON(w, http.StatusOK, &diagnosis.TCMDiagnosis{PatientNHI: patientNHI})
}

func (s *Server) handleUpdateDiagnosis(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	patientNHI, ok := validateNHIParam(w, r, "patientNhi")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var diag diagnosis.TCMDiagnosis
	if err := decodeJSON(r, &diag); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	diag.PatientNHI = patientNHI
	diag.ClinicianID = principal.PractitionerID
	diag.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "TCMDiagnosis", patientNHI, patientNHI)
	writeJSON(w, http.StatusOK, diag)
}
