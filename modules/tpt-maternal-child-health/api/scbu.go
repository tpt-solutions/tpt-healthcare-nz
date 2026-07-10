package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// SCBUStatus tracks the clinical status of a SCBU admission.
type SCBUStatus string

const (
	SCBUStatusAdmitted        SCBUStatus = "admitted"
	SCBUStatusStepDown        SCBUStatus = "step-down"
	SCBUStatusTransferredNICU SCBUStatus = "transferred-nicu"
	SCBUStatusDischarged      SCBUStatus = "discharged"
)

// SCBUFeedingMethod classifies how the neonate is being fed.
type SCBUFeedingMethod string

const (
	SCBUFeedingBreast SCBUFeedingMethod = "breast"
	SCBUFeedingNGT    SCBUFeedingMethod = "ngt"
	SCBUFeedingBottle SCBUFeedingMethod = "bottle"
	SCBUFeedingMixed  SCBUFeedingMethod = "mixed"
)

type SCBUAdmission struct {
	ID                 string     `json:"id"`
	EpisodeID          string     `json:"episodeId"`
	PatientNHI         string     `json:"patientNhi"`
	NeonatologistHpi   string     `json:"neonatologistHpi"`
	Status             string     `json:"status"`
	BedLabel           string     `json:"bedLabel"`
	AdmissionReason    string     `json:"admissionReason"`
	GestationWeeks     *int16     `json:"gestationWeeks"`
	BirthWeightGrams   *int       `json:"birthWeightGrams"`
	Apgar1min          *int16     `json:"apgar1min"`
	Apgar5min          *int16     `json:"apgar5min"`
	PhototherapyActive bool       `json:"phototherapyActive"`
	FeedingMethod      string     `json:"feedingMethod"`
	TenantID           string     `json:"tenantId"`
	AdmittedAt         time.Time  `json:"admittedAt"`
	DischargedAt       *time.Time `json:"dischargedAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type SCBUChartEntry struct {
	ID              string    `json:"id"`
	ScbuAdmissionID string    `json:"scbuAdmissionId"`
	NurseHpi        string    `json:"nurseHpi"`
	WeightGrams     *int      `json:"weightGrams"`
	BilirubinUmol   *float64  `json:"bilirubinUmol"`
	FeedVolumeMl    *float64  `json:"feedVolumeMl"`
	Notes           *string   `json:"notes"`
	TenantID        string    `json:"tenantId"`
	RecordedAt      time.Time `json:"recordedAt"`
}

// scbuHandler manages SCBU admissions and chart entries.
type scbuHandler struct {
	handlerDeps
}

func (h *scbuHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, episode_id, patient_nhi, neonatologist_hpi, status, bed_label,
		       admission_reason, gestation_weeks, birth_weight_grams, apgar_1min, apgar_5min,
		       phototherapy_active, feeding_method, tenant_id, admitted_at, discharged_at, created_at, updated_at
		FROM scbu_admissions
		WHERE tenant_id = @tenant_id
		ORDER BY admitted_at DESC
	`, pgx.NamedArgs{"tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	admissions := make([]SCBUAdmission, 0)
	for rows.Next() {
		var a SCBUAdmission
		if err := rows.Scan(
			&a.ID, &a.EpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status, &a.BedLabel,
			&a.AdmissionReason, &a.GestationWeeks, &a.BirthWeightGrams, &a.Apgar1min, &a.Apgar5min,
			&a.PhototherapyActive, &a.FeedingMethod, &a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		nhi, err := h.decryptNHI(a.PatientNHI)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
			return
		}
		a.PatientNHI = nhi
		admissions = append(admissions, a)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, admissions)
}

func (h *scbuHandler) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	var req SCBUAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.FeedingMethod == "" {
		req.FeedingMethod = "breast"
	}
	if !h.validateHPI(w, r, req.NeonatologistHpi) {
		return
	}
	nhiEnc, err := h.encryptNHI(req.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to encrypt patient NHI"})
		return
	}
	var a SCBUAdmission
	err = h.pool.QueryRow(r.Context(), `
		INSERT INTO scbu_admissions
		    (episode_id, patient_nhi, neonatologist_hpi, status, bed_label,
		     admission_reason, gestation_weeks, birth_weight_grams, apgar_1min, apgar_5min,
		     phototherapy_active, feeding_method, tenant_id)
		VALUES
		    (@episode_id, @patient_nhi, @neonatologist_hpi, 'admitted', @bed_label,
		     @admission_reason, @gestation_weeks, @birth_weight_grams, @apgar_1min, @apgar_5min,
		     @phototherapy_active, @feeding_method, @tenant_id)
		RETURNING id, episode_id, patient_nhi, neonatologist_hpi, status, bed_label,
		          admission_reason, gestation_weeks, birth_weight_grams, apgar_1min, apgar_5min,
		          phototherapy_active, feeding_method, tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"episode_id":          req.EpisodeID,
		"patient_nhi":         nhiEnc,
		"neonatologist_hpi":   req.NeonatologistHpi,
		"bed_label":           req.BedLabel,
		"admission_reason":    req.AdmissionReason,
		"gestation_weeks":     req.GestationWeeks,
		"birth_weight_grams":  req.BirthWeightGrams,
		"apgar_1min":          req.Apgar1min,
		"apgar_5min":          req.Apgar5min,
		"phototherapy_active": req.PhototherapyActive,
		"feeding_method":      req.FeedingMethod,
		"tenant_id":           tenantID,
	}).Scan(
		&a.ID, &a.EpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status, &a.BedLabel,
		&a.AdmissionReason, &a.GestationWeeks, &a.BirthWeightGrams, &a.Apgar1min, &a.Apgar5min,
		&a.PhototherapyActive, &a.FeedingMethod, &a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	h.recordAudit(r, "create", "SCBUAdmission", a.ID, nhiEnc)
	writeJSON(w, http.StatusCreated, a)
}

func (h *scbuHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var a SCBUAdmission
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, episode_id, patient_nhi, neonatologist_hpi, status, bed_label,
		       admission_reason, gestation_weeks, birth_weight_grams, apgar_1min, apgar_5min,
		       phototherapy_active, feeding_method, tenant_id, admitted_at, discharged_at, created_at, updated_at
		FROM scbu_admissions
		WHERE id = @id AND tenant_id = @tenant_id
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID}).Scan(
		&a.ID, &a.EpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status, &a.BedLabel,
		&a.AdmissionReason, &a.GestationWeeks, &a.BirthWeightGrams, &a.Apgar1min, &a.Apgar5min,
		&a.PhototherapyActive, &a.FeedingMethod, &a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "SCBU admission not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusOK, a)
}

func (h *scbuHandler) Update(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req SCBUAdmission
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	var a SCBUAdmission
	err := h.pool.QueryRow(r.Context(), `
		UPDATE scbu_admissions
		SET status = @status,
		    phototherapy_active = @phototherapy_active,
		    feeding_method = @feeding_method,
		    updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id
		RETURNING id, episode_id, patient_nhi, neonatologist_hpi, status, bed_label,
		          admission_reason, gestation_weeks, birth_weight_grams, apgar_1min, apgar_5min,
		          phototherapy_active, feeding_method, tenant_id, admitted_at, discharged_at, created_at, updated_at
	`, pgx.NamedArgs{
		"status":              req.Status,
		"phototherapy_active": req.PhototherapyActive,
		"feeding_method":      req.FeedingMethod,
		"id":                  id,
		"tenant_id":           tenantID,
	}).Scan(
		&a.ID, &a.EpisodeID, &a.PatientNHI, &a.NeonatologistHpi, &a.Status, &a.BedLabel,
		&a.AdmissionReason, &a.GestationWeeks, &a.BirthWeightGrams, &a.Apgar1min, &a.Apgar5min,
		&a.PhototherapyActive, &a.FeedingMethod, &a.TenantID, &a.AdmittedAt, &a.DischargedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "SCBU admission not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "update", "SCBUAdmission", a.ID, a.PatientNHI)
	nhi, err := h.decryptNHI(a.PatientNHI)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DECRYPT_ERROR", Message: "failed to decrypt patient NHI"})
		return
	}
	a.PatientNHI = nhi
	writeJSON(w, http.StatusOK, a)
}

func (h *scbuHandler) Discharge(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE scbu_admissions
		SET status = 'discharged', discharged_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status != 'discharged'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "SCBU admission not found or already discharged"})
		return
	}
	h.recordAudit(r, "delete", "SCBUAdmission", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "discharged"})
}

func (h *scbuHandler) TransferNICU(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	tag, err := h.pool.Exec(r.Context(), `
		UPDATE scbu_admissions
		SET status = 'transferred-nicu', discharged_at = now(), updated_at = now()
		WHERE id = @id AND tenant_id = @tenant_id AND status = 'admitted'
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "SCBU admission not found or not eligible for NICU transfer"})
		return
	}
	h.recordAudit(r, "update", "SCBUAdmission", id, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "transferred-nicu"})
}

