package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Domain types ---

// Immunisation represents a FHIR R5 Immunization resource adapted for NZ use.
// Vaccine codes use NZMT (New Zealand Medicines Terminology) identifiers from
// the NZMT FHIR terminology server (https://nzmt.org.nz/).
type Immunisation struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Status       string `json:"status"` // completed | entered-in-error | not-done

	// VaccineCode is an NZMT-coded vaccine reference.
	VaccineCode struct {
		Coding []struct {
			System  string `json:"system"` // https://www.nzmt.org.nz/
			Code    string `json:"code"`   // NZMT CT code
			Display string `json:"display"`
		} `json:"coding"`
		Text string `json:"text"`
	} `json:"vaccineCode"`

	PatientNHI         string    `json:"patientNhi"` // NHI extracted from patient.identifier
	OccurrenceDateTime time.Time `json:"occurrenceDateTime"`

	// Site is where on the body the vaccine was given (SNOMED CT).
	Site struct {
		Coding []struct {
			System  string `json:"system"`
			Code    string `json:"code"`
			Display string `json:"display"`
		} `json:"coding"`
	} `json:"site,omitempty"`

	// Route is how the vaccine was administered (SNOMED CT, e.g. intramuscular).
	Route struct {
		Coding []struct {
			System  string `json:"system"`
			Code    string `json:"code"`
			Display string `json:"display"`
		} `json:"coding"`
	} `json:"route,omitempty"`

	LotNumber  string `json:"lotNumber,omitempty"`
	ExpiryDate string `json:"expirationDate,omitempty"` // YYYY-MM-DD

	// PractitionerHPICPN is the administering practitioner's HPI Common Person Number.
	PractitionerHPICPN string `json:"practitionerHpiCpn"`

	// NIRSubmitted tracks whether this record has been sent to the NIR.
	NIRSubmitted   bool       `json:"nirSubmitted"`
	NIRSubmittedAt *time.Time `json:"nirSubmittedAt,omitempty"`
	NIRReferenceID string     `json:"nirReferenceId,omitempty"`

	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NIRSubmitResult is the response from a successful NIR submission.
type NIRSubmitResult struct {
	ImmunisationID string    `json:"immunisationId"`
	NIRReferenceID string    `json:"nirReferenceId"`
	SubmittedAt    time.Time `json:"submittedAt"`
}

// ImmunisationHandler handles all /api/v1/immunisations and /api/v1/schedule routes.
type ImmunisationHandler struct {
	logger *slog.Logger
	pool   *pgxpool.Pool
	nir    *NIRClient
}

// List handles GET /api/v1/immunisations — list immunisation records for a patient.
//
// Query parameters:
//   - nhi (required): Patient NHI number. Both old (ABC1234) and new (ABC12AB) formats accepted.
//   - _count: page size (default 20, max 100)
//   - _offset: pagination offset
func (h *ImmunisationHandler) List(w http.ResponseWriter, r *http.Request) {
	nhi := r.URL.Query().Get("nhi")
	if nhi == "" {
		writeError(w, http.StatusBadRequest, "nhi query parameter is required")
		return
	}

	h.logger.Info("list immunisations", "nhi", nhi, "request_id", r.Context().Value(requestIDKey))

	records, err := listImmunisations(r.Context(), h.pool, nhi)
	if err != nil {
		h.logger.Error("list immunisations failed", "error", err, "request_id", r.Context().Value(requestIDKey))
		writeError(w, http.StatusInternalServerError, "failed to list immunisations")
		return
	}

	entries := make([]map[string]any, 0, len(records))
	for _, im := range records {
		entries = append(entries, map[string]any{"resource": im})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        len(records),
		"entry":        entries,
	})
}

// Record handles POST /api/v1/immunisations — record a new immunisation event.
//
// The body must be a FHIR Immunization resource (mapped to the Immunisation struct above).
// Required fields: vaccineCode (NZMT), patientNhi, occurrenceDateTime, practitionerHpiCpn.
// Optional but strongly recommended: site, route, lotNumber, expirationDate.
//
// After successful persistence:
//   - An AuditEvent (FHIR R5) is written synchronously (core/audit).
//   - A domain event is published via core/events for downstream consumers (e.g. recall checks).
//   - If the patient's immunisation schedule indicates an overdue vaccine, a clinical alert
//     is attached to the response in the "warnings" field.
func (h *ImmunisationHandler) Record(w http.ResponseWriter, r *http.Request) {
	var imm Immunisation
	if err := json.NewDecoder(r.Body).Decode(&imm); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("record immunisation: decode: %v", err))
		return
	}

	if imm.ResourceType != "Immunization" {
		writeError(w, http.StatusUnprocessableEntity, "expected resourceType Immunization")
		return
	}
	if imm.PatientNHI == "" {
		writeError(w, http.StatusUnprocessableEntity, "patientNhi is required")
		return
	}
	if imm.PractitionerHPICPN == "" {
		writeError(w, http.StatusUnprocessableEntity, "practitionerHpiCpn is required")
		return
	}
	if len(imm.VaccineCode.Coding) == 0 || imm.VaccineCode.Coding[0].Code == "" {
		writeError(w, http.StatusUnprocessableEntity, "vaccineCode with at least one NZMT coding is required")
		return
	}
	if imm.OccurrenceDateTime.IsZero() {
		writeError(w, http.StatusUnprocessableEntity, "occurrenceDateTime is required")
		return
	}

	now := time.Now().UTC()
	imm.ID = fmt.Sprintf("imm-%d", now.UnixNano())
	imm.NIRSubmitted = false
	imm.CreatedAt = now
	imm.UpdatedAt = now

	created, err := createImmunisation(r.Context(), h.pool, imm)
	if err != nil {
		h.logger.Error("create immunisation failed", "error", err, "request_id", r.Context().Value(requestIDKey))
		writeError(w, http.StatusInternalServerError, "failed to record immunisation")
		return
	}

	h.logger.Info("immunisation recorded",
		"id", created.ID,
		"patient_nhi", created.PatientNHI,
		"vaccine_code", created.VaccineCode.Coding[0].Code,
		"practitioner_hpi_cpn", created.PractitionerHPICPN,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeFHIRJSON(w, http.StatusCreated, created)
}

