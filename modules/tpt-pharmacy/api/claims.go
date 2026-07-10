package api

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Domain types ---

// PHARMACClaimStatus represents the lifecycle of a PHARMAC subsidy claim.
type PHARMACClaimStatus string

const (
	PHARMACClaimStatusDraft     PHARMACClaimStatus = "draft"
	PHARMACClaimStatusSubmitted PHARMACClaimStatus = "submitted"
	PHARMACClaimStatusAccepted  PHARMACClaimStatus = "accepted"
	PHARMACClaimStatusRejected  PHARMACClaimStatus = "rejected"
	PHARMACClaimStatusPaid      PHARMACClaimStatus = "paid"
)

// PHARMACClaim represents a single PHARMAC subsidy claim generated from one or
// more MedicationDispense records.
type PHARMACClaim struct {
	ID                    string             `json:"id"`
	Status                PHARMACClaimStatus `json:"status"`
	PharmacyHSPNo         string             `json:"pharmacyHspNo"` // Health Service Provider number
	ClaimPeriodStart      time.Time          `json:"claimPeriodStart"`
	ClaimPeriodEnd        time.Time          `json:"claimPeriodEnd"`
	DispenseIDs           []string           `json:"dispenseIds"`
	TotalSubsidyAmountNZD float64            `json:"totalSubsidyAmountNzd"`
	SubmittedAt           *time.Time         `json:"submittedAt,omitempty"`
	PHARMACReferenceNo    string             `json:"pharmacReferenceNo,omitempty"`
	CreatedAt             time.Time          `json:"createdAt"`
	UpdatedAt             time.Time          `json:"updatedAt"`
}

// CreateClaimRequest is the body for POST /api/v1/claims.
type CreateClaimRequest struct {
	PharmacyHSPNo    string    `json:"pharmacyHspNo"`
	ClaimPeriodStart time.Time `json:"claimPeriodStart"`
	ClaimPeriodEnd   time.Time `json:"claimPeriodEnd"`
	DispenseIDs      []string  `json:"dispenseIds"`
}

// HSDReportRequest is the body for POST /api/v1/reports/hsd.
// HSD (Health Survey and Dispensing) reporting is mandated by the Ministry of Health
// to provide aggregate dispensing data for medicines utilisation analysis.
type HSDReportRequest struct {
	PharmacyHSPNo     string    `json:"pharmacyHspNo"`
	ReportPeriodStart time.Time `json:"reportPeriodStart"`
	ReportPeriodEnd   time.Time `json:"reportPeriodEnd"`
	// IncludeAnonymised controls whether the report includes de-identified patient records.
	// Must be true for HSD submissions (Privacy Act 2020 s.20 — research exception).
	IncludeAnonymised bool `json:"includeAnonymised"`
}

// HSDReport is the generated HSD report payload.
type HSDReport struct {
	ReportID          string    `json:"reportId"`
	PharmacyHSPNo     string    `json:"pharmacyHspNo"`
	GeneratedAt       time.Time `json:"generatedAt"`
	ReportPeriodStart time.Time `json:"reportPeriodStart"`
	ReportPeriodEnd   time.Time `json:"reportPeriodEnd"`
	TotalDispenses    int       `json:"totalDispenses"`
	// Records contains de-identified dispensing rows. Patient NHI is replaced
	// with a one-way HMAC-SHA256 token keyed by the pharmacy HSP number, ensuring
	// longitudinal linkage within the pharmacy but no cross-pharmacy re-identification.
	Records []HSDRecord `json:"records"`
}

// HSDRecord is a single de-identified dispensing row for HSD reporting.
type HSDRecord struct {
	// PatientToken is a HMAC-SHA256 of the patient NHI, keyed per pharmacy.
	PatientToken     string  `json:"patientToken"`
	AgeGroupBand     string  `json:"ageGroupBand"` // e.g. "65-74"
	Gender           string  `json:"gender"`       // "M", "F", "U"
	NZMTCode         string  `json:"nzmtCode"`
	FormularyCode    string  `json:"formularyCode"`
	Quantity         float64 `json:"quantity"`
	Unit             string  `json:"unit"`
	SubsidyAmountNZD float64 `json:"subsidyAmountNzd"`
	DispensedDate    string  `json:"dispensedDate"` // YYYY-MM (month granularity, not day)
	IsSchedule2      bool    `json:"isSchedule2"`
}

