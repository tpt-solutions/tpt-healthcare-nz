package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Roster — staff shifts
// ============================================================

type shift struct {
	ID           uuid.UUID  `json:"id"`
	TenantID     uuid.UUID  `json:"tenant_id"`
	PrincipalID  string     `json:"principal_id"`
	DepartmentID *uuid.UUID `json:"department_id,omitempty"`
	ShiftStart   time.Time  `json:"shift_start"`
	ShiftEnd     time.Time  `json:"shift_end"`
	ShiftType    string     `json:"shift_type"`
	Notes        string     `json:"notes,omitempty"`
	PayrollPushAt *time.Time `json:"payroll_push_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *Server) listShifts(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// Optional ?from=YYYY-MM-DD&to=YYYY-MM-DD filter.
	from := time.Now().UTC().AddDate(0, -1, 0)
	to := time.Now().UTC().AddDate(0, 1, 0)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	rows, err := s.cfg.Pool.Query(r.Context(), `
		SELECT id, tenant_id, principal_id, department_id,
		       shift_start, shift_end, shift_type, notes, payroll_push_at, created_at
		FROM staff_shifts
		WHERE tenant_id = @tid AND shift_start >= @from AND shift_start < @to
		ORDER BY shift_start`,
		map[string]any{"tid": tid, "from": from, "to": to})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []shift
	for rows.Next() {
		var sh shift
		if err := rows.Scan(&sh.ID, &sh.TenantID, &sh.PrincipalID, &sh.DepartmentID,
			&sh.ShiftStart, &sh.ShiftEnd, &sh.ShiftType, &sh.Notes,
			&sh.PayrollPushAt, &sh.CreatedAt); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		result = append(result, sh)
	}
	if rows.Err() != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createShift(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		PrincipalID  string     `json:"principal_id"`
		DepartmentID *uuid.UUID `json:"department_id"`
		ShiftStart   time.Time  `json:"shift_start"`
		ShiftEnd     time.Time  `json:"shift_end"`
		ShiftType    string     `json:"shift_type"`
		Notes        string     `json:"notes"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.ShiftType == "" {
		body.ShiftType = "ordinary"
	}
	if body.PrincipalID == "" || body.ShiftStart.IsZero() || body.ShiftEnd.IsZero() {
		http.Error(w, "principal_id, shift_start, and shift_end are required", http.StatusBadRequest)
		return
	}
	if !body.ShiftEnd.After(body.ShiftStart) {
		http.Error(w, "shift_end must be after shift_start", http.StatusBadRequest)
		return
	}
	var sh shift
	err := s.cfg.Pool.QueryRow(r.Context(), `
		INSERT INTO staff_shifts (id, tenant_id, principal_id, department_id,
		                          shift_start, shift_end, shift_type, notes)
		VALUES (gen_random_uuid(), @tid, @pid, @dept, @start, @end, @type, @notes)
		RETURNING id, tenant_id, principal_id, department_id,
		          shift_start, shift_end, shift_type, notes, payroll_push_at, created_at`,
		map[string]any{
			"tid":   tid,
			"pid":   body.PrincipalID,
			"dept":  body.DepartmentID,
			"start": body.ShiftStart,
			"end":   body.ShiftEnd,
			"type":  body.ShiftType,
			"notes": body.Notes,
		},
	).Scan(&sh.ID, &sh.TenantID, &sh.PrincipalID, &sh.DepartmentID,
		&sh.ShiftStart, &sh.ShiftEnd, &sh.ShiftType, &sh.Notes,
		&sh.PayrollPushAt, &sh.CreatedAt)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, sh)
}

