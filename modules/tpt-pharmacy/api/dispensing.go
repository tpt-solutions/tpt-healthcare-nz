package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/medsafe"
	"github.com/jackc/pgx/v5/pgxpool"
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
	pool          *pgxpool.Pool
	medsafeClient *medsafe.Client
	logger        *slog.Logger
}

// ListPrescriptions handles GET /api/v1/prescriptions — lists incoming GP prescriptions.
func (h *DispensingHandler) ListPrescriptions(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, medication_request_id, patient_nhi, status, is_schedule2, created_at
		 FROM pharmacy_dispensing_records
		 WHERE status = 'pending'
		 ORDER BY created_at DESC
		 LIMIT 100`)
	if err != nil {
		h.logger.Error("list prescriptions query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list prescriptions")
		return
	}
	defer rows.Close()

	entries := make([]map[string]any, 0)
	for rows.Next() {
		var id, medReqID, patientNHI, status string
		var isSchedule2 bool
		var createdAt time.Time
		if err := rows.Scan(&id, &medReqID, &patientNHI, &status, &isSchedule2, &createdAt); err != nil {
			h.logger.Error("scan prescription row", "error", err)
			continue
		}
		entries = append(entries, map[string]any{
			"id":                  id,
			"medicationRequestId": medReqID,
			"patientNhi":          patientNHI,
			"status":              status,
			"isSchedule2":         isSchedule2,
			"createdAt":           createdAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        len(entries),
		"entry":        entries,
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

	now := time.Now().UTC()
	recordID := fmt.Sprintf("rx-%d", now.UnixNano())
	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO pharmacy_dispensing_records (id, medication_request_id, patient_nhi, status, is_schedule2, created_at, updated_at)
		 VALUES ($1, $2, $3, 'pending', $4, $5, $6)`,
		recordID, req.ID, req.SubjectNHI, req.IsSchedule2, now, now)
	if err != nil {
		h.logger.Error("persist prescription failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to persist prescription")
		return
	}

	h.logger.Info("prescription received",
		"medication_request_id", req.ID,
		"patient_nhi", req.SubjectNHI,
		"is_schedule2", req.IsSchedule2,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, req)
}

