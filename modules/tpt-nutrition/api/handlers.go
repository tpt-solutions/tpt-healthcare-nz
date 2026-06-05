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
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-nutrition/internal/bodycomp"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-nutrition/internal/fooddiary"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-nutrition/internal/mealplan"
	"github.com/google/uuid"
)

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

// ---- Food Diary Handlers ----

func (s *Server) handleListDiaryEntries(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, []fooddiary.Entry{})
}

func (s *Server) handleCreateDiaryEntry(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var entry fooddiary.Entry
	if err := decodeJSON(r, &entry); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(entry.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	entry.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	entry.CreatedAt = now
	entry.UpdatedAt = now
	s.recordEvent(r, principal, "create", "FoodDiaryEntry", entry.ID, entry.PatientNHI)
	writeJSON(w, http.StatusCreated, entry)
}

func (s *Server) handleGetDiaryEntry(w http.ResponseWriter, r *http.Request) {
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
	entryID := r.PathValue("entryId")
	s.recordEvent(r, principal, "read", "FoodDiaryEntry", entryID, patientNHI)
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("diary entry %s not found", entryID)})
}

func (s *Server) handleUpdateDiaryEntry(w http.ResponseWriter, r *http.Request) {
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
	entryID := r.PathValue("entryId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var entry fooddiary.Entry
	if err := decodeJSON(r, &entry); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	entry.ID = entryID
	entry.PatientNHI = patientNHI
	entry.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "FoodDiaryEntry", entryID, patientNHI)
	writeJSON(w, http.StatusOK, entry)
}

// ---- Meal Plan Handlers ----

func (s *Server) handleListMealPlans(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, []mealplan.Plan{})
}

func (s *Server) handleCreateMealPlan(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var plan mealplan.Plan
	if err := decodeJSON(r, &plan); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(plan.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	plan.ID = uuid.New().String()
	plan.CreatedBy = principal.PractitionerID
	now := time.Now().UnixMilli()
	plan.CreatedAt = now
	plan.UpdatedAt = now
	s.recordEvent(r, principal, "create", "MealPlan", plan.ID, plan.PatientNHI)
	writeJSON(w, http.StatusCreated, plan)
}

func (s *Server) handleGetMealPlan(w http.ResponseWriter, r *http.Request) {
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
	planID := r.PathValue("planId")
	s.recordEvent(r, principal, "read", "MealPlan", planID, patientNHI)
	writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("meal plan %s not found", planID)})
}

func (s *Server) handleUpdateMealPlan(w http.ResponseWriter, r *http.Request) {
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
	planID := r.PathValue("planId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var plan mealplan.Plan
	if err := decodeJSON(r, &plan); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	plan.ID = planID
	plan.PatientNHI = patientNHI
	plan.CreatedBy = principal.PractitionerID
	plan.UpdatedAt = time.Now().UnixMilli()
	s.recordEvent(r, principal, "update", "MealPlan", planID, patientNHI)
	writeJSON(w, http.StatusOK, plan)
}

// ---- Body Composition Handlers ----

func (s *Server) handleGetBodyComp(w http.ResponseWriter, r *http.Request) {
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
	s.recordEvent(r, principal, "read", "BodyComposition", patientNHI, patientNHI)
	writeJSON(w, http.StatusOK, &bodycomp.BodyComposition{PatientNHI: patientNHI})
}

func (s *Server) handleCreateBodyComp(w http.ResponseWriter, r *http.Request) {
	principal, ok := requirePrincipal(w, r)
	if !ok {
		return
	}
	if !s.checkAPC(w, r, principal) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var bc bodycomp.BodyComposition
	if err := decodeJSON(r, &bc); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(bc.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	bc.ID = uuid.New().String()
	bc.MeasuredBy = principal.PractitionerID
	now := time.Now().UnixMilli()
	bc.CreatedAt = now
	bc.UpdatedAt = now
	s.recordEvent(r, principal, "create", "BodyComposition", bc.ID, bc.PatientNHI)
	writeJSON(w, http.StatusCreated, bc)
}
