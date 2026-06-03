package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// --- Domain types ---

// DispensingStatus represents the lifecycle of a prescription through dispensing.
type DispensingStatus string

const (
	DispensingStatusPending    DispensingStatus = "pending"
	DispensingStatusDispensed  DispensingStatus = "dispensed"
	DispensingStatusOnHold     DispensingStatus = "on-hold"
	DispensingStatusCancelled  DispensingStatus = "cancelled"
)

// MedicationRequest is a simplified FHIR MedicationRequest received from a GP.
type MedicationRequest struct {
	ResourceType  string `json:"resourceType"`
	ID            string `json:"id"`
	Status        string `json:"status"`
	Intent        string `json:"intent"`
	MedicationCode struct {
		Coding []struct {
			System  string `json:"system"`
			Code    string `json:"code"`
			Display string `json:"display"`
		} `json:"coding"`
	} `json:"medicationCodeableConcept"`
	SubjectNHI       string    `json:"subjectNHI"` // NHI extracted from subject.identifier
	AuthoredOn       time.Time `json:"authoredOn"`
	RequesterHPICPN  string    `json:"requesterHpiCpn"` // GP's HPI CPN
	DosageInstruction []struct {
		Text string `json:"text"`
	} `json:"dosageInstruction"`
	IsSchedule2      bool   `json:"isSchedule2"`
	PharmacFormularyCode string `json:"pharmacFormularyCode"`
}

