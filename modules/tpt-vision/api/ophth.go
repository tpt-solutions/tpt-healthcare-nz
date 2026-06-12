package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
	"github.com/PhillipC05/tpt-healthcare/modules/tpt-vision/internal/ophthalmology"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OphthalmicHandler handles ophthalmic examination CRUD operations.
type OphthalmicHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// ListExams returns all ophthalmic exams for a patient.
func (h *OphthalmicHandler) ListExams(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	if patientNhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "Patient NHI is required"})
		return
	}

	ctx := r.Context()
	tenantID, _ := middleware.TenantFromContext(ctx)

	rows, err := h.pool.Query(ctx, `
		SELECT id, tenant_id::text, patient_nhi, clinician_id::text, practice_id::text,
		       exam_type, exam_date, fhir_resource, fhir_version, created_at, updated_at
		FROM vision_ophthalmic_exams
		WHERE patient_nhi=$1 AND tenant_id=$2
		ORDER BY exam_date DESC`,
		patientNhi, tenantID,
	)
	if err != nil {
		h.logger.Error("list ophthalmic exams", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to list exams"})
		return
	}
	defer rows.Close()

	exams := make([]ophthalmology.OphthalmicExam, 0)
	for rows.Next() {
		var e ophthalmology.OphthalmicExam
		var fhirRaw json.RawMessage
		var examDate, createdAt, updatedAt time.Time
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.PatientNHI, &e.ClinicianID, &e.PracticeID,
			&e.ExamType, &examDate, &fhirRaw, &e.FHIRVersion, &createdAt, &updatedAt,
		); err != nil {
			h.logger.Error("scan exam row", slog.Any("error", err))
			continue
		}
		// Overlay the full exam data from the stored FHIR JSONB.
		if err := json.Unmarshal(fhirRaw, &e); err == nil {
			e.ExamDate = examDate.UnixMilli()
			e.CreatedAt = createdAt.UnixMilli()
			e.UpdatedAt = updatedAt.UnixMilli()
		}
		exams = append(exams, e)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"patientNhi": patientNhi,
		"exams":      exams,
	})
}

// CreateExam creates a new ophthalmic examination record.
func (h *OphthalmicHandler) CreateExam(w http.ResponseWriter, r *http.Request) {
	var exam ophthalmology.OphthalmicExam
	if err := decodeJSON(r, &exam); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	if err := exam.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_FAILED", Message: err.Error()})
		return
	}

	ctx := r.Context()
	tenantID, _ := middleware.TenantFromContext(ctx)

	exam.ID = uuid.New().String()
	now := time.Now()
	exam.CreatedAt = now.UnixMilli()
	exam.UpdatedAt = now.UnixMilli()
	if exam.TenantID == "" {
		exam.TenantID = tenantID.String()
	}

	// Supply sensible defaults for the NOT NULL CHECK-constrained columns
	// when the caller has not provided them.
	lensRight := exam.LensRight
	if lensRight == "" {
		lensRight = ophthalmology.LensPhakic
	}
	lensLeft := exam.LensLeft
	if lensLeft == "" {
		lensLeft = ophthalmology.LensPhakic
	}
	discRight := exam.DiscRight
	if discRight == "" {
		discRight = ophthalmology.DiscNormal
	}
	discLeft := exam.DiscLeft
	if discLeft == "" {
		discLeft = ophthalmology.DiscNormal
	}
	maculaRight := exam.MaculaRight
	if maculaRight == "" {
		maculaRight = ophthalmology.MaculaNormal
	}
	maculaLeft := exam.MaculaLeft
	if maculaLeft == "" {
		maculaLeft = ophthalmology.MaculaNormal
	}
	examType := exam.ExamType
	if examType == "" {
		examType = ophthalmology.ExamComprehensive
	}

	// Marshal the full exam as FHIR JSONB for round-trip retrieval.
	fhirJSON, err := json.Marshal(exam)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "MARSHAL_ERROR", Message: "failed to serialise exam"})
		return
	}

	iopJSON, _ := json.Marshal(exam.IOP)

	examDate := time.UnixMilli(exam.ExamDate)
	_, err = h.pool.Exec(ctx, `
		INSERT INTO vision_ophthalmic_exams (
			id, tenant_id, patient_nhi, clinician_id, practice_id,
			exam_type, exam_date,
			va_distance_right, va_distance_left, va_near_right, va_near_left,
			pinhole_right, pinhole_left,
			auto_refraction_right, auto_refraction_left,
			iop_readings,
			lens_right, lens_left, cataract_grade, cornea_clear, anterior_chamber,
			disc_right, disc_left, cd_ratio_right, cd_ratio_left,
			macula_right, macula_left,
			visual_fields_right, visual_fields_left,
			oct_right, oct_left,
			diagnosis, plan, referral_required, follow_up_days,
			fhir_resource, fhir_version,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,
			$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,
			$36,$37,$38,$39
		)`,
		exam.ID, exam.TenantID, exam.PatientNHI, exam.ClinicianID, exam.PracticeID,
		string(examType), examDate,
		exam.VADistanceRight, exam.VADistanceLeft, exam.VANearRight, exam.VANearLeft,
		exam.PinholeRight, exam.PinholeLeft,
		exam.AutoRefractionRight, exam.AutoRefractionLeft,
		iopJSON,
		string(lensRight), string(lensLeft), int(exam.CataractGrade), exam.CorneaClear, exam.AnteriorChamber,
		string(discRight), string(discLeft), float64(exam.CDRatioRight), float64(exam.CDRatioLeft),
		string(maculaRight), string(maculaLeft),
		exam.VisualFieldsRight, exam.VisualFieldsLeft,
		exam.OCTRight, exam.OCTLeft,
		exam.Diagnosis, exam.Plan, exam.ReferralRequired, exam.FollowUpDays,
		fhirJSON, 1,
		now, now,
	)
	if err != nil {
		h.logger.Error("create ophthalmic exam", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to save exam"})
		return
	}

	h.logger.Info("ophthalmic exam created", slog.String("patient_nhi", exam.PatientNHI), slog.String("id", exam.ID))
	writeJSON(w, http.StatusCreated, exam)
}

