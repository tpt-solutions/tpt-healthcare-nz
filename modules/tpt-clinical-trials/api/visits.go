package api

import "net/http"

// VisitStatus reflects the completion state of a participant's study visit.
type VisitStatus string

const (
	VisitStatusScheduled  VisitStatus = "scheduled"
	VisitStatusInProgress VisitStatus = "in-progress"
	VisitStatusCompleted  VisitStatus = "completed"
	VisitStatusMissed     VisitStatus = "missed"
	VisitStatusCancelled  VisitStatus = "cancelled"
)

// DeviationSeverity categorises the impact of a protocol deviation.
type DeviationSeverity string

const (
	DeviationSeverityMinor    DeviationSeverity = "minor"
	DeviationSeverityMajor    DeviationSeverity = "major"
	DeviationSeverityCritical DeviationSeverity = "critical"
)

// DeviationCategory classifies the type of protocol deviation.
type DeviationCategory string

const (
	DeviationCategoryEligibility     DeviationCategory = "eligibility"
	DeviationCategoryConsent         DeviationCategory = "consent"
	DeviationCategoryVisitWindow     DeviationCategory = "visit-window"
	DeviationCategoryProcedure       DeviationCategory = "procedure"
	DeviationCategoryDosing          DeviationCategory = "dosing"
	DeviationCategoryConcomitant     DeviationCategory = "concomitant-medication"
	DeviationCategoryDataCollection  DeviationCategory = "data-collection"
	DeviationCategoryOther           DeviationCategory = "other"
)

// CRFFieldType describes the data type of a case report form field.
type CRFFieldType string

const (
	CRFFieldTypeText     CRFFieldType = "text"
	CRFFieldTypeNumber   CRFFieldType = "number"
	CRFFieldTypeDate     CRFFieldType = "date"
	CRFFieldTypeBoolean  CRFFieldType = "boolean"
	CRFFieldTypeEnum     CRFFieldType = "enum"
	CRFFieldTypeScale    CRFFieldType = "scale"
	CRFFieldTypeLabValue CRFFieldType = "lab-value"
)

// visitHandler manages scheduled study visits, CRF data entry, and protocol
// deviation recording. Visit windows are validated against the protocol schedule;
// visits outside the allowed window trigger an automatic deviation record.
type visitHandler struct {
	handlerDeps
}

func (h *visitHandler) List(w http.ResponseWriter, r *http.Request)            { notImplemented(w, r) }
func (h *visitHandler) Create(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *visitHandler) Get(w http.ResponseWriter, r *http.Request)             { notImplemented(w, r) }
func (h *visitHandler) Update(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *visitHandler) Complete(w http.ResponseWriter, r *http.Request)        { notImplemented(w, r) }
func (h *visitHandler) GetCRF(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *visitHandler) SaveCRF(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *visitHandler) ListDeviations(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *visitHandler) RecordDeviation(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