func (s *Server) deleteShift(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ct, err := s.cfg.Pool.Exec(r.Context(),
		`DELETE FROM staff_shifts WHERE id = @id`, map[string]any{"id": id})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// Rooms & bookings
// ============================================================

type room struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	Name      string    `json:"name"`
	Location  string    `json:"location,omitempty"`
	Capacity  int       `json:"capacity"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type roomBooking struct {
	ID             uuid.UUID  `json:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"`
	RoomID         uuid.UUID  `json:"room_id"`
	BookedBy       string     `json:"booked_by"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        time.Time  `json:"end_time"`
	AppointmentRef string     `json:"appointment_ref,omitempty"`
	EncounterRef   string     `json:"encounter_ref,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (s *Server) listRooms(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := s.cfg.Pool.Query(r.Context(), `
		SELECT id, tenant_id, name, location, capacity, active, created_at
		FROM rooms WHERE tenant_id = @tid AND active = true ORDER BY name`,
		map[string]any{"tid": tid})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []room
	for rows.Next() {
		var rm room
		if err := rows.Scan(&rm.ID, &rm.TenantID, &rm.Name, &rm.Location,
			&rm.Capacity, &rm.Active, &rm.CreatedAt); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		result = append(result, rm)
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createRoom(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		Name     string `json:"name"`
		Location string `json:"location"`
		Capacity int    `json:"capacity"`
	}
	if err := readJSON(r, &body); err != nil || body.Name == "" {
		http.Error(w, "bad request: name required", http.StatusBadRequest)
		return
	}
	if body.Capacity == 0 {
		body.Capacity = 1
	}
	var rm room
	err := s.cfg.Pool.QueryRow(r.Context(), `
		INSERT INTO rooms (id, tenant_id, name, location, capacity)
		VALUES (gen_random_uuid(), @tid, @name, @location, @capacity)
		RETURNING id, tenant_id, name, location, capacity, active, created_at`,
		map[string]any{"tid": tid, "name": body.Name, "location": body.Location, "capacity": body.Capacity},
	).Scan(&rm.ID, &rm.TenantID, &rm.Name, &rm.Location, &rm.Capacity, &rm.Active, &rm.CreatedAt)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, rm)
}

func (s *Server) listBookings(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	from := time.Now().UTC().AddDate(0, 0, -1)
	to := time.Now().UTC().AddDate(0, 0, 30)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	rows, err := s.cfg.Pool.Query(r.Context(), `
		SELECT id, tenant_id, room_id, booked_by, start_time, end_time,
		       appointment_ref, encounter_ref, created_at
		FROM room_bookings
		WHERE tenant_id = @tid AND start_time >= @from AND start_time < @to
		ORDER BY start_time`,
		map[string]any{"tid": tid, "from": from, "to": to})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []roomBooking
	for rows.Next() {
		var b roomBooking
		if err := rows.Scan(&b.ID, &b.TenantID, &b.RoomID, &b.BookedBy,
			&b.StartTime, &b.EndTime, &b.AppointmentRef, &b.EncounterRef, &b.CreatedAt); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		result = append(result, b)
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	pid, pok := principalID(r)
	if !ok || !pok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		RoomID         uuid.UUID `json:"room_id"`
		StartTime      time.Time `json:"start_time"`
		EndTime        time.Time `json:"end_time"`
		AppointmentRef string    `json:"appointment_ref"`
		EncounterRef   string    `json:"encounter_ref"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.RoomID == uuid.Nil || body.StartTime.IsZero() || body.EndTime.IsZero() {
		http.Error(w, "room_id, start_time, and end_time are required", http.StatusBadRequest)
		return
	}
	if !body.EndTime.After(body.StartTime) {
		http.Error(w, "end_time must be after start_time", http.StatusBadRequest)
		return
	}
	var b roomBooking
	err := s.cfg.Pool.QueryRow(r.Context(), `
		INSERT INTO room_bookings (id, tenant_id, room_id, booked_by, start_time, end_time,
		                           appointment_ref, encounter_ref)
		VALUES (gen_random_uuid(), @tid, @room, @by, @start, @end, @appt, @enc)
		RETURNING id, tenant_id, room_id, booked_by, start_time, end_time,
		          appointment_ref, encounter_ref, created_at`,
		map[string]any{
			"tid":   tid,
			"room":  body.RoomID,
			"by":    pid,
			"start": body.StartTime,
			"end":   body.EndTime,
			"appt":  body.AppointmentRef,
			"enc":   body.EncounterRef,
		},
	).Scan(&b.ID, &b.TenantID, &b.RoomID, &b.BookedBy,
		&b.StartTime, &b.EndTime, &b.AppointmentRef, &b.EncounterRef, &b.CreatedAt)
	if err != nil {
		// Postgres exclusion constraint violation (double-booking)
		errMsg := err.Error()
		if len(errMsg) > 0 && (contains(errMsg, "exclusion") || contains(errMsg, "conflict") || contains(errMsg, "overlap")) {
			http.Error(w, fmt.Sprintf("room already booked for that time slot"), http.StatusConflict)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (s *Server) deleteBooking(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ct, err := s.cfg.Pool.Exec(r.Context(),
		`DELETE FROM room_bookings WHERE id = @id`, map[string]any{"id": id})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if ct.RowsAffected() == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// Leave requests
// ============================================================

type leaveRequest struct {
	ID                  uuid.UUID  `json:"id"`
	TenantID            uuid.UUID  `json:"tenant_id"`
	PrincipalID         string     `json:"principal_id"`
	LeaveType           string     `json:"leave_type"`
	StartDate           string     `json:"start_date"` // YYYY-MM-DD
	EndDate             string     `json:"end_date"`
	Status              string     `json:"status"`
	Notes               string     `json:"notes,omitempty"`
	ApprovedBy          string     `json:"approved_by,omitempty"`
	ApprovedAt          *time.Time `json:"approved_at,omitempty"`
	PayrollLeaveID      string     `json:"payroll_leave_id,omitempty"`
	PayrollSubmittedAt  *time.Time `json:"payroll_submitted_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

func (s *Server) listLeaveRequests(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	rows, err := s.cfg.Pool.Query(r.Context(), `
		SELECT id, tenant_id, principal_id, leave_type,
		       start_date::text, end_date::text, status, notes,
		       COALESCE(approved_by, ''), approved_at,
		       COALESCE(payroll_leave_id, ''), payroll_submitted_at,
		       created_at
		FROM leave_requests
		WHERE tenant_id = @tid
		ORDER BY created_at DESC`,
		map[string]any{"tid": tid})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var result []leaveRequest
	for rows.Next() {
		var lr leaveRequest
		if err := rows.Scan(
			&lr.ID, &lr.TenantID, &lr.PrincipalID, &lr.LeaveType,
			&lr.StartDate, &lr.EndDate, &lr.Status, &lr.Notes,
			&lr.ApprovedBy, &lr.ApprovedAt,
			&lr.PayrollLeaveID, &lr.PayrollSubmittedAt,
			&lr.CreatedAt,
		); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		result = append(result, lr)
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) createLeaveRequest(w http.ResponseWriter, r *http.Request) {
	tid, ok := tenantID(r)
	pid, pok := principalID(r)
	if !ok || !pok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var body struct {
		LeaveType string `json:"leave_type"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
		Notes     string `json:"notes"`
	}
	if err := readJSON(r, &body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.LeaveType == "" || body.StartDate == "" || body.EndDate == "" {
		http.Error(w, "leave_type, start_date, and end_date are required", http.StatusBadRequest)
		return
	}
	var lr leaveRequest
	err := s.cfg.Pool.QueryRow(r.Context(), `
		INSERT INTO leave_requests (id, tenant_id, principal_id, leave_type,
		                            start_date, end_date, notes)
		VALUES (gen_random_uuid(), @tid, @pid, @type, @start, @end, @notes)
		RETURNING id, tenant_id, principal_id, leave_type,
		          start_date::text, end_date::text, status, notes,
		          COALESCE(approved_by, ''), approved_at,
		          COALESCE(payroll_leave_id, ''), payroll_submitted_at,
		          created_at`,
		map[string]any{
			"tid":   tid,
			"pid":   pid,
			"type":  body.LeaveType,
			"start": body.StartDate,
			"end":   body.EndDate,
			"notes": body.Notes,
		},
	).Scan(
		&lr.ID, &lr.TenantID, &lr.PrincipalID, &lr.LeaveType,
		&lr.StartDate, &lr.EndDate, &lr.Status, &lr.Notes,
		&lr.ApprovedBy, &lr.ApprovedAt,
		&lr.PayrollLeaveID, &lr.PayrollSubmittedAt,
		&lr.CreatedAt,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, lr)
}

func (s *Server) approveLeave(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	pid, ok := principalID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var lr leaveRequest
	err = s.cfg.Pool.QueryRow(r.Context(), `
		UPDATE leave_requests
		SET status = 'approved', approved_by = @by, approved_at = NOW()
		WHERE id = @id AND status = 'pending'
		RETURNING id, tenant_id, principal_id, leave_type,
		          start_date::text, end_date::text, status, notes,
		          COALESCE(approved_by, ''), approved_at,
		          COALESCE(payroll_leave_id, ''), payroll_submitted_at,
		          created_at`,
		map[string]any{"id": id, "by": pid},
	).Scan(
		&lr.ID, &lr.TenantID, &lr.PrincipalID, &lr.LeaveType,
		&lr.StartDate, &lr.EndDate, &lr.Status, &lr.Notes,
		&lr.ApprovedBy, &lr.ApprovedAt,
		&lr.PayrollLeaveID, &lr.PayrollSubmittedAt,
		&lr.CreatedAt,
	)
	if err != nil {
		http.Error(w, "not found or already actioned", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, lr)
}

func (s *Server) declineLeave(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	pid, ok := principalID(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var lr leaveRequest
	err = s.cfg.Pool.QueryRow(r.Context(), `
		UPDATE leave_requests
		SET status = 'declined', approved_by = @by, approved_at = NOW()
		WHERE id = @id AND status = 'pending'
		RETURNING id, tenant_id, principal_id, leave_type,
		          start_date::text, end_date::text, status, notes,
		          COALESCE(approved_by, ''), approved_at,
		          COALESCE(payroll_leave_id, ''), payroll_submitted_at,
		          created_at`,
		map[string]any{"id": id, "by": pid},
	).Scan(
		&lr.ID, &lr.TenantID, &lr.PrincipalID, &lr.LeaveType,
		&lr.StartDate, &lr.EndDate, &lr.Status, &lr.Notes,
		&lr.ApprovedBy, &lr.ApprovedAt,
		&lr.PayrollLeaveID, &lr.PayrollSubmittedAt,
		&lr.CreatedAt,
	)
	if err != nil {
		http.Error(w, "not found or already actioned", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, lr)
}

// contains is a simple string containment helper used for error classification.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