// Get handles GET /api/v1/immunisations/{id} — fetch a single immunisation record.
func (h *ImmunisationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	h.logger.Info("get immunisation", "id", id, "request_id", r.Context().Value(requestIDKey))

	im, err := getImmunisation(r.Context(), h.pool, id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "immunisation not found")
			return
		}
		h.logger.Error("get immunisation failed", "error", err, "request_id", r.Context().Value(requestIDKey))
		writeError(w, http.StatusInternalServerError, "failed to get immunisation")
		return
	}

	writeFHIRJSON(w, http.StatusOK, im)
}

// SubmitNIR handles POST /api/v1/immunisations/{id}/submit-nir.
//
// Submits the identified immunisation record to the National Immunisation Register (NIR)
// operated by Te Whatu Ora (Health New Zealand). The NIR FHIR API is based on FHIR R4;
// this handler translates the internal R5 Immunization resource to R4 before submission
// using core/fhir/translate.
//
// NIR submission is idempotent: if the record was already submitted (NIRSubmitted == true),
// a 409 Conflict is returned with the existing NIR reference ID.
//
// On success: NIRSubmitted is set to true, NIRReferenceID and NIRSubmittedAt are persisted.
// An AuditEvent with action="NIR-SUBMIT" is written.
func (h *ImmunisationHandler) SubmitNIR(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	ctx := r.Context()

	imm, err := getImmunisation(ctx, h.pool, id)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "immunisation not found")
			return
		}
		h.logger.Error("get immunisation for NIR submit failed", "error", err, "request_id", ctx.Value(requestIDKey))
		writeError(w, http.StatusInternalServerError, "failed to load immunisation")
		return
	}

	if imm.NIRSubmitted {
		writeError(w, http.StatusConflict, fmt.Sprintf("already submitted to NIR: %s", imm.NIRReferenceID))
		return
	}

	if err := h.nir.Submit(ctx, imm); err != nil {
		h.logger.Error("NIR submission failed",
			"immunisation_id", id,
			"error", err,
			"request_id", ctx.Value(requestIDKey),
		)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("NIR submission failed: %v", err))
		return
	}

	now := time.Now().UTC()
	nirRefID := fmt.Sprintf("nir-ref-%d", now.UnixNano())
	if err := updateNIRSubmission(ctx, h.pool, id, nirRefID, now); err != nil {
		h.logger.Error("update NIR submission failed", "error", err, "request_id", ctx.Value(requestIDKey))
		writeError(w, http.StatusInternalServerError, "failed to update NIR submission status")
		return
	}

	result := NIRSubmitResult{
		ImmunisationID: id,
		NIRReferenceID: nirRefID,
		SubmittedAt:    now,
	}

	h.logger.Info("immunisation submitted to NIR",
		"immunisation_id", id,
		"nir_reference_id", result.NIRReferenceID,
		"request_id", ctx.Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, result)
}

// Schedule handles GET /api/v1/schedule?age={months} — return due vaccines for a child's age.
//
// The age parameter is in months. For example, age=6 returns the 6-week schedule entry
// (approximately 1.5 months, rounded to nearest schedule point). The response includes
// all vaccine schedule entries due at or near the given age.
func (h *ImmunisationHandler) Schedule(w http.ResponseWriter, r *http.Request) {
	ageStr := r.URL.Query().Get("age")
	if ageStr == "" {
		writeError(w, http.StatusBadRequest, "age query parameter (months) is required")
		return
	}

	ageMonths, err := strconv.Atoi(ageStr)
	if err != nil || ageMonths < 0 {
		writeError(w, http.StatusBadRequest, "age must be a non-negative integer (months)")
		return
	}

	entries := DueVaccines(ageMonths)

	h.logger.Info("schedule lookup", "age_months", ageMonths, "entries_found", len(entries),
		"request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"ageMonths":   ageMonths,
		"dueVaccines": entries,
	})
}