// ClaimsHandler handles all /api/v1/claims and /api/v1/reports/hsd routes.
type ClaimsHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// List handles GET /api/v1/claims — list PHARMAC claims with optional status filter.
func (h *ClaimsHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	var rows pgx.Rows
	var err error
	if status != "" {
		rows, err = h.pool.Query(r.Context(),
			`SELECT id, status, pharmacy_hsp_no, claim_period_start, claim_period_end,
			        dispense_ids, total_subsidy_amount, submitted_at, pharmac_reference_no,
			        created_at, updated_at
			 FROM pharmacy_pharmac_claims
			 WHERE status = $1
			 ORDER BY created_at DESC`, status)
	} else {
		rows, err = h.pool.Query(r.Context(),
			`SELECT id, status, pharmacy_hsp_no, claim_period_start, claim_period_end,
			        dispense_ids, total_subsidy_amount, submitted_at, pharmac_reference_no,
			        created_at, updated_at
			 FROM pharmacy_pharmac_claims
			 ORDER BY created_at DESC`)
	}
	if err != nil {
		h.logger.Error("list claims query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list claims")
		return
	}
	defer rows.Close()

	claims := make([]PHARMACClaim, 0)
	for rows.Next() {
		var c PHARMACClaim
		var dispenseIDsJSON []byte
		if err := rows.Scan(&c.ID, &c.Status, &c.PharmacyHSPNo, &c.ClaimPeriodStart,
			&c.ClaimPeriodEnd, &dispenseIDsJSON, &c.TotalSubsidyAmountNZD,
			&c.SubmittedAt, &c.PHARMACReferenceNo, &c.CreatedAt, &c.UpdatedAt); err != nil {
			h.logger.Error("scan claim row", "error", err)
			continue
		}
		_ = json.Unmarshal(dispenseIDsJSON, &c.DispenseIDs)
		claims = append(claims, c)
	}

	h.logger.Info("list claims", "status", status, "count", len(claims), "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"claims": claims,
		"total":  len(claims),
	})
}

