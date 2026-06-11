package api

import (
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

// OutbreakInvestigation tracks a cluster of linked notifiable disease cases
// sharing a common suspected source. Managed by public health units (PHUs).
// status values: suspected | confirmed | controlled | closed
type OutbreakInvestigation struct {
	ID                  string     `json:"id"`
	OutbreakName        string     `json:"outbreakName"`
	DiseaseCode         string     `json:"diseaseCode"`
	DiseaseName         string     `json:"diseaseName"`
	Status              string     `json:"status"`
	StartDate           *string    `json:"startDate"`  // DATE as YYYY-MM-DD
	EndDate             *string    `json:"endDate"`    // DATE as YYYY-MM-DD
	LocationDescription *string    `json:"locationDescription"`
	SuspectedSource     *string    `json:"suspectedSource"`
	CaseCount           int        `json:"caseCount"`
	Notes               *string    `json:"notes"`
	TenantID            string     `json:"tenantId"`
	ConfirmedAt         *time.Time `json:"confirmedAt"`
	ControlledAt        *time.Time `json:"controlledAt"`
	ClosedAt            *time.Time `json:"closedAt"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

const outbreakSelectCols = `id, outbreak_name, disease_code, disease_name,
       status, start_date::text, end_date::text,
       location_description, suspected_source, case_count, notes,
       tenant_id, confirmed_at, controlled_at, closed_at,
       created_at, updated_at`

func scanOutbreak(row interface{ Scan(...any) error }, o *OutbreakInvestigation) error {
	return row.Scan(
		&o.ID, &o.OutbreakName, &o.DiseaseCode, &o.DiseaseName,
		&o.Status, &o.StartDate, &o.EndDate,
		&o.LocationDescription, &o.SuspectedSource, &o.CaseCount, &o.Notes,
		&o.TenantID, &o.ConfirmedAt, &o.ControlledAt, &o.ClosedAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
}

type outbreakHandler struct{ handlerDeps }

func (h *outbreakHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	q := r.URL.Query()
	diseaseCode := q.Get("disease_code")
	status := q.Get("status")

	var rows pgx.Rows
	var err error
	switch {
	case diseaseCode != "" && status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+outbreakSelectCols+` FROM outbreak_investigations
			 WHERE tenant_id = @tenant_id AND disease_code = @disease_code AND status = @status
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "disease_code": diseaseCode, "status": status})
	case diseaseCode != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+outbreakSelectCols+` FROM outbreak_investigations
			 WHERE tenant_id = @tenant_id AND disease_code = @disease_code
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "disease_code": diseaseCode})
	case status != "":
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+outbreakSelectCols+` FROM outbreak_investigations
			 WHERE tenant_id = @tenant_id AND status = @status
			 ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID, "status": status})
	default:
		rows, err = h.pool.Query(r.Context(),
			`SELECT `+outbreakSelectCols+` FROM outbreak_investigations
			 WHERE tenant_id = @tenant_id ORDER BY created_at DESC`,
			pgx.NamedArgs{"tenant_id": tenantID})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	outbreaks := make([]OutbreakInvestigation, 0)
	for rows.Next() {
		var o OutbreakInvestigation
		if err := scanOutbreak(rows, &o); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		outbreaks = append(outbreaks, o)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, outbreaks)
}

func (h *outbreakHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req OutbreakInvestigation
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.OutbreakName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "outbreakName is required"})
		return
	}
	if req.DiseaseCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "diseaseCode is required"})
		return
	}
	var o OutbreakInvestigation
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO outbreak_investigations
		    (outbreak_name, disease_code, disease_name, status,
		     start_date, location_description, suspected_source, notes, tenant_id)
		VALUES
		    (@outbreak_name, @disease_code, @disease_name, 'suspected',
		     @start_date, @location_description, @suspected_source, @notes, @tenant_id)
		RETURNING `+outbreakSelectCols,
		pgx.NamedArgs{
			"outbreak_name":        req.OutbreakName,
			"disease_code":         req.DiseaseCode,
			"disease_name":         req.DiseaseName,
			"start_date":           req.StartDate,
			"location_description": req.LocationDescription,
			"suspected_source":     req.SuspectedSource,
			"notes":                req.Notes,
			"tenant_id":            tenantID,
		}).Scan(
		&o.ID, &o.OutbreakName, &o.DiseaseCode, &o.DiseaseName,
		&o.Status, &o.StartDate, &o.EndDate,
		&o.LocationDescription, &o.SuspectedSource, &o.CaseCount, &o.Notes,
		&o.TenantID, &o.ConfirmedAt, &o.ControlledAt, &o.ClosedAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "OutbreakInvestigation", o.ID, "")
	writeJSON(w, http.StatusCreated, o)
}

func (h *outbreakHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var o OutbreakInvestigation
	err := h.pool.QueryRow(r.Context(),
		`SELECT `+outbreakSelectCols+` FROM outbreak_investigations WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&o.ID, &o.OutbreakName, &o.DiseaseCode, &o.DiseaseName,
		&o.Status, &o.StartDate, &o.EndDate,
		&o.LocationDescription, &o.SuspectedSource, &o.CaseCount, &o.Notes,
		&o.TenantID, &o.ConfirmedAt, &o.ControlledAt, &o.ClosedAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outbreak investigation not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "read", "OutbreakInvestigation", o.ID, "")
	writeJSON(w, http.StatusOK, o)
}

