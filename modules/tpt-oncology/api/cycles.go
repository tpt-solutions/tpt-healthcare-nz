package api

import "net/http"

// CycleStatus reflects the current state of a chemotherapy treatment cycle.
type CycleStatus string

const (
	CycleStatusScheduled   CycleStatus = "scheduled"
	CycleStatusInProgress  CycleStatus = "in-progress"
	CycleStatusCompleted   CycleStatus = "completed"
	CycleStatusDelayed     CycleStatus = "delayed"
	CycleStatusOmitted     CycleStatus = "omitted"
	CycleStatusDoseReduced CycleStatus = "dose-reduced"
)

// AdministrationStatus tracks whether a drug was given as planned.
type AdministrationStatus string

const (
	AdministrationStatusGiven       AdministrationStatus = "given"
	AdministrationStatusPartialGiven AdministrationStatus = "partial-given"
	AdministrationStatusOmitted     AdministrationStatus = "omitted"
	AdministrationStatusDelayed     AdministrationStatus = "delayed"
	AdministrationStatusReaction    AdministrationStatus = "reaction"
)

// DoseModificationReason is the documented reason for any dose change.
type DoseModificationReason string

const (
	DoseModReasonToxicity       DoseModificationReason = "toxicity"
	DoseModReasonRenalImpairment DoseModificationReason = "renal-impairment"
	DoseModReasonHepaticImpairment DoseModificationReason = "hepatic-impairment"
	DoseModReasonWeightChange   DoseModificationReason = "weight-change"
	DoseModReasonPatientRequest DoseModificationReason = "patient-request"
	DoseModReasonClinicalDecision DoseModificationReason = "clinical-decision"
)

// cycleHandler manages treatment cycle scheduling and drug administration records.
// Each cycle belongs to a patient protocol assignment. Administration records capture
// actual drug delivery including dose, route, duration, and any reactions.
type cycleHandler struct {
	handlerDeps
}

func (h *cycleHandler) List(w http.ResponseWriter, r *http.Request)                { notImplemented(w, r) }
func (h *cycleHandler) Create(w http.ResponseWriter, r *http.Request)              { notImplemented(w, r) }
func (h *cycleHandler) Get(w http.ResponseWriter, r *http.Request)                 { notImplemented(w, r) }
func (h *cycleHandler) Update(w http.ResponseWriter, r *http.Request)              { notImplemented(w, r) }
func (h *cycleHandler) Complete(w http.ResponseWriter, r *http.Request)            { notImplemented(w, r) }
func (h *cycleHandler) ListAdministrations(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *cycleHandler) RecordAdministration(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