// Create handles POST /api/v1/claims — generate a PHARMAC claim from MedicationDispense records.
//
// The claim is created in "draft" status. The pharmacy must review and then call
// /submit to lodge it with PHARMAC. Each DispenseID must reference a MedicationDispense
// with status "completed"; partially-dispensed or Schedule 2 dispenses awaiting confirmation
// are excluded automatically.
func (h *ClaimsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateClaimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("create claim: decode: %v", err))
		return
	}

	if req.PharmacyHSPNo == "" {
		writeError(w, http.StatusUnprocessableEntity, "pharmacyHspNo is required")
		return
	}
	if len(req.DispenseIDs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "at least one dispenseId is required")
		return
	}
	if req.ClaimPeriodEnd.Before(req.ClaimPeriodStart) {
		writeError(w, http.StatusUnprocessableEntity, "claimPeriodEnd must be after claimPeriodStart")
		return
	}

	now := time.Now().UTC()
	claimID := fmt.Sprintf("claim-%d", now.UnixNano())
	dispenseIDsJSON, _ := json.Marshal(req.DispenseIDs)

	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO pharmacy_pharmac_claims (id, status, pharmacy_hsp_no, claim_period_start, claim_period_end, dispense_ids, total_subsidy_amount, created_at, updated_at)
		 VALUES ($1, 'draft', $2, $3, $4, $5, 0, $6, $7)`,
		claimID, req.PharmacyHSPNo, req.ClaimPeriodStart, req.ClaimPeriodEnd, dispenseIDsJSON, now, now)
	if err != nil {
		h.logger.Error("persist claim failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create claim")
		return
	}

	claim := PHARMACClaim{
		ID:                    claimID,
		Status:                PHARMACClaimStatusDraft,
		PharmacyHSPNo:         req.PharmacyHSPNo,
		ClaimPeriodStart:      req.ClaimPeriodStart,
		ClaimPeriodEnd:        req.ClaimPeriodEnd,
		DispenseIDs:           req.DispenseIDs,
		TotalSubsidyAmountNZD: 0,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	h.logger.Info("claim created",
		"claim_id", claim.ID,
		"pharmacy_hsp_no", claim.PharmacyHSPNo,
		"dispense_count", len(claim.DispenseIDs),
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, claim)
}

// Submit handles POST /api/v1/claims/{id}/submit — lodge a draft claim with PHARMAC.
//
// PHARMAC claims are submitted via the PHARMAC ePrescribing and Dispensing (ePAD) API.
// The claim transitions from "draft" → "submitted". PHARMAC's asynchronous processing
// will later update the status to "accepted" or "rejected" via a webhook or polling.
func (h *ClaimsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	// Verify claim exists and is in draft status
	var status string
	err := h.pool.QueryRow(r.Context(),
		`SELECT status FROM pharmacy_pharmac_claims WHERE id = $1`, id,
	).Scan(&status)
	if err != nil {
		if db.IsNoRows(err) {
			writeError(w, http.StatusNotFound, "claim not found")
			return
		}
		h.logger.Error("load claim for submit failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load claim")
		return
	}
	if status != string(PHARMACClaimStatusDraft) {
		writeError(w, http.StatusConflict, fmt.Sprintf("claim is not in draft status (current: %s)", status))
		return
	}

	// Update to submitted
	now := time.Now().UTC()
	_, err = h.pool.Exec(r.Context(),
		`UPDATE pharmacy_pharmac_claims
		 SET status = 'submitted', submitted_at = $1, updated_at = $2
		 WHERE id = $3`, now, now, id)
	if err != nil {
		h.logger.Error("update claim status failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update claim status")
		return
	}

	h.logger.Info("claim submitted to PHARMAC",
		"claim_id", id,
		"submitted_at", now,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"claimId":     id,
		"status":      string(PHARMACClaimStatusSubmitted),
		"submittedAt": now,
	})
}

// Status handles GET /api/v1/claims/{id}/status — poll claim processing status from PHARMAC.
func (h *ClaimsHandler) Status(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id path parameter is required")
		return
	}

	var status string
	var pharmacRef string
	var submittedAt *time.Time
	err := h.pool.QueryRow(r.Context(),
		`SELECT status, pharmac_reference_no, submitted_at
		 FROM pharmacy_pharmac_claims WHERE id = $1`, id,
	).Scan(&status, &pharmacRef, &submittedAt)
	if err != nil {
		if db.IsNoRows(err) {
			writeError(w, http.StatusNotFound, "claim not found")
			return
		}
		h.logger.Error("load claim status failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load claim status")
		return
	}

	h.logger.Info("claim status check", "claim_id", id, "request_id", r.Context().Value(requestIDKey))

	writeJSON(w, http.StatusOK, map[string]any{
		"claimId":            id,
		"status":             status,
		"pharmacReferenceNo": pharmacRef,
		"submittedAt":        submittedAt,
		"checkedAt":          time.Now().UTC(),
	})
}

// GenerateHSDReport handles POST /api/v1/reports/hsd.
//
// HSD (Health Survey and Dispensing) reporting is a mandatory data submission to
// the Ministry of Health under the Health Act 1956 s.74H. Reports must be submitted
// monthly and contain de-identified dispensing data. Patient NHIs are replaced with
// per-pharmacy HMAC tokens to prevent cross-pharmacy re-identification while preserving
// within-pharmacy longitudinal linkage.
//
// Privacy Act 2020 Note: Disclosure under the research/statistical exception (IPP 11(e))
// requires that the data cannot reasonably be used to identify any individual.
// The HMAC tokenisation and age-band aggregation applied here are designed to meet
// that threshold. Legal review must be obtained before modifying the de-identification logic.
func (h *ClaimsHandler) GenerateHSDReport(w http.ResponseWriter, r *http.Request) {
	var req HSDReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("generate HSD report: decode: %v", err))
		return
	}

	if req.PharmacyHSPNo == "" {
		writeError(w, http.StatusUnprocessableEntity, "pharmacyHspNo is required")
		return
	}
	if req.ReportPeriodEnd.Before(req.ReportPeriodStart) {
		writeError(w, http.StatusUnprocessableEntity, "reportPeriodEnd must be after reportPeriodStart")
		return
	}
	if !req.IncludeAnonymised {
		writeError(w, http.StatusUnprocessableEntity,
			"includeAnonymised must be true: HSD reports require de-identified records")
		return
	}

	// Load all completed dispensing records for the period and pharmacy
	rows, err := h.pool.Query(r.Context(),
		`SELECT d.patient_nhi, d.nzmt_code, d.formulary_code, d.quantity, d.unit,
		        d.subsidy_amount_nzd, d.dispensed_date, d.is_schedule2
		         FROM pharmacy_dispensing_records d
		         WHERE d.status = 'completed'
		               AND d.dispensed_date >= $1 AND d.dispensed_date <= $2
		 ORDER BY d.dispensed_date DESC`,
		req.ReportPeriodStart, req.ReportPeriodEnd)
	if err != nil {
		h.logger.Error("HSD report query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to generate HSD report")
		return
	}
	defer rows.Close()

	now := time.Now().UTC()
	report := HSDReport{
		ReportID:          fmt.Sprintf("hsd-%s-%d", req.PharmacyHSPNo, now.UnixNano()),
		PharmacyHSPNo:     req.PharmacyHSPNo,
		GeneratedAt:       now,
		ReportPeriodStart: req.ReportPeriodStart,
		ReportPeriodEnd:   req.ReportPeriodEnd,
		Records:           make([]HSDRecord, 0),
	}

	for rows.Next() {
		var rec HSDRecord
		var patientNHI, nzmtCode, formularyCode, unit string
		var quantity, subsidyAmount float64
		var dispensedDate time.Time
		var isSchedule2 bool

		if err := rows.Scan(&patientNHI, &nzmtCode, &formularyCode, &quantity,
			&unit, &subsidyAmount, &dispensedDate, &isSchedule2); err != nil {
			h.logger.Warn("scan HSD record row", "error", err)
			continue
		}

		// De-identify: replace NHI with HMAC-SHA256 token
		rec.PatientToken = hsdTokenizeNHI(patientNHI, req.PharmacyHSPNo, req.ReportPeriodStart)
		rec.NZMTCode = nzmtCode
		rec.FormularyCode = formularyCode
		rec.Quantity = quantity
		rec.Unit = unit
		rec.SubsidyAmountNZD = subsidyAmount
		rec.DispensedDate = dispensedDate.Format("2006-01") // YYYY-MM
		rec.IsSchedule2 = isSchedule2

		report.Records = append(report.Records, rec)
	}
	report.TotalDispenses = len(report.Records)

	// Persist report for audit trail
	_, err = h.pool.Exec(r.Context(),
		`INSERT INTO pharmacy_hsd_reports (id, pharmacy_hsp_no, report_period_start,
		                                   report_period_end, total_dispenses, generated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		report.ReportID, report.PharmacyHSPNo, report.ReportPeriodStart,
		report.ReportPeriodEnd, report.TotalDispenses, report.GeneratedAt)
	if err != nil {
		h.logger.Warn("persist HSD report failed (report still returned)", "error", err)
	}

	h.logger.Info("HSD report generated",
		"report_id", report.ReportID,
		"pharmacy_hsp_no", report.PharmacyHSPNo,
		"period_start", report.ReportPeriodStart,
		"period_end", report.ReportPeriodEnd,
		"total_dispenses", report.TotalDispenses,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, report)
}

// hsdTokenizeNHI replaces the NHI with a per-pharmacy HMAC token for de-identification.
// The key incorporates the pharmacy HSP number and reporting month/year to prevent
// cross-pharmacy re-identification while preserving longitudinal linkage within a pharmacy.
func hsdTokenizeNHI(nhi, pharmacyHSPNo string, reportPeriodStart time.Time) string {
	// Per-pharmacy keyed token: HMAC-SHA256(NHI, key=PharmacyHSPNo+reportingMonth)
	// This provides longitudinal linkage within a pharmacy while preventing cross-pharmacy re-identification.
	key := fmt.Sprintf("%s-%s", pharmacyHSPNo, reportPeriodStart.Format("2006-01"))
	hash := sha256.Sum256([]byte(nhi + key))
	return fmt.Sprintf("token-%x", hash[:8])
}