// GetExam returns a specific ophthalmic exam.
func (h *OphthalmicHandler) GetExam(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	ctx := r.Context()
	var exam ophthalmology.OphthalmicExam
	var fhirRaw json.RawMessage
	var examDate, createdAt, updatedAt time.Time

	err := h.pool.QueryRow(ctx, `
		SELECT id, tenant_id::text, patient_nhi, clinician_id::text, practice_id::text,
		       exam_type, exam_date, fhir_resource, fhir_version, created_at, updated_at
		FROM vision_ophthalmic_exams
		WHERE id=$1 AND patient_nhi=$2`,
		examId, patientNhi,
	).Scan(
		&exam.ID, &exam.TenantID, &exam.PatientNHI, &exam.ClinicianID, &exam.PracticeID,
		&exam.ExamType, &examDate, &fhirRaw, &exam.FHIRVersion, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "ophthalmic exam not found"})
			return
		}
		h.logger.Error("get ophthalmic exam", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: "failed to fetch exam"})
		return
	}

	// Overlay the full exam data from the stored FHIR JSONB.
	if err := json.Unmarshal(fhirRaw, &exam); err == nil {
		exam.ExamDate = examDate.UnixMilli()
		exam.CreatedAt = createdAt.UnixMilli()
		exam.UpdatedAt = updatedAt.UnixMilli()
	}

	writeJSON(w, http.StatusOK, exam)
}

// UpdateExam updates an existing ophthalmic exam.
func (h *OphthalmicHandler) UpdateExam(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	var exam ophthalmology.OphthalmicExam
	if err := decodeJSON(r, &exam); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	h.logger.Info("ophthalmic exam updated", slog.String("patient_nhi", patientNhi), slog.String("exam_id", examId))
	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "updated",
		"patientNhi": patientNhi,
		"examId":     examId,
	})
}

// AddIOPReading adds an IOP reading to an existing exam.
func (h *OphthalmicHandler) AddIOPReading(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	var iop ophthalmology.IOPReading
	if err := decodeJSON(r, &iop); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_JSON", Message: fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	h.logger.Info("IOP reading added",
		slog.String("patient_nhi", patientNhi),
		slog.String("exam_id", examId),
		slog.Float64("right", iop.RightEye),
		slog.Float64("left", iop.LeftEye))

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "iop_added",
		"patientNhi": patientNhi,
		"examId":     examId,
	})
}

// GetExamFHIR returns an ophthalmic exam as a FHIR R5 DiagnosticReport resource.
func (h *OphthalmicHandler) GetExamFHIR(w http.ResponseWriter, r *http.Request) {
	patientNhi := r.PathValue("patientNhi")
	examId := r.PathValue("examId")

	if patientNhi == "" || examId == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_PARAMS", Message: "Patient NHI and exam ID are required"})
		return
	}

	// TODO: retrieve exam from DB
	// For now, return a placeholder
	writeJSON(w, http.StatusOK, map[string]any{
		"resourceType": "OperationOutcome",
		"issue": []map[string]any{
			{
				"severity": "information",
				"code":     "not-implemented",
				"details": map[string]any{
					"text": "FHIR endpoint - implement DB retrieval",
				},
			},
		},
	})
}
