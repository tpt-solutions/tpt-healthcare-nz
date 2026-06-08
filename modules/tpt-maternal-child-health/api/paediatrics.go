package api

import "net/http"

// PaediatricAdmissionStatus tracks the inpatient lifecycle.
type PaediatricAdmissionStatus string

const (
	PaedAdmissionStatusAdmitted          PaediatricAdmissionStatus = "admitted"
	PaedAdmissionStatusStable            PaediatricAdmissionStatus = "stable"
	PaedAdmissionStatusDischargePlanning PaediatricAdmissionStatus = "discharge-planning"
	PaedAdmissionStatusDischarged        PaediatricAdmissionStatus = "discharged"
	PaedAdmissionStatusTransferred       PaediatricAdmissionStatus = "transferred"
)

// PaediatricAdmissionType classifies how the child came to be admitted.
type PaediatricAdmissionType string

const (
	PaedAdmissionElective PaediatricAdmissionType = "elective"
	PaedAdmissionAcute    PaediatricAdmissionType = "acute"
	PaedAdmissionTransfer PaediatricAdmissionType = "transfer"
)

// PICUStatus tracks the clinical status of a PICU admission.
// PICU covers children >28 days requiring intensive care; neonates are managed
// in the NICU under /api/v1/maternity/nicu.
type PICUStatus string

const (
	PICUStatusAdmitted  PICUStatus = "admitted"
	PICUStatusStable    PICUStatus = "stable"
	PICUStatusCritical  PICUStatus = "critical"
	PICUStatusDischarged PICUStatus = "discharged"
)

// DevelopmentalDomain classifies the developmental area being assessed.
type DevelopmentalDomain string

const (
	DevDomainGrossMotor     DevelopmentalDomain = "gross-motor"
	DevDomainFineMotor      DevelopmentalDomain = "fine-motor"
	DevDomainSpeechLanguage DevelopmentalDomain = "speech-language"
	DevDomainSocialEmotional DevelopmentalDomain = "social-emotional"
	DevDomainCognitive      DevelopmentalDomain = "cognitive"
)

// ChildProtectionStatus tracks the child protection concern lifecycle.
// Flagging and reporting must comply with the Children's Act 2014 (NZ).
type ChildProtectionStatus string

const (
	ChildProtectionNone             ChildProtectionStatus = "none"
	ChildProtectionConcernRaised    ChildProtectionStatus = "concern-raised"
	ChildProtectionNotified         ChildProtectionStatus = "notified"
	ChildProtectionUnderInvestigation ChildProtectionStatus = "under-investigation"
)

// paediatricHandler manages paediatric inpatient admissions, PICU,
// growth and developmental milestone tracking, and child protection flagging.
type paediatricHandler struct {
	handlerDeps
}

func (h *paediatricHandler) List(w http.ResponseWriter, r *http.Request)                { notImplemented(w, r) }
func (h *paediatricHandler) Admit(w http.ResponseWriter, r *http.Request)               { notImplemented(w, r) }
func (h *paediatricHandler) Get(w http.ResponseWriter, r *http.Request)                 { notImplemented(w, r) }
func (h *paediatricHandler) Update(w http.ResponseWriter, r *http.Request)              { notImplemented(w, r) }
func (h *paediatricHandler) Discharge(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *paediatricHandler) ListGrowth(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *paediatricHandler) RecordGrowth(w http.ResponseWriter, r *http.Request)        { notImplemented(w, r) }
func (h *paediatricHandler) ListMilestones(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *paediatricHandler) RecordMilestone(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *paediatricHandler) GetChildProtection(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *paediatricHandler) UpdateChildProtection(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *paediatricHandler) ListPICU(w http.ResponseWriter, r *http.Request)            { notImplemented(w, r) }
func (h *paediatricHandler) AdmitPICU(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *paediatricHandler) GetPICU(w http.ResponseWriter, r *http.Request)             { notImplemented(w, r) }
func (h *paediatricHandler) UpdatePICU(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *paediatricHandler) DischargePICU(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
