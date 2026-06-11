package api

import "net/http"

// AEGrade is the CTCAE v5.0 toxicity grade (1–5).
type AEGrade int

const (
	AEGrade1 AEGrade = 1 // mild
	AEGrade2 AEGrade = 2 // moderate
	AEGrade3 AEGrade = 3 // severe
	AEGrade4 AEGrade = 4 // life-threatening
	AEGrade5 AEGrade = 5 // death
)

// AEStatus tracks the lifecycle of a reported adverse event.
type AEStatus string

const (
	AEStatusOngoing   AEStatus = "ongoing"
	AEStatusResolved  AEStatus = "resolved"
	AEStatusResolvedWithSequalae AEStatus = "resolved-with-sequelae"
	AEStatusFatal     AEStatus = "fatal"
	AEStatusUnknown   AEStatus = "unknown"
)

// AECausality reflects the investigator's assessment of relationship to study treatment.
type AECausality string

const (
	AECausalityUnrelated     AECausality = "unrelated"
	AECausalityUnlikely      AECausality = "unlikely"
	AECausalityPossible      AECausality = "possible"
	AECausalityProbable      AECausality = "probable"
	AECausalityDefinite      AECausality = "definite"
	AECausalityNotAssessable AECausality = "not-assessable"
)

// SAECategory captures which SAE criterion applies under ICH E2A.
type SAECategory string

const (
	SAECategoryDeath               SAECategory = "death"
	SAECategoryLifeThreatening     SAECategory = "life-threatening"
	SAECategoryHospitalisation     SAECategory = "hospitalisation"
	SAECategoryProlongedHospital   SAECategory = "prolonged-hospitalisation"
	SAECategoryPersistentDisability SAECategory = "persistent-disability"
	SAECategoryCongenitalAnomaly   SAECategory = "congenital-anomaly"
	SAECategoryMedicallyImportant  SAECategory = "medically-important"
)

// SUSARExpectedness indicates whether the reaction was listed in the IB/SmPC.
type SUSARExpectedness string

const (
	SUSARExpected   SUSARExpectedness = "expected"
	SUSARUnexpected SUSARExpectedness = "unexpected"
)

// RegulatoryReportStatus tracks submission of an SAE/SUSAR to Medsafe.
type RegulatoryReportStatus string

const (
	RegulatoryReportStatusPending    RegulatoryReportStatus = "pending"
	RegulatoryReportStatusDue        RegulatoryReportStatus = "due"        // 7-day or 15-day clock running
	RegulatoryReportStatusSubmitted  RegulatoryReportStatus = "submitted"
	RegulatoryReportStatusAcknowledged RegulatoryReportStatus = "acknowledged"
	RegulatoryReportStatusFollowUpDue RegulatoryReportStatus = "follow-up-due"
	RegulatoryReportStatusClosed     RegulatoryReportStatus = "closed"
)

// adverseEventHandler manages adverse event recording, SAE classification, and
// SUSAR reporting to Medsafe under the Medicines Act 1981.
// Fatal and life-threatening events require a 7-day expedited report;
// all other SAEs require a 15-day report per ICH E2A guidelines.
type adverseEventHandler struct {
	handlerDeps
}

func (h *adverseEventHandler) List(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *adverseEventHandler) Create(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *adverseEventHandler) Get(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *adverseEventHandler) Update(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *adverseEventHandler) Resolve(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *adverseEventHandler) GetSAE(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *adverseEventHandler) ReportSAE(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
func (h *adverseEventHandler) UpdateSAE(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
func (h *adverseEventHandler) ReportSUSAR(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *adverseEventHandler) SafetyReport(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