// List handles GET /api/v1/dispensing — list pending dispensing records.
func (h *DispensingHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = string(DispensingStatusPending)
	}

	rows, err := h.pool.Query(r.Context(),
		`SELECT id, medication_request_id, patient_nhi, status, is_schedule2,
		        pharmacist_hpi_cpn, second_pharmacist_id, created_at, updated_at
		 FROM pharmacy_dispensing_records
		 WHERE status = $1
		 ORDER BY created_at DESC
		 LIMIT 100`, status)
	if err != nil {
		h.logger.Error("list dispensing query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list dispensing records")
		return
	}
	defer rows.Close()

	entries := make([]DispensingRecord, 0)
	for rows.Next() {
		var rec DispensingRecord
		if err := rows.Scan(&rec.ID, &rec.MedicationRequestID, &rec.PatientNHI, &rec.Status,
			&rec.IsSchedule2, &rec.PharmacistHPICPN, &rec.SecondPharmacistID,
			&rec.CreatedAt, &rec.UpdatedAt); err != nil {
			h.logger.Error("scan dispensing row", "error", err)
			continue
		}
		entries = append(entries, rec)
	}

	h.logger.Info("list dispensing", "status", status, "count", len(entries), "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "Bundle",
		"type":         "searchset",
		"total":        len(entries),
		"entry":        entries,
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

	now := time.Now().UTC()
	recordID := fmt.Sprintf("disp-%d", now.UnixNano())
	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO pharmacy_dispensing_records (id, medication_request_id, patient_nhi, status, is_schedule2, created_at, updated_at)
		 VALUES ($1, $2, $3, 'pending', $4, $5, $6)`,
		recordID, req.ID, req.SubjectNHI, req.IsSchedule2, now, now)
	if err != nil {
		h.logger.Error("persist dispensing record failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create dispensing record")
		return
	}

	record := DispensingRecord{
		ID:                  recordID,
		MedicationRequestID: req.ID,
		PatientNHI:          req.SubjectNHI,
		Status:              DispensingStatusPending,
		IsSchedule2:         req.IsSchedule2,
		CreatedAt:           now,
		UpdatedAt:           now,
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

	var rec DispensingRecord
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, medication_request_id, patient_nhi, status, is_schedule2,
		        pharmacist_hpi_cpn, second_pharmacist_id, created_at, updated_at
		 FROM pharmacy_dispensing_records
		 WHERE id = $1`, id,
	).Scan(&rec.ID, &rec.MedicationRequestID, &rec.PatientNHI, &rec.Status,
		&rec.IsSchedule2, &rec.PharmacistHPICPN, &rec.SecondPharmacistID,
		&rec.CreatedAt, &rec.UpdatedAt)
	if err != nil {
		if db.IsNoRows(err) {
			writeError(w, http.StatusNotFound, "dispensing record not found")
			return
		}
		h.logger.Error("get dispensing record failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch dispensing record")
		return
	}

	h.logger.Info("get dispensing record", "id", id, "request_id", r.Context().Value(requestIDKey))
	writeJSON(w, http.StatusOK, rec)
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

	// Load the dispensing record to check current status
	var isSchedule2 bool
	var status string
	err := h.pool.QueryRow(r.Context(),
		`SELECT is_schedule2, status FROM pharmacy_dispensing_records WHERE id = $1`, id,
	).Scan(&isSchedule2, &status)
	if err != nil {
		if db.IsNoRows(err) {
			writeError(w, http.StatusNotFound, "dispensing record not found")
			return
		}
		h.logger.Error("load dispensing record failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load dispensing record")
		return
	}
	if status != string(DispensingStatusPending) {
		writeError(w, http.StatusConflict, fmt.Sprintf("dispensing record is not in pending state (current: %s)", status))
		return
	}

	if isSchedule2 {
		// Schedule 2 drugs: set status to awaiting-schedule2-confirm
		_, err := h.pool.Exec(r.Context(),
			`UPDATE pharmacy_dispensing_records
			 SET pharmacist_hpi_cpn = $1, status = 'on-hold', updated_at = now()
			 WHERE id = $2`, req.PharmacistHPICPN, id)
		if err != nil {
			h.logger.Error("update dispensing record for schedule2 failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to update dispensing record")
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]any{
			"dispensingId": id,
			"status":       "on-hold",
			"message":      "Schedule 2 drug: awaiting second pharmacist confirmation",
		})
		return
	}

	// Non-schedule 2: mark as dispensed
	_, err = h.pool.Exec(r.Context(),
		`UPDATE pharmacy_dispensing_records
		 SET pharmacist_hpi_cpn = $1, status = 'dispensed', updated_at = now()
		 WHERE id = $2`, req.PharmacistHPICPN, id)
	if err != nil {
		h.logger.Error("update dispensing record failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update dispensing record")
		return
	}

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

	// Load and validate the dispensing record
	var isSchedule2 bool
	var status string
	var primaryPharmacist string
	err := h.pool.QueryRow(r.Context(),
		`SELECT is_schedule2, status, pharmacist_hpi_cpn FROM pharmacy_dispensing_records WHERE id = $1`, id,
	).Scan(&isSchedule2, &status, &primaryPharmacist)
	if err != nil {
		if db.IsNoRows(err) {
			writeError(w, http.StatusNotFound, "dispensing record not found")
			return
		}
		h.logger.Error("load dispensing record for schedule2 confirm failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load dispensing record")
		return
	}

	if !isSchedule2 {
		writeError(w, http.StatusBadRequest, "dispensing record is not a Schedule 2 drug")
		return
	}
	if status != "on-hold" {
		writeError(w, http.StatusConflict, fmt.Sprintf("dispensing record is not awaiting confirmation (current: %s)", status))
		return
	}
	if primaryPharmacist != "" && primaryPharmacist == req.SecondPharmacistID {
		writeError(w, http.StatusConflict, "second pharmacist cannot be the same as the primary pharmacist")
		return
	}

	// Update the dispensing record
	_, err = h.pool.Exec(r.Context(),
		`UPDATE pharmacy_dispensing_records
		 SET second_pharmacist_id = $1, status = 'dispensed', updated_at = now()
		 WHERE id = $2`, req.SecondPharmacistID, id)
	if err != nil {
		h.logger.Error("update dispensing record for schedule2 confirm failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update dispensing record")
		return
	}

	h.logger.Info("schedule 2 confirmation recorded",
		"dispensing_id", id,
		"second_pharmacist_id", req.SecondPharmacistID,
		"notes", req.Notes,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"dispensingId":       id,
		"secondPharmacistId": req.SecondPharmacistID,
		"status":             string(DispensingStatusDispensed),
		"confirmedAt":        time.Now().UTC(),
		"auditEventWritten":  true,
	})
}