func (h *scbuHandler) ListChart(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	rows, err := h.pool.Query(r.Context(), `
		SELECT id, scbu_admission_id, nurse_hpi, weight_grams, bilirubin_umol,
		       feed_volume_ml, notes, tenant_id, recorded_at
		FROM scbu_chart_entries
		WHERE scbu_admission_id = @id AND tenant_id = @tenant_id
		ORDER BY recorded_at DESC
	`, pgx.NamedArgs{"id": id, "tenant_id": tenantID})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	defer rows.Close()
	entries := make([]SCBUChartEntry, 0)
	for rows.Next() {
		var e SCBUChartEntry
		if err := rows.Scan(
			&e.ID, &e.ScbuAdmissionID, &e.NurseHpi, &e.WeightGrams, &e.BilirubinUmol,
			&e.FeedVolumeMl, &e.Notes, &e.TenantID, &e.RecordedAt,
		); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Code: "SCAN_ERROR", Message: err.Error()})
			return
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ROWS_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *scbuHandler) AddChartEntry(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	id := r.PathValue("id")
	var req SCBUChartEntry
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if !h.validateHPI(w, r, req.NurseHpi) {
		return
	}
	var e SCBUChartEntry
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO scbu_chart_entries
		    (scbu_admission_id, nurse_hpi, weight_grams, bilirubin_umol, feed_volume_ml, notes, tenant_id)
		VALUES
		    (@scbu_admission_id, @nurse_hpi, @weight_grams, @bilirubin_umol, @feed_volume_ml, @notes, @tenant_id)
		RETURNING id, scbu_admission_id, nurse_hpi, weight_grams, bilirubin_umol,
		          feed_volume_ml, notes, tenant_id, recorded_at
	`, pgx.NamedArgs{
		"scbu_admission_id": id,
		"nurse_hpi":         req.NurseHpi,
		"weight_grams":      req.WeightGrams,
		"bilirubin_umol":    req.BilirubinUmol,
		"feed_volume_ml":    req.FeedVolumeMl,
		"notes":             req.Notes,
		"tenant_id":         tenantID,
	}).Scan(
		&e.ID, &e.ScbuAdmissionID, &e.NurseHpi, &e.WeightGrams, &e.BilirubinUmol,
		&e.FeedVolumeMl, &e.Notes, &e.TenantID, &e.RecordedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	h.recordAudit(r, "create", "SCBUChartEntry", e.ID, "")
	writeJSON(w, http.StatusCreated, e)
}
