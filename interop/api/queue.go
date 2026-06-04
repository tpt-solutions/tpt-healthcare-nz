package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
	corenhi "github.com/PhillipC05/tpt-healthcare/core/nhi"
	corequeue "github.com/PhillipC05/tpt-healthcare/core/queue"
	"github.com/google/uuid"
)

// handleQueueCheckIn adds a patient to today's queue using their NHI.
//
//	POST /api/v1/queue/{queueID}/check-in
//	Body: { "nhi": "ZZZ1234", "appointmentId": "..." (optional) }
func (s *Server) handleQueueCheckIn(w http.ResponseWriter, r *http.Request) {
	queueID, ok := parseUUIDParam(w, r, "queueID")
	if !ok {
		return
	}

	var body struct {
		NHI           string  `json:"nhi"`
		AppointmentID *string `json:"appointmentId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if body.NHI == "" {
		http.Error(w, "nhi is required", http.StatusBadRequest)
		return
	}

	// Determine check-in method from auth context.
	method := corequeue.CheckInPortal
	principal, hasPrincipal := auth.PrincipalFromContext(r.Context())
	if hasPrincipal {
		for _, r := range principal.Roles {
			if r == "staff" || r == "practitioner" {
				method = corequeue.CheckInStaff
				break
			}
		}
	}

	// Validate NHI format/checksum locally before touching the DB.
	nhiUpper := strings.ToUpper(strings.TrimSpace(body.NHI))
	if !corenhi.ValidateNHI(nhiUpper) {
		http.Error(w, "invalid NHI number", http.StatusBadRequest)
		return
	}

	// Look up the local patient record by NHI index.
	// nhi_index is a deterministic encrypted value; the service/repo resolves it.
	patientID, err := s.lookupPatientIDByNHI(r.Context(), nhiUpper)
	if err != nil {
		// Unknown patient is not a hard error — they can still join the queue anonymously.
		patientID = uuid.Nil
	}
	var patientIDPtr *uuid.UUID
	if patientID != uuid.Nil {
		patientIDPtr = &patientID
	}

	var apptID *uuid.UUID
	if body.AppointmentID != nil {
		id, err := uuid.Parse(*body.AppointmentID)
		if err != nil {
			http.Error(w, "invalid appointmentId", http.StatusBadRequest)
			return
		}
		apptID = &id
	}

	entry, estWait, err := s.queueService.CheckIn(r.Context(), queueID, patientIDPtr, nhiUpper, apptID, method)
	if err != nil {
		if errors.Is(err, corequeue.ErrQueueNotFound) {
			http.Error(w, "queue not found", http.StatusNotFound)
			return
		}
		http.Error(w, "check-in failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"entryId":              entry.ID,
		"position":             entry.Position,
		"estimatedWaitMinutes": estWait,
	})
}

// handleQueueCallNext marks the next waiting patient as "called".
//
//	POST /api/v1/queue/{queueID}/call-next
//	Body: { "room": "Room 2" } (optional)
func (s *Server) handleQueueCallNext(w http.ResponseWriter, r *http.Request) {
	queueID, ok := parseUUIDParam(w, r, "queueID")
	if !ok {
		return
	}

	var body struct {
		Room string `json:"room"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body) // room is optional

	entry, err := s.queueService.CallNext(r.Context(), queueID, body.Room)
	if err != nil {
		if errors.Is(err, corequeue.ErrNoWaitingEntries) {
			http.Error(w, "no waiting entries", http.StatusConflict)
			return
		}
		http.Error(w, "call-next failed", http.StatusInternalServerError)
		return
	}

	// Broadcast to patient SSE stream so their phone updates immediately.
	if s.sseHub != nil {
		s.sseHub.BroadcastPatient(entry.ID, sseEvent{
			Event: "entry-called",
			Data: map[string]any{
				"entryId": entry.ID,
				"room":    entry.RoomHint,
			},
		})
		s.sseHub.BroadcastStaff(queueID, sseEvent{
			Event: "entry-called",
			Data: map[string]any{
				"entryId": entry.ID,
				"room":    entry.RoomHint,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// handleQueueUpdateEntry updates an entry's status (done, skipped, left, in_progress).
//
//	PATCH /api/v1/queue/{queueID}/entries/{entryID}
//	Body: { "status": "done" }
func (s *Server) handleQueueUpdateEntry(w http.ResponseWriter, r *http.Request) {
	queueID, ok := parseUUIDParam(w, r, "queueID")
	if !ok {
		return
	}
	entryID, ok := parseUUIDParam(w, r, "entryID")
	if !ok {
		return
	}
	_ = queueID // used for SSE broadcast below

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	status := corequeue.EntryStatus(body.Status)
	entry, err := s.queueService.UpdateStatus(r.Context(), entryID, status)
	if err != nil {
		if errors.Is(err, corequeue.ErrEntryNotFound) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}

	if s.sseHub != nil {
		s.sseHub.BroadcastStaff(queueID, sseEvent{
			Event: "entry-updated",
			Data:  map[string]any{"entryId": entryID, "status": string(status)},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

// handleQueueGetEntries returns the current queue with all entries.
//
//	GET /api/v1/queue/{queueID}
func (s *Server) handleQueueGetEntries(w http.ResponseWriter, r *http.Request) {
	queueID, ok := parseUUIDParam(w, r, "queueID")
	if !ok {
		return
	}

	queue, err := s.queueRepo.GetQueue(r.Context(), queueID)
	if err != nil {
		if errors.Is(err, corequeue.ErrQueueNotFound) {
			http.Error(w, "queue not found", http.StatusNotFound)
			return
		}
		http.Error(w, "get queue failed", http.StatusInternalServerError)
		return
	}

	entries, err := s.queueRepo.ListEntries(r.Context(), queueID)
	if err != nil {
		http.Error(w, "list entries failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"queue":   queue,
		"entries": entries,
	})
}

// handleQueueUpdateLocation saves an ephemeral GPS fix for a waiting patient.
//
//	POST /api/v1/queue/{queueID}/entries/{entryID}/location
//	Body: { "lat": -36.8, "lng": 174.7, "accuracyMeters": 10 }
func (s *Server) handleQueueUpdateLocation(w http.ResponseWriter, r *http.Request) {
	queueID, ok := parseUUIDParam(w, r, "queueID")
	if !ok {
		return
	}
	entryID, ok := parseUUIDParam(w, r, "entryID")
	if !ok {
		return
	}

	var body struct {
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
		AccuracyM float32 `json:"accuracyMeters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if err := s.queueService.UpdateLocation(r.Context(), queueID, entryID, body.Lat, body.Lng, body.AccuracyM); err != nil {
		if errors.Is(err, corequeue.ErrEntryNotActive) {
			http.Error(w, "entry is no longer active", http.StatusConflict)
			return
		}
		if errors.Is(err, corequeue.ErrEntryNotFound) {
			http.Error(w, "entry not found", http.StatusNotFound)
			return
		}
		http.Error(w, "location update failed", http.StatusInternalServerError)
		return
	}

	// Broadcast to staff SSE stream only — patient stream never receives location data.
	if s.sseHub != nil {
		s.sseHub.BroadcastStaff(queueID, sseEvent{
			Event: "entry-location-updated",
			Data: map[string]any{
				"entryId": entryID,
				"lat":     body.Lat,
				"lng":     body.Lng,
			},
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCreateQueue creates today's queue for the current tenant.
//
//	POST /api/v1/queue
//	Body: { "name": "General Practice" }
func (s *Server) handleCreateQueue(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	queue, err := s.queueRepo.GetOrCreateTodayQueue(r.Context(), principal.TenantID, body.Name)
	if err != nil {
		http.Error(w, "create queue failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(queue)
}

// parseUUIDParam extracts a named path value as uuid.UUID.
func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		http.Error(w, "invalid "+name, http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

// lookupPatientIDByNHI looks up a patient UUID by their plaintext NHI in the local DB.
// The nhi_index column stores the deterministic cipher of the NHI. Since the server
// applies the cipher at write time, this query uses the raw NHI for now and relies on
// the encryption layer being transparent at this level.
func (s *Server) lookupPatientIDByNHI(ctx context.Context, nhi string) (uuid.UUID, error) {
	if s.pool == nil {
		return uuid.Nil, errors.New("no db pool")
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id FROM patients WHERE nhi_index = $1 LIMIT 1`, nhi,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
