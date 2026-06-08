package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/internal/homevisit"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// createHomeVisit creates a new home visit.
func (s *Server) createHomeVisit(w http.ResponseWriter, r *http.Request) {
	var v homevisit.HomeVisit
	if err := decodeJSON(r, &v); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	v.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	v.CreatedAt = now
	v.UpdatedAt = now
	if err := v.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.visitRepo.Create(r.Context(), &v); err != nil {
		s.logger.Error("create home visit failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create home visit"})
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

// getHomeVisit retrieves a home visit by ID.
func (s *Server) getHomeVisit(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	v, err := s.visitRepo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Code: "not_found", Message: "home visit not found"})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// listHomeVisits lists home visits with filters.
func (s *Server) listHomeVisits(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset := parsePagination(r)
	visits, total, err := s.visitRepo.List(r.Context(), q.Get("patient_nhi"), q.Get("clinician_id"), q.Get("status"), q.Get("visit_type"), limit, offset)
	if err != nil {
		s.logger.Error("list home visits failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list home visits"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": visits, "limit": limit, "offset": offset, "total": total,
		"filters": map[string]string{
			"patient_nhi":  q.Get("patient_nhi"),
			"clinician_id": q.Get("clinician_id"),
			"status":       q.Get("status"),
			"visit_type":   q.Get("visit_type"),
		},
	})
}

// updateHomeVisit updates a home visit.
func (s *Server) updateHomeVisit(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var v homevisit.HomeVisit
	if err := decodeJSON(r, &v); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	v.ID = id
	v.UpdatedAt = time.Now().UnixMilli()
	if err := v.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.visitRepo.Update(r.Context(), &v); err != nil {
		s.logger.Error("update home visit failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to update home visit"})
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// deleteHomeVisit deletes a home visit.
func (s *Server) deleteHomeVisit(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.visitRepo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to delete home visit"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// createVisitNote creates a new visit note.
func (s *Server) createVisitNote(w http.ResponseWriter, r *http.Request) {
	var n homevisit.HomeVisitNote
	if err := decodeJSON(r, &n); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	n.ID = uuid.New().String()
	n.HomeVisitID = mux.Vars(r)["id"]
	now := time.Now().UnixMilli()
	n.CreatedAt = now
	n.UpdatedAt = now
	if err := n.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.visitRepo.CreateNote(r.Context(), &n); err != nil {
		s.logger.Error("create visit note failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create note"})
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

// listVisitNotes lists visit notes.
func (s *Server) listVisitNotes(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	notes, err := s.visitRepo.ListNotes(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list notes"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": notes, "visitId": id})
}

// createSafetyCheck creates a new safety check.
func (s *Server) createSafetyCheck(w http.ResponseWriter, r *http.Request) {
	var sc homevisit.SafetyCheck
	if err := decodeJSON(r, &sc); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	sc.ID = uuid.New().String()
	sc.HomeVisitID = mux.Vars(r)["id"]
	sc.CreatedAt = time.Now().UnixMilli()
	if err := sc.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.visitRepo.CreateSafetyCheck(r.Context(), &sc); err != nil {
		s.logger.Error("create safety check failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create safety check"})
		return
	}
	writeJSON(w, http.StatusCreated, sc)
}

// listSafetyChecks lists safety checks.
func (s *Server) listSafetyChecks(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	checks, err := s.visitRepo.ListSafetyChecks(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list safety checks"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": checks, "visitId": id})
}

// createEquipmentCheck creates a new equipment check.
func (s *Server) createEquipmentCheck(w http.ResponseWriter, r *http.Request) {
	var ec homevisit.EquipmentCheck
	if err := decodeJSON(r, &ec); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	ec.ID = uuid.New().String()
	ec.HomeVisitID = mux.Vars(r)["id"]
	now := time.Now().UnixMilli()
	ec.CreatedAt = now
	ec.UpdatedAt = now
	if err := ec.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.visitRepo.CreateEquipmentCheck(r.Context(), &ec); err != nil {
		s.logger.Error("create equipment check failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create equipment check"})
		return
	}
	writeJSON(w, http.StatusCreated, ec)
}

// listEquipmentChecks lists equipment checks.
func (s *Server) listEquipmentChecks(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	checks, err := s.visitRepo.ListEquipmentChecks(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list equipment checks"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": checks, "visitId": id})
}