func (h *outbreakHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req OutbreakInvestigation
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var o OutbreakInvestigation
	err := h.pool.QueryRow(r.Context(), `
		UPDATE outbreak_investigations
		SET outbreak_name        = @outbreak_name,
		    disease_name         = @disease_name,
		    start_date           = @start_date,
		    end_date             = @end_date,
		    location_description = @location_description,
		    suspected_source     = @suspected_source,
		    notes                = @notes,
		    updated_at           = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status NOT IN ('closed')
		RETURNING `+outbreakSelectCols,
		pgx.NamedArgs{
			"outbreak_name":        req.OutbreakName,
			"disease_name":         req.DiseaseName,
			"start_date":           req.StartDate,
			"end_date":             req.EndDate,
			"location_description": req.LocationDescription,
			"suspected_source":     req.SuspectedSource,
			"notes":                req.Notes,
			"id":                   id,
			"tenant_id":            tenantID,
		}).Scan(
		&o.ID, &o.OutbreakName, &o.DiseaseCode, &o.DiseaseName,
		&o.Status, &o.StartDate, &o.EndDate,
		&o.LocationDescription, &o.SuspectedSource, &o.CaseCount, &o.Notes,
		&o.TenantID, &o.ConfirmedAt, &o.ControlledAt, &o.ClosedAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outbreak investigation not found or already closed"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "OutbreakInvestigation", o.ID, "")
	writeJSON(w, http.StatusOK, o)
}

// Confirm transitions a suspected outbreak to confirmed after epidemiological evidence.
func (h *outbreakHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	h.transitionOutbreak(w, r, "confirmed", "confirmed_at = now(),", []string{"suspected"})
}

// Control marks the outbreak as under control (transmission interrupted).
func (h *outbreakHandler) Control(w http.ResponseWriter, r *http.Request) {
	h.transitionOutbreak(w, r, "controlled", "controlled_at = now(),", []string{"confirmed"})
}

// Close marks the outbreak investigation as closed.
func (h *outbreakHandler) Close(w http.ResponseWriter, r *http.Request) {
	h.transitionOutbreak(w, r, "closed", "closed_at = now(),", []string{"suspected", "confirmed", "controlled"})
}

func (h *outbreakHandler) transitionOutbreak(w http.ResponseWriter, r *http.Request, toStatus, extraSet string, fromStatuses []string) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")

	// Verify the outbreak exists for this tenant.
	var exists bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM outbreak_investigations WHERE id = @id AND tenant_id = @tenant_id)`,
		pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(&exists); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outbreak investigation not found"})
		return
	}

	inClause := "'"
	for i, s := range fromStatuses {
		if i > 0 {
			inClause += "', '"
		}
		inClause += s
	}
	inClause += "'"

	tag, err := h.pool.Exec(r.Context(),
		`UPDATE outbreak_investigations
		 SET status     = @status,
		     `+extraSet+`
		     updated_at = now()
		 WHERE id = @id AND tenant_id = @tenant_id AND status IN (`+inClause+`)`,
		pgx.NamedArgs{"status": toStatus, "id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "outbreak is not in a valid state for this transition"})
		return
	}
	h.recordAudit(r, "update", "OutbreakInvestigation", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": toStatus})
}

// LinkCase associates an existing surveillance case with this outbreak and increments case_count.
func (h *outbreakHandler) LinkCase(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	outbreakID := r.PathValue("id")
	var body struct {
		CaseID string `json:"caseId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if body.CaseID == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELD", Message: "caseId is required"})
		return
	}

	// Ensure the outbreak exists and is not closed.
	var outbreakStatus string
	if err := h.pool.QueryRow(r.Context(),
		`SELECT status FROM outbreak_investigations WHERE id = @id AND tenant_id = @tenant_id`,
		pgx.NamedArgs{"id": outbreakID, "tenant_id": tenantID}).Scan(&outbreakStatus); err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "outbreak investigation not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if outbreakStatus == "closed" {
		writeJSON(w, http.StatusConflict, apiError{Code: "INVALID_STATUS", Message: "cannot link cases to a closed outbreak"})
		return
	}

	// Link the case and increment the outbreak case count atomically.
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	tag, err := tx.Exec(r.Context(), `
		UPDATE surveillance_cases
		SET outbreak_id = @outbreak_id, updated_at = now()
		WHERE id = @case_id AND tenant_id = @tenant_id AND outbreak_id IS NULL
	`, pgx.NamedArgs{"outbreak_id": outbreakID, "case_id": body.CaseID, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusConflict, apiError{Code: "ALREADY_LINKED", Message: "case not found or already linked to an outbreak"})
		return
	}

	if _, err := tx.Exec(r.Context(), `
		UPDATE outbreak_investigations
		SET case_count = case_count + 1, updated_at = now()
		WHERE id = @id
	`, pgx.NamedArgs{"id": outbreakID}); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "OutbreakInvestigation", outbreakID, "")
	writeJSON(w, http.StatusOK, map[string]string{"outbreakId": outbreakID, "caseId": body.CaseID})
}
