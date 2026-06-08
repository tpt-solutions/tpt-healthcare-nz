package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/internal/districtnursing"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) createCarePlan(w http.ResponseWriter, r *http.Request) {
	var p districtnursing.CarePlan
	if err := decodeJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	p.ID = uuid.New().String()
	now := time.Now().UnixMilli()
	p.CreatedAt = now
	p.UpdatedAt = now
	if err := p.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.planRepo.Create(r.Context(), &p); err != nil {
		s.logger.Error("create care plan failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create care plan"})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) getCarePlan(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	p, err := s.planRepo.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Code: "not_found", Message: "care plan not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) listCarePlans(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset := parsePagination(r)
	plans, total, err := s.planRepo.List(r.Context(), q.Get("patient_nhi"), q.Get("clinician_id"), q.Get("plan_type"), q.Get("status"), limit, offset)
	if err != nil {
		s.logger.Error("list care plans failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list care plans"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": plans, "limit": limit, "offset": offset, "total": total,
		"filters": map[string]string{
			"patient_nhi":  q.Get("patient_nhi"),
			"clinician_id": q.Get("clinician_id"),
			"plan_type":    q.Get("plan_type"),
			"status":       q.Get("status"),
		},
	})
}

func (s *Server) updateCarePlan(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var p districtnursing.CarePlan
	if err := decodeJSON(r, &p); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	p.ID = id
	p.UpdatedAt = time.Now().UnixMilli()
	if err := p.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.planRepo.Update(r.Context(), &p); err != nil {
		s.logger.Error("update care plan failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to update care plan"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteCarePlan(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.planRepo.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to delete care plan"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createNursingVisit(w http.ResponseWriter, r *http.Request) {
	var v districtnursing.NursingVisit
	if err := decodeJSON(r, &v); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	v.ID = uuid.New().String()
	v.CarePlanID = mux.Vars(r)["id"]
	now := time.Now().UnixMilli()
	v.CreatedAt = now
	v.UpdatedAt = now
	if err := v.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.planRepo.CreateNursingVisit(r.Context(), &v); err != nil {
		s.logger.Error("create nursing visit failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create nursing visit"})
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (s *Server) listNursingVisits(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset := parsePagination(r)
	visits, total, err := s.planRepo.ListNursingVisits(r.Context(), q.Get("patient_nhi"), q.Get("care_plan_id"), limit, offset)
	if err != nil {
		s.logger.Error("list nursing visits failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list nursing visits"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": visits, "limit": limit, "offset": offset, "total": total,
		"filters": map[string]string{
			"patient_nhi":   q.Get("patient_nhi"),
			"care_plan_id": q.Get("care_plan_id"),
		},
	})
}
