package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-community-health/internal/outreach"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

func (s *Server) createProgram(w http.ResponseWriter, r *http.Request) {
	var p outreach.Program
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
	if err := s.outreachRepo.CreateProgram(r.Context(), &p); err != nil {
		s.logger.Error("create program failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create program"})
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) getProgram(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	p, err := s.outreachRepo.GetProgram(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Code: "not_found", Message: "program not found"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) listPrograms(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset := parsePagination(r)
	programs, total, err := s.outreachRepo.ListPrograms(r.Context(), q.Get("practice_id"), q.Get("program_type"), q.Get("status"), limit, offset)
	if err != nil {
		s.logger.Error("list programs failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list programs"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": programs, "limit": limit, "offset": offset, "total": total,
		"filters": map[string]string{
			"practice_id":  q.Get("practice_id"),
			"program_type": q.Get("program_type"),
			"status":       q.Get("status"),
		},
	})
}

func (s *Server) updateProgram(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var p outreach.Program
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
	if err := s.outreachRepo.UpdateProgram(r.Context(), &p); err != nil {
		s.logger.Error("update program failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to update program"})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProgram(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := s.outreachRepo.DeleteProgram(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to delete program"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	var e outreach.Event
	if err := decodeJSON(r, &e); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	e.ID = uuid.New().String()
	e.ProgramID = mux.Vars(r)["id"]
	now := time.Now().UnixMilli()
	e.CreatedAt = now
	e.UpdatedAt = now
	if err := e.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.outreachRepo.CreateEvent(r.Context(), &e); err != nil {
		s.logger.Error("create event failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create event"})
		return
	}
	writeJSON(w, http.StatusCreated, e)
}

func (s *Server) getEvent(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["eventId"]
	e, err := s.outreachRepo.GetEvent(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiError{Code: "not_found", Message: "event not found"})
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	programID := mux.Vars(r)["id"]
	limit, offset := parsePagination(r)
	events, total, err := s.outreachRepo.ListEvents(r.Context(), programID, limit, offset)
	if err != nil {
		s.logger.Error("list events failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list events"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": events, "limit": limit, "offset": offset, "total": total, "programId": programID})
}

func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["eventId"]
	var e outreach.Event
	if err := decodeJSON(r, &e); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	e.ID = id
	e.UpdatedAt = time.Now().UnixMilli()
	if err := e.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.outreachRepo.UpdateEvent(r.Context(), &e); err != nil {
		s.logger.Error("update event failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to update event"})
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) createAttendee(w http.ResponseWriter, r *http.Request) {
	var a outreach.Attendee
	if err := decodeJSON(r, &a); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	a.ID = uuid.New().String()
	a.EventID = mux.Vars(r)["eventId"]
	now := time.Now().UnixMilli()
	a.CreatedAt = now
	a.UpdatedAt = now
	if err := a.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.outreachRepo.CreateAttendee(r.Context(), &a); err != nil {
		s.logger.Error("create attendee failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create attendee"})
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) listAttendees(w http.ResponseWriter, r *http.Request) {
	eventID := mux.Vars(r)["eventId"]
	limit, offset := parsePagination(r)
	attendees, total, err := s.outreachRepo.ListAttendees(r.Context(), eventID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list attendees"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": attendees, "limit": limit, "offset": offset, "total": total, "eventId": eventID})
}

func (s *Server) createReferral(w http.ResponseWriter, r *http.Request) {
	var ref outreach.Referral
	if err := decodeJSON(r, &ref); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	ref.ID = uuid.New().String()
	ref.EventID = mux.Vars(r)["eventId"]
	now := time.Now().UnixMilli()
	ref.CreatedAt = now
	ref.UpdatedAt = now
	if err := ref.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.outreachRepo.CreateReferral(r.Context(), &ref); err != nil {
		s.logger.Error("create referral failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create referral"})
		return
	}
	writeJSON(w, http.StatusCreated, ref)
}

func (s *Server) listReferrals(w http.ResponseWriter, r *http.Request) {
	eventID := mux.Vars(r)["eventId"]
	refs, err := s.outreachRepo.ListReferrals(r.Context(), eventID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list referrals"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": refs, "eventId": eventID})
}

func (s *Server) createScreening(w http.ResponseWriter, r *http.Request) {
	var sc outreach.Screening
	if err := decodeJSON(r, &sc); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "bad_request", Message: err.Error()})
		return
	}
	sc.ID = uuid.New().String()
	sc.EventID = mux.Vars(r)["eventId"]
	now := time.Now().UnixMilli()
	sc.CreatedAt = now
	sc.UpdatedAt = now
	if err := sc.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "validation_error", Message: err.Error()})
		return
	}
	if err := s.outreachRepo.CreateScreening(r.Context(), &sc); err != nil {
		s.logger.Error("create screening failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to create screening"})
		return
	}
	writeJSON(w, http.StatusCreated, sc)
}

func (s *Server) listScreenings(w http.ResponseWriter, r *http.Request) {
	eventID := mux.Vars(r)["eventId"]
	screenings, err := s.outreachRepo.ListScreenings(r.Context(), eventID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "internal_error", Message: "failed to list screenings"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": screenings, "eventId": eventID})
}