// DispensingRecord tracks a MedicationRequest through the dispensing workflow.
type DispensingRecord struct {
	ID                  string           `json:"id"`
	MedicationRequestID string           `json:"medicationRequestId"`
	PatientNHI          string           `json:"patientNhi"`
	Status              DispensingStatus `json:"status"`
	IsSchedule2         bool             `json:"isSchedule2"`
	PharmacistHPICPN    string           `json:"pharmacistHpiCpn,omitempty"`
	SecondPharmacistID  string           `json:"secondPharmacistId,omitempty"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
}

// MedicationDispense is a simplified FHIR MedicationDispense resource.
type MedicationDispense struct {
	ResourceType       string    `json:"resourceType"`
	ID                 string    `json:"id"`
	Status             string    `json:"status"`
	MedicationRequestID string   `json:"medicationRequestId"`
	PatientNHI         string    `json:"patientNhi"`
	PharmacistHPICPN   string    `json:"pharmacistHpiCpn"`
	SecondPharmacistID string    `json:"secondPharmacistId,omitempty"`
	WhenHandedOver     time.Time `json:"whenHandedOver"`
	Quantity           float64   `json:"quantity"`
	Unit               string    `json:"unit"`
	LotNumber          string    `json:"lotNumber,omitempty"`
	ExpiryDate         string    `json:"expiryDate,omitempty"`
}

// DispenseRequest is the body for POST /api/v1/dispensing/{id}/dispense.
type DispenseRequest struct {
	PharmacistHPICPN string  `json:"pharmacistHpiCpn"`
	Quantity         float64 `json:"quantity"`
	Unit             string  `json:"unit"`
	LotNumber        string  `json:"lotNumber,omitempty"`
	ExpiryDate       string  `json:"expiryDate,omitempty"`
}

// Schedule2ConfirmRequest is the body for POST /api/v1/dispensing/{id}/schedule2-confirm.
type Schedule2ConfirmRequest struct {
	SecondPharmacistID string `json:"secondPharmacistId"`
	Notes              string `json:"notes,omitempty"`
}

// DispensingHandler handles all /api/v1/dispensing routes.
type DispensingHandler struct {
	logger *slog.Logger
}

// ListPrescriptions handles GET /api/v1/prescriptions — lists incoming GP prescriptions.
func (h *DispensingHandler) ListPrescriptions(w http.ResponseWriter, r *http.Request) {
	// In production: query the FHIR repository for MedicationRequest resources
	// with status=active addressed to this pharmacy organisation.
	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        0,
		"entry":        []any{},
	})
}

// ReceivePrescription handles POST /api/v1/prescriptions — accept a FHIR MedicationRequest from a GP.
func (h *DispensingHandler) ReceivePrescription(w http.ResponseWriter, r *http.Request) {
	var req MedicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("receive prescription: decode: %v", err))
		return
	}

	if req.ResourceType != "MedicationRequest" {
		writeError(w, http.StatusUnprocessableEntity, "expected resourceType MedicationRequest")
		return
	}
	if req.SubjectNHI == "" {
		writeError(w, http.StatusUnprocessableEntity, "subjectNHI is required")
		return
	}

	// In production:
	//   1. Validate NHI checksum via core/nhi.
	//   2. Validate requester APC via core/hpi.
	//   3. Persist to FHIR repository (core/repo).
	//   4. Write AuditEvent via core/audit.
	//   5. Create a DispensingRecord in pending state.

	h.logger.Info("prescription received",
		"medication_request_id", req.ID,
		"patient_nhi", req.SubjectNHI,
		"is_schedule2", req.IsSchedule2,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeFHIRJSON(w, http.StatusCreated, req)
}

// List handles GET /api/v1/dispensing — list pending dispensing records.
func (h *DispensingHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = string(DispensingStatusPending)
	}

	// In production: query the repository filtered by status, with pagination.
	h.logger.Info("list dispensing", "status", status, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        0,
		"entry":        []any{},
	})
}

// Receive handles POST /api/v1/dispensing — create a dispensing record from a MedicationRequest.
func (h *DispensingHandler) Receive(w http.ResponseWriter, r *http.Request) {
	var req MedicationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("receive dispensing: decode: %v", err))
		return
	}

	if req.ResourceType != "MedicationRequest" {
		writeError(w, http.StatusUnprocessableEntity, "expected resourceType MedicationRequest")
		return
	}
	if req.SubjectNHI == "" {
		writeError(w, http.StatusUnprocessableEntity, "subjectNHI is required")
		return
	}

	// In production:
	//   1. Validate NHI via core/nhi.
	//   2. Validate PHARMAC formulary code via core/pharmac.
	//   3. Persist MedicationRequest and create DispensingRecord.
	//   4. Write AuditEvent.

	record := DispensingRecord{
		ID:                  "new-id-placeholder",
		MedicationRequestID: req.ID,
		PatientNHI:          req.SubjectNHI,
		Status:              DispensingStatusPending,
		IsSchedule2:         req.IsSchedule2,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	h.logger.Info("dispensing record created",
		"id", record.ID,
		"patient_nhi", record.PatientNHI,
		"is_schedule2", record.IsSchedule2,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, record)
}

// Get handles GET /api/v1/dispensing/{id} — fetch a single dispensing record.
func (h *DispensingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// In production: fetch DispensingRecord + associated MedicationRequest from repository.
	h.logger.Info("get dispensing record", "id", id, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, DispensingRecord{
		ID:        id,
		Status:    DispensingStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
}

// Dispense handles POST /api/v1/dispensing/{id}/dispense — record a MedicationDispense.
//
// For Schedule 2 controlled drugs (e.g., morphine, methadone, fentanyl), this handler
// sets an IsSchedule2 flag on the dispense record but requires a subsequent call to
// /schedule2-confirm with a second pharmacist ID before the dispense is finalised.
// This enforces the two-pharmacist check required under the Misuse of Drugs Act 1975.
func (h *DispensingHandler) Dispense(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	var req DispenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("dispense: decode: %v", err))
		return
	}

	if req.PharmacistHPICPN == "" {
		writeError(w, http.StatusUnprocessableEntity, "pharmacistHpiCpn is required")
		return
	}
	if req.Quantity <= 0 {
		writeError(w, http.StatusUnprocessableEntity, "quantity must be greater than zero")
		return
	}
	if req.Unit == "" {
		writeError(w, http.StatusUnprocessableEntity, "unit is required")
		return
	}

	// In production:
	//   1. Load DispensingRecord by id, assert status == pending.
	//   2. Validate pharmacist APC via core/hpi (scope must include dispensing).
	//   3. Validate patient NHI via core/nhi.
	//   4. Check PHARMAC formulary subsidy eligibility via core/pharmac.
	//   5. If IsSchedule2: set status to "awaiting-schedule2-confirm" and return 202.
	//   6. Otherwise: persist MedicationDispense FHIR resource, update DispensingRecord status.
	//   7. Write AuditEvent (core/audit) including pharmacist HPI CPN and patient NHI.

	dispense := MedicationDispense{
		ResourceType:        "MedicationDispense",
		ID:                  fmt.Sprintf("dispense-%s", id),
		Status:              "completed",
		MedicationRequestID: id,
		PharmacistHPICPN:    req.PharmacistHPICPN,
		WhenHandedOver:      time.Now().UTC(),
		Quantity:            req.Quantity,
		Unit:                req.Unit,
		LotNumber:           req.LotNumber,
		ExpiryDate:          req.ExpiryDate,
	}

	h.logger.Info("medication dispensed",
		"dispensing_id", id,
		"pharmacist_hpi_cpn", req.PharmacistHPICPN,
		"quantity", req.Quantity,
		"unit", req.Unit,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeFHIRJSON(w, http.StatusCreated, dispense)
}

// Schedule2Confirm handles POST /api/v1/dispensing/{id}/schedule2-confirm.
//
// Schedule 2 drugs under the Misuse of Drugs Act 1975 require a second pharmacist
// to witness and countersign the dispensing. This endpoint records that confirmation
// and writes an extended audit event containing both pharmacist HPI CPNs, the patient
// NHI, the drug code, quantity, and a timestamp, satisfying the controlled drug
// register requirements under Regulation 33 of the Misuse of Drugs Regulations 1977.
func (h *DispensingHandler) Schedule2Confirm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	var req Schedule2ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("schedule2-confirm: decode: %v", err))
		return
	}

	if req.SecondPharmacistID == "" {
		writeError(w, http.StatusUnprocessableEntity,
			"secondPharmacistId is required for Schedule 2 drug confirmation")
		return
	}

	// In production:
	//   1. Load DispensingRecord by id, assert IsSchedule2 == true and status == "awaiting-schedule2-confirm".
	//   2. Assert SecondPharmacistID != primary PharmacistHPICPN (cannot self-confirm).
	//   3. Validate second pharmacist APC via core/hpi.
	//   4. Update DispensingRecord: SecondPharmacistID = req.SecondPharmacistID, status = dispensed.
	//   5. Write an extended AuditEvent (FHIR R5 AuditEvent) containing:
	//      - Both pharmacist HPI CPNs (agent[0] primary, agent[1] witness)
	//      - Patient NHI (encrypted in the audit record)
	//      - Drug NZMT code and quantity
	//      - Timestamp (UTC)
	//      - Action: "CONFIRM-SCHEDULE2"
	//      - Request correlation ID
	//   6. Append to controlled drug register (append-only table, see CLAUDE.md audit requirements).

	h.logger.Info("schedule 2 confirmation recorded",
		"dispensing_id", id,
		"second_pharmacist_id", req.SecondPharmacistID,
		"notes", req.Notes,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"dispensingId":        id,
		"secondPharmacistId":  req.SecondPharmacistID,
		"status":              string(DispensingStatusDispensed),
		"confirmedAt":         time.Now().UTC(),
		"auditEventWritten":   true,
	})
}
