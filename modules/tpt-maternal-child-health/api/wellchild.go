package api

import "net/http"

// WellChildCheckType identifies the scheduled Well Child Tamariki Ora contact.
// The schedule is defined by Te Whatu Ora under the Well Child Tamariki Ora
// Framework; contacts span birth to ~5 years.
type WellChildCheckType string

const (
	WellChildNeonatal    WellChildCheckType = "neonatal"   // LMC handover check (~4–5 days)
	WellChildCheck6wk    WellChildCheckType = "6wk"        // 4–6 week GP check
	WellChildCheck3mo    WellChildCheckType = "3mo"
	WellChildCheck5mo    WellChildCheckType = "5mo"
	WellChildCheck9mo    WellChildCheckType = "9mo"
	WellChildCheck12mo   WellChildCheckType = "12mo"
	WellChildCheck15mo   WellChildCheckType = "15mo"
	WellChildCheck2yr    WellChildCheckType = "2yr"
	WellChildB4School    WellChildCheckType = "B4SchoolCheck" // ~age 4, before school entry
)

// SDQBand classifies the Strengths and Difficulties Questionnaire total score.
type SDQBand string

const (
	SDQBandNormal     SDQBand = "normal"
	SDQBandBorderline SDQBand = "borderline"
	SDQBandAbnormal   SDQBand = "abnormal"
)

// WellChildCheckStatus tracks whether the check has been completed.
type WellChildCheckStatus string

const (
	WellChildStatusScheduled WellChildCheckStatus = "scheduled"
	WellChildStatusCompleted WellChildCheckStatus = "completed"
	WellChildStatusMissed    WellChildCheckStatus = "missed"
	WellChildStatusDeclined  WellChildCheckStatus = "declined"
)

// wellChildHandler manages Well Child Tamariki Ora checks and growth monitoring.
// Growth points store raw measurements (weight, height, head circumference);
// centile band calculation uses WHO growth standards and is performed client-side.
type wellChildHandler struct {
	handlerDeps
}

func (h *wellChildHandler) List(w http.ResponseWriter, r *http.Request)             { notImplemented(w, r) }
func (h *wellChildHandler) Create(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *wellChildHandler) Get(w http.ResponseWriter, r *http.Request)              { notImplemented(w, r) }
func (h *wellChildHandler) Update(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *wellChildHandler) ListGrowthPoints(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *wellChildHandler) RecordGrowthPoint(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
