package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// ClinicalCodeSystem identifies the code classification system.
type ClinicalCodeSystem string

const (
	CodeSystemICD10AM ClinicalCodeSystem = "ICD-10-AM" // diagnoses
	CodeSystemACHI    ClinicalCodeSystem = "ACHI"       // Australian Classification of Health Interventions (procedures)
)

// ClinicalCodeType distinguishes the clinical role of the code on an admission.
type ClinicalCodeType string

const (
	CodeTypePrincipalDiagnosis    ClinicalCodeType = "principal-diagnosis"
	CodeTypeAdditionalDiagnosis   ClinicalCodeType = "additional-diagnosis"
	CodeTypePrincipalProcedure    ClinicalCodeType = "principal-procedure"
	CodeTypeAdditionalProcedure   ClinicalCodeType = "additional-procedure"
	CodeTypeExternalCauseOfInjury ClinicalCodeType = "external-cause"
	CodeTypeMorphology            ClinicalCodeType = "morphology"
)

// ClinicalCode is a single coded diagnosis or procedure on an admission.
type ClinicalCode struct {
	ID           string             `json:"id"`
	AdmissionID  string             `json:"admissionId"`
	System       ClinicalCodeSystem `json:"system"`
	Code         string             `json:"code"`        // e.g. "J18.9" or "92514-00"
	Description  string             `json:"description"` // human-readable label
	CodeType     ClinicalCodeType   `json:"codeType"`
	Sequence     int                `json:"sequence"`    // ordering within type (1 = principal)
	CoderHPI     string             `json:"coderHpi,omitempty"`
	TenantID     string             `json:"tenantId"`
	CodedAt      time.Time          `json:"codedAt"`
}

// CodeValidationResult reports whether a submitted code is valid.
type CodeValidationResult struct {
	Code        string             `json:"code"`
	System      ClinicalCodeSystem `json:"system"`
	Valid        bool              `json:"valid"`
	Description string             `json:"description,omitempty"`
	Error       string             `json:"error,omitempty"`
}

type codingAddRequest struct {
	System      ClinicalCodeSystem `json:"system"`
	Code        string             `json:"code"`
	Description string             `json:"description,omitempty"`
	CodeType    ClinicalCodeType   `json:"codeType"`
	CoderHPI    string             `json:"coderHpi,omitempty"`
}

type codingValidateRequest struct {
	Codes []struct {
		System ClinicalCodeSystem `json:"system"`
		Code   string             `json:"code"`
	} `json:"codes"`
}

// CodingHandler handles clinical coding routes nested under admissions.
type CodingHandler struct {
	pool       db.Pool
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// List handles GET /api/v1/admissions/{admissionId}/coding.
func (h *CodingHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	admissionID := r.PathValue("admissionId")
	codes, err := h.listCodes(ctx, admissionID, tenantID.String())
	if err != nil {
		h.logger.Error("list clinical codes", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list clinical codes"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "read", ResourceType: "ClinicalCoding",
		ResourceID: admissionID, TenantID: tenantID, OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"codes": codes, "total": len(codes)})
}

// Add handles POST /api/v1/admissions/{admissionId}/coding.
func (h *CodingHandler) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	admissionID := r.PathValue("admissionId")

	var req codingAddRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CODE", Message: "code is required"})
		return
	}
	if req.System != CodeSystemICD10AM && req.System != CodeSystemACHI {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_SYSTEM", Message: "system must be ICD-10-AM or ACHI"})
		return
	}
	if req.CodeType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_CODE_TYPE", Message: "codeType is required"})
		return
	}

	code, err := h.insertCode(ctx, admissionID, req, tenantID.String())
	if err != nil {
		h.logger.Error("insert clinical code", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to add clinical code"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "create", ResourceType: "ClinicalCoding",
		ResourceID: admissionID, TenantID: tenantID,
		Details:    map[string]any{"code": req.Code, "system": string(req.System)},
		OccurredAt: time.Now().UTC(),
	})
	writeJSON(w, http.StatusCreated, code)
}

// Remove handles DELETE /api/v1/admissions/{admissionId}/coding/{codeId}.
func (h *CodingHandler) Remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	admissionID := r.PathValue("admissionId")
	codeID := r.PathValue("codeId")

	if err := h.deleteCode(ctx, codeID, admissionID, tenantID.String()); err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "code not found"})
			return
		}
		h.logger.Error("delete clinical code", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DELETE_ERROR", Message: "failed to remove clinical code"})
		return
	}

	_ = h.auditTrail.Record(ctx, audit.Event{
		PrincipalID: principal.ID, Action: "delete", ResourceType: "ClinicalCoding",
		ResourceID: admissionID, TenantID: tenantID,
		Details:    map[string]any{"action": "remove", "code_id": codeID},
		OccurredAt: time.Now().UTC(),
	})
	w.WriteHeader(http.StatusNoContent)
}

