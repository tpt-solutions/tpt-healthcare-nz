package api

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// NBRSNotificationStatus tracks the NBRS birth notification submission lifecycle.
// Every birth in NZ must be notified to NBRS within 5 working days under the
// Births, Deaths, Marriages, and Relationships Registration Act 2021.
type NBRSNotificationStatus string

const (
	NBRSStatusPending   NBRSNotificationStatus = "pending"
	NBRSStatusSubmitted NBRSNotificationStatus = "submitted"
	NBRSStatusAccepted  NBRSNotificationStatus = "accepted"
	NBRSStatusRejected  NBRSNotificationStatus = "rejected"
	NBRSStatusError     NBRSNotificationStatus = "error"
)

type NBRSNotification struct {
	ID                    string     `json:"id"`
	MaternityEpisodeID    string     `json:"maternityEpisodeId"`
	IntrapartumEpisodeID  *string    `json:"intrapartumEpisodeId"`
	NhiMother             string     `json:"nhiMother"`
	NhiBaby               string     `json:"nhiBaby"`
	BabyFirstName         string     `json:"babyFirstName"`
	BabyFamilyName        string     `json:"babyFamilyName"`
	BirthDate             *string    `json:"birthDate"`
	BirthTime             *string    `json:"birthTime"`
	Sex                   string     `json:"sex"`
	BirthWeightGrams      *int       `json:"birthWeightGrams"`
	GestationWeeks        *int16     `json:"gestationWeeks"`
	BirthOrder            int16      `json:"birthOrder"`
	Plurality             string     `json:"plurality"`
	FatherName            *string    `json:"fatherName"`
	Ethnicities           []string   `json:"ethnicities"`
	BirthFacilityHpi      string     `json:"birthFacilityHpi"`
	AttendingClinicianHpi string     `json:"attendingClinicianHpi"`
	NotificationStatus    string     `json:"notificationStatus"`
	SubmittedAt           *time.Time `json:"submittedAt"`
	ResponseCode          *string    `json:"responseCode"`
	ResponseMessage       *string    `json:"responseMessage"`
	TenantID              string     `json:"tenantId"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

// nbrsHandler manages NBRS birth notifications.
type nbrsHandler struct {
	handlerDeps
}

// Get returns the NBRS notification record for a maternity episode,
// including current submission status and any response from NBRS.
func (h *nbrsHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var n NBRSNotification
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, maternity_episode_id, intrapartum_episode_id,
		       nhi_mother, nhi_baby, baby_first_name, baby_family_name,
		       birth_date, birth_time::text, sex, birth_weight_grams, gestation_weeks,
		       birth_order, plurality, father_name,
		       COALESCE(ethnicities::text[],'{}'), birth_facility_hpi, attending_clinician_hpi,
		       notification_status, submitted_at, response_code, response_message,
		       tenant_id, created_at, updated_at
		FROM nbrs_notifications
		WHERE maternity_episode_id = @episode_id AND tenant_id = @tenant_id
		ORDER BY created_at DESC
		LIMIT 1
	`, pgx.NamedArgs{"episode_id": episodeID, "tenant_id": tenantID}).Scan(
		&n.ID, &n.MaternityEpisodeID, &n.IntrapartumEpisodeID,
		&n.NhiMother, &n.NhiBaby, &n.BabyFirstName, &n.BabyFamilyName,
		&n.BirthDate, &n.BirthTime, &n.Sex, &n.BirthWeightGrams, &n.GestationWeeks,
		&n.BirthOrder, &n.Plurality, &n.FatherName,
		&n.Ethnicities, &n.BirthFacilityHpi, &n.AttendingClinicianHpi,
		&n.NotificationStatus, &n.SubmittedAt, &n.ResponseCode, &n.ResponseMessage,
		&n.TenantID, &n.CreatedAt, &n.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "no NBRS notification found for this episode"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// Submit creates or resubmits the NBRS birth notification for the episode.
func (h *nbrsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := middleware.TenantFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "NO_TENANT", Message: "tenant not found in context"})
		return
	}
	episodeID := r.PathValue("id")
	var req NBRSNotification
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.NhiBaby == "" || req.BirthDate == nil || req.BirthWeightGrams == nil ||
		req.GestationWeeks == nil || req.AttendingClinicianHpi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code:    "MISSING_REQUIRED_FIELDS",
			Message: "nhiBaby, birthDate, birthWeightGrams, gestationWeeks, and attendingClinicianHpi are required for NBRS submission",
		})
		return
	}
	if req.Sex == "" {
		req.Sex = "unknown"
	}
	if req.Plurality == "" {
		req.Plurality = "single"
	}
	if req.BirthOrder == 0 {
		req.BirthOrder = 1
	}

	var n NBRSNotification
	err := h.pool.QueryRow(r.Context(), `
		INSERT INTO nbrs_notifications
		    (maternity_episode_id, intrapartum_episode_id,
		     nhi_mother, nhi_baby, baby_first_name, baby_family_name,
		     birth_date, sex, birth_weight_grams, gestation_weeks,
		     birth_order, plurality, father_name,
		     birth_facility_hpi, attending_clinician_hpi,
		     notification_status, submitted_at, tenant_id)
		VALUES
		    (@episode_id, @intrapartum_episode_id,
		     @nhi_mother, @nhi_baby, @baby_first_name, @baby_family_name,
		     @birth_date, @sex, @birth_weight_grams, @gestation_weeks,
		     @birth_order, @plurality, @father_name,
		     @birth_facility_hpi, @attending_clinician_hpi,
		     'submitted', now(), @tenant_id)
		ON CONFLICT DO NOTHING
		RETURNING id, maternity_episode_id, intrapartum_episode_id,
		          nhi_mother, nhi_baby, baby_first_name, baby_family_name,
		          birth_date, birth_time::text, sex, birth_weight_grams, gestation_weeks,
		          birth_order, plurality, father_name,
		          COALESCE(ethnicities::text[],'{}'), birth_facility_hpi, attending_clinician_hpi,
		          notification_status, submitted_at, response_code, response_message,
		          tenant_id, created_at, updated_at
	`, pgx.NamedArgs{
		"episode_id":              episodeID,
		"intrapartum_episode_id":  req.IntrapartumEpisodeID,
		"nhi_mother":              req.NhiMother,
		"nhi_baby":                req.NhiBaby,
		"baby_first_name":         req.BabyFirstName,
		"baby_family_name":        req.BabyFamilyName,
		"birth_date":              req.BirthDate,
		"sex":                     req.Sex,
		"birth_weight_grams":      req.BirthWeightGrams,
		"gestation_weeks":         req.GestationWeeks,
		"birth_order":             req.BirthOrder,
		"plurality":               req.Plurality,
		"father_name":             req.FatherName,
		"birth_facility_hpi":      req.BirthFacilityHpi,
		"attending_clinician_hpi": req.AttendingClinicianHpi,
		"tenant_id":               tenantID,
	}).Scan(
		&n.ID, &n.MaternityEpisodeID, &n.IntrapartumEpisodeID,
		&n.NhiMother, &n.NhiBaby, &n.BabyFirstName, &n.BabyFamilyName,
		&n.BirthDate, &n.BirthTime, &n.Sex, &n.BirthWeightGrams, &n.GestationWeeks,
		&n.BirthOrder, &n.Plurality, &n.FatherName,
		&n.Ethnicities, &n.BirthFacilityHpi, &n.AttendingClinicianHpi,
		&n.NotificationStatus, &n.SubmittedAt, &n.ResponseCode, &n.ResponseMessage,
		&n.TenantID, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "DB_ERROR", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, n)
}
