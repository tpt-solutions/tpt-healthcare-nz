package api

import (
	"net/http"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/jackc/pgx/v5"
)

func (h *paediatricHandler) GetChildProtection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, paediatric_admission_id, patient_nhi, raised_by_hpi, status,
		       concern_description, notified_at, notified_body, case_reference,
		       resolved_at, notes, tenant_id, created_at, updated_at
		FROM child_protection_flags
		WHERE paediatric_admission_id = @id AND tenant_id = @tenant_id
		ORDER BY created_at DESC
		LIMIT 100
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	flags := make([]ChildProtectionFlag, 0)
	for rows.Next() {
		var f ChildProtectionFlag
		if err := rows.Scan(
			&f.ID, &f.PaediatricAdmissionID, &f.PatientNHI, &f.RaisedByHpi, &f.Status,
			&f.ConcernDescription, &f.NotifiedAt, &f.NotifiedBody, &f.CaseReference,
			&f.ResolvedAt, &f.Notes, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(f.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		f.PatientNHI = nhi
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, flags)
}

func (h *paediatricHandler) UpdateChildProtection(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req ChildProtectionFlag
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.ID == "" {
		// Create new flag — validate required fields first.
		if req.PatientNHI == "" || req.RaisedByHpi == "" || req.ConcernDescription == "" {
			writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_FIELDS", Message: "patientNhi, raisedByHpi, and concernDescription are required"})
			return
		}
		if !h.validateHPI(w, r, req.RaisedByHpi) {
			return
		}
		nhiEnc, err := h.encryptNHI(req.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
			return
		}
		var f ChildProtectionFlag
		err = h.pool.QueryRow(r.Context(), `
			INSERT INTO child_protection_flags
			    (paediatric_admission_id, patient_nhi, raised_by_hpi, status,
			     concern_description, notified_at, notified_body, case_reference,
			     resolved_at, notes, tenant_id)
			VALUES
			    (@admission_id, @patient_nhi, @raised_by_hpi, COALESCE(@status, 'concern-raised'),
			     @concern_description, @notified_at, @notified_body, @case_reference,
			     @resolved_at, @notes, @tenant_id)
			RETURNING id, paediatric_admission_id, patient_nhi, raised_by_hpi, status,
			          concern_description, notified_at, notified_body, case_reference,
			          resolved_at, notes, tenant_id, created_at, updated_at
		`, pgx.NamedArgs{
			"admission_id":        id,
			"patient_nhi":         nhiEnc,
			"raised_by_hpi":       req.RaisedByHpi,
			"status":              req.Status,
			"concern_description": req.ConcernDescription,
			"notified_at":         req.NotifiedAt,
			"notified_body":       req.NotifiedBody,
			"case_reference":      req.CaseReference,
			"resolved_at":         req.ResolvedAt,
			"notes":               req.Notes,
			"tenant_id":           tenantID,
		}).Scan(
			&f.ID, &f.PaediatricAdmissionID, &f.PatientNHI, &f.RaisedByHpi, &f.Status,
			&f.ConcernDescription, &f.NotifiedAt, &f.NotifiedBody, &f.CaseReference,
			&f.ResolvedAt, &f.Notes, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(f.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		f.PatientNHI = nhi
		h.recordAudit(r, "create", "ChildProtectionFlag", f.ID, nhiEnc)
		writeJSON(w, http.StatusCreated, f)
		return
	}
	// Update existing flag by its ID.
	var f ChildProtectionFlag
	err := h.pool.QueryRow(r.Context(), `
		UPDATE child_protection_flags
		SET status = @status,
		    notified_at = @notified_at,
		    notified_body = @notified_body,
		    case_reference = @case_reference,
		    resolved_at = @resolved_at,
		    notes = @notes,
		    updated_at = now()
		WHERE id = @flag_id AND paediatric_admission_id = @admission_id AND tenant_id = @tenant_id
		RETURNING id, paediatric_admission_id, patient_nhi, raised_by_hpi, status,
		          concern_description, notified_at, notified_body, case_reference,
		          resolved_at, notes, tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"flag_id":        req.ID,
		"admission_id":   id,
		"status":         req.Status,
		"notified_at":    req.NotifiedAt,
		"notified_body":  req.NotifiedBody,
		"case_reference": req.CaseReference,
		"resolved_at":    req.ResolvedAt,
		"notes":          req.Notes,
		"tenant_id":      tenantID,
	}).Scan(
		&f.ID, &f.PaediatricAdmissionID, &f.PatientNHI, &f.RaisedByHpi, &f.Status,
		&f.ConcernDescription, &f.NotifiedAt, &f.NotifiedBody, &f.CaseReference,
		&f.ResolvedAt, &f.Notes, &f.TenantID, &f.CreatedAt, &f.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "child protection flag not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "ChildProtectionFlag", f.ID, f.PatientNHI)
	nhi, err := h.decryptNHI(f.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	f.PatientNHI = nhi
	writeJSON(w, http.StatusOK, f)
}