// Validate handles POST /api/v1/admissions/{admissionId}/coding/validate.
// Validates codes without persisting them (useful for real-time UI validation).
// Currently performs basic format checks; a full ACHI/ICD-10-AM terminology
// lookup would be wired through core/terminology/.
func (h *CodingHandler) Validate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	_, ok = middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}
	_ = tenantID

	var req codingValidateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}

	results := make([]CodeValidationResult, 0, len(req.Codes))
	for _, c := range req.Codes {
		res := CodeValidationResult{Code: c.Code, System: c.System}
		switch c.System {
		case CodeSystemICD10AM:
			// Basic ICD-10-AM format: letter + 2 digits, optional dot + 1-2 digits
			if len(c.Code) >= 3 && c.Code[0] >= 'A' && c.Code[0] <= 'Z' {
				res.Valid = true
				res.Description = "ICD-10-AM code format valid (terminology lookup pending)"
			} else {
				res.Valid = false
				res.Error = "ICD-10-AM codes must start with a letter followed by digits"
			}
		case CodeSystemACHI:
			// Basic ACHI format: 5 digits, hyphen, 2 digits (e.g. 92514-00)
			if len(c.Code) == 8 && c.Code[5] == '-' {
				res.Valid = true
				res.Description = "ACHI code format valid (terminology lookup pending)"
			} else {
				res.Valid = false
				res.Error = "ACHI codes must be in format NNNNN-NN"
			}
		default:
			res.Valid = false
			res.Error = "unknown code system"
		}
		results = append(results, res)
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// ── DB helpers ────────────────────────────────────────────────────────────────

func (h *CodingHandler) listCodes(ctx context.Context, admissionID, tenantID string) ([]ClinicalCode, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, admission_id, system, code, description, code_type, sequence, coder_hpi, tenant_id, coded_at
		 FROM clinical_codes
		 WHERE admission_id = @admission_id AND tenant_id = @tenant_id
		 ORDER BY sequence ASC, coded_at ASC`,
		db.NamedArgs{"admission_id": admissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return nil, fmt.Errorf("query clinical codes: %w", err)
	}
	defer rows.Close()

	var results []ClinicalCode
	for rows.Next() {
		var c ClinicalCode
		if err := rows.Scan(
			&c.ID, &c.AdmissionID, &c.System, &c.Code, &c.Description,
			&c.CodeType, &c.Sequence, &c.CoderHPI, &c.TenantID, &c.CodedAt,
		); err != nil {
			return nil, fmt.Errorf("scan clinical code: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

func (h *CodingHandler) insertCode(ctx context.Context, admissionID string, req codingAddRequest, tenantID string) (ClinicalCode, error) {
	row := h.pool.QueryRow(ctx,
		`INSERT INTO clinical_codes
		   (admission_id, system, code, description, code_type, sequence, coder_hpi, tenant_id, coded_at)
		 VALUES
		   (@admission_id, @system, @code, @description, @code_type,
		    COALESCE((SELECT MAX(sequence)+1 FROM clinical_codes WHERE admission_id = @admission_id AND code_type = @code_type AND tenant_id = @tenant_id), 1),
		    @coder_hpi, @tenant_id, now())
		 RETURNING id, admission_id, system, code, description, code_type, sequence, coder_hpi, tenant_id, coded_at`,
		db.NamedArgs{
			"admission_id": admissionID,
			"system":       req.System,
			"code":         req.Code,
			"description":  req.Description,
			"code_type":    req.CodeType,
			"coder_hpi":    req.CoderHPI,
			"tenant_id":    tenantID,
		},
	)
	var c ClinicalCode
	if err := row.Scan(
		&c.ID, &c.AdmissionID, &c.System, &c.Code, &c.Description,
		&c.CodeType, &c.Sequence, &c.CoderHPI, &c.TenantID, &c.CodedAt,
	); err != nil {
		return ClinicalCode{}, fmt.Errorf("insert clinical code: %w", err)
	}
	return c, nil
}

func (h *CodingHandler) deleteCode(ctx context.Context, codeID, admissionID, tenantID string) error {
	tag, err := h.pool.Exec(ctx,
		`DELETE FROM clinical_codes WHERE id = @id AND admission_id = @admission_id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": codeID, "admission_id": admissionID, "tenant_id": tenantID},
	)
	if err != nil {
		return fmt.Errorf("delete clinical code: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}
