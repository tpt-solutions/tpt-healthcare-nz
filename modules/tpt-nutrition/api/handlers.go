package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	apcStatus, err := s.hpiClient.ValidateAPC(r.Context(), principal.PractitionerID)
	if err != nil {
		s.logger.Error("HPI APC validation failed", slog.String("cpn", principal.PractitionerID), slog.Any("error", err))
		writeJSON(w, http.StatusServiceUnavailable, apiError{Code: "HPI_UNAVAILABLE", Message: "unable to verify practitioner APC"})
		return false
	}
	if !apcStatus.Valid {
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
	entries, err := s.listDiaryEntries(r.Context(), patientNHI)
	if err != nil {
		s.logger.Error("list diary entries failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to retrieve food diary entries"})
		return
	}
	writeJSON(w, http.StatusOK, entries)
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
	var entry fooddiary.FoodDiaryEntry
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
	created, err := s.createDiaryEntry(r.Context(), entry)
	if err != nil {
		s.logger.Error("create diary entry failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to save food diary entry"})
		return
	}
	s.recordEvent(r, principal, "create", "FoodDiaryEntry", created.ID, created.PatientNHI)
	writeJSON(w, http.StatusCreated, created)
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
	entry, err := s.getDiaryEntry(r.Context(), entryID, patientNHI)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("diary entry %s not found", entryID)})
			return
		}
		s.logger.Error("get diary entry failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to retrieve food diary entry"})
		return
	}
	s.recordEvent(r, principal, "read", "FoodDiaryEntry", entryID, patientNHI)
	writeJSON(w, http.StatusOK, entry)
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
	var entry fooddiary.FoodDiaryEntry
	if err := decodeJSON(r, &entry); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	entry.ID = entryID
	entry.PatientNHI = patientNHI
	entry.UpdatedAt = time.Now().UnixMilli()
	updated, err := s.updateDiaryEntry(r.Context(), entry)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("diary entry %s not found", entryID)})
			return
		}
		s.logger.Error("update diary entry failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to update food diary entry"})
		return
	}
	s.recordEvent(r, principal, "update", "FoodDiaryEntry", entryID, patientNHI)
	writeJSON(w, http.StatusOK, updated)
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
	plans, err := s.listMealPlans(r.Context(), patientNHI)
	if err != nil {
		s.logger.Error("list meal plans failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to retrieve meal plans"})
		return
	}
	writeJSON(w, http.StatusOK, plans)
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
	var plan mealplan.MealPlan
	if err := decodeJSON(r, &plan); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	if !nhi.ValidateNHI(plan.PatientNHI) {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_NHI", Message: "patientNhi format is invalid"})
		return
	}
	plan.ID = uuid.New().String()
	plan.ClinicianID = principal.PractitionerID
	now := time.Now().UnixMilli()
	plan.CreatedAt = now
	plan.UpdatedAt = now
	created, err := s.createMealPlan(r.Context(), plan)
	if err != nil {
		s.logger.Error("create meal plan failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to save meal plan"})
		return
	}
	s.recordEvent(r, principal, "create", "MealPlan", created.ID, created.PatientNHI)
	writeJSON(w, http.StatusCreated, created)
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
	plan, err := s.getMealPlan(r.Context(), planID, patientNHI)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("meal plan %s not found", planID)})
			return
		}
		s.logger.Error("get meal plan failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to retrieve meal plan"})
		return
	}
	s.recordEvent(r, principal, "read", "MealPlan", planID, patientNHI)
	writeJSON(w, http.StatusOK, plan)
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
	var plan mealplan.MealPlan
	if err := decodeJSON(r, &plan); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: err.Error()})
		return
	}
	plan.ID = planID
	plan.PatientNHI = patientNHI
	plan.ClinicianID = principal.PractitionerID
	plan.UpdatedAt = time.Now().UnixMilli()
	updated, err := s.updateMealPlan(r.Context(), plan)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: fmt.Sprintf("meal plan %s not found", planID)})
			return
		}
		s.logger.Error("update meal plan failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to update meal plan"})
		return
	}
	s.recordEvent(r, principal, "update", "MealPlan", planID, patientNHI)
	writeJSON(w, http.StatusOK, updated)
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
	bc, err := s.getLatestBodyComp(r.Context(), patientNHI)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusOK, &bodycomp.BodyComposition{PatientNHI: patientNHI})
			return
		}
		s.logger.Error("get body composition failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to retrieve body composition"})
		return
	}
	s.recordEvent(r, principal, "read", "BodyComposition", patientNHI, patientNHI)
	writeJSON(w, http.StatusOK, bc)
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
	bc.ClinicianID = principal.PractitionerID
	now := time.Now().UnixMilli()
	bc.CreatedAt = now
	bc.UpdatedAt = now
	created, err := s.createBodyComp(r.Context(), bc)
	if err != nil {
		s.logger.Error("create body composition failed", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "unable to save body composition"})
		return
	}
	s.recordEvent(r, principal, "create", "BodyComposition", created.ID, created.PatientNHI)
	writeJSON(w, http.StatusCreated, created)
}