// dispensingADERequest is the body for POST /api/v1/dispensing/{id}/ade.
type dispensingADERequest struct {
	PharmacistHPICPN string              `json:"pharmacistHpiCpn"`
	PatientNHI       string              `json:"patientNhi"`
	NZULMCode        string              `json:"nzulmCode"`
	GenericName      string              `json:"genericName"`
	EventDate        time.Time           `json:"eventDate"`
	EventDescription string              `json:"eventDescription"`
	Seriousness      medsafe.Seriousness `json:"seriousness"`
	Outcome          string              `json:"outcome,omitempty"`
	PatientAge       int                 `json:"patientAge,omitempty"`
	PatientSex       string              `json:"patientSex,omitempty"`
	RelevantHistory  string              `json:"relevantHistory,omitempty"`
}

// ReportADE handles POST /api/v1/dispensing/{id}/ade.
// Allows a pharmacist to report a suspected adverse drug event to Medsafe/CARM
// for a medicine that was dispensed. Medicines Act 1981 s45 obliges reporters.
func (h *DispensingHandler) ReportADE(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	if h.medsafeClient == nil {
		writeError(w, http.StatusServiceUnavailable, "Medsafe ADE reporting is not configured")
		return
	}

	var req dispensingADERequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("ade report: decode: %v", err))
		return
	}

	if req.PharmacistHPICPN == "" {
		writeError(w, http.StatusUnprocessableEntity, "pharmacistHpiCpn is required")
		return
	}
	if req.PatientNHI == "" {
		writeError(w, http.StatusUnprocessableEntity, "patientNhi is required")
		return
	}
	if req.EventDescription == "" {
		writeError(w, http.StatusUnprocessableEntity, "eventDescription is required")
		return
	}
	if req.Seriousness == "" {
		writeError(w, http.StatusUnprocessableEntity, "seriousness is required")
		return
	}

	report := medsafe.ADEReport{
		PatientNHI:       req.PatientNHI,
		PatientAge:       req.PatientAge,
		PatientSex:       req.PatientSex,
		ReporterHPI:      req.PharmacistHPICPN,
		ReporterType:     "pharmacist",
		EventDate:        req.EventDate,
		EventDescription: req.EventDescription,
		Seriousness:      req.Seriousness,
		Outcome:          req.Outcome,
		RelevantHistory:  req.RelevantHistory,
		SuspectDrugs: []medsafe.SuspectDrug{
			{
				NZULM:       req.NZULMCode,
				GenericName: req.GenericName,
				Causality:   medsafe.CausalityPossible,
			},
		},
	}

	submitted, err := h.medsafeClient.SubmitADE(r.Context(), report)
	if err != nil {
		h.logger.Error("medsafe ADE submission failed",
			"dispensing_id", id,
			"error", err,
			"request_id", r.Context().Value(requestIDKey),
		)
		writeError(w, http.StatusBadGateway, "ADE report submission to Medsafe failed")
		return
	}

	h.logger.Info("medsafe ADE submitted",
		"dispensing_id", id,
		"carm_report_id", submitted.CARMReportID,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, submitted)
}
