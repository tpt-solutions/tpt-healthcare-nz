package api

import "net/http"

// TrialPhase classifies the clinical development phase of the study.
type TrialPhase string

const (
	TrialPhaseI    TrialPhase = "I"
	TrialPhaseII   TrialPhase = "II"
	TrialPhaseIIa  TrialPhase = "IIa"
	TrialPhaseIIb  TrialPhase = "IIb"
	TrialPhaseIII  TrialPhase = "III"
	TrialPhaseIIIa TrialPhase = "IIIa"
	TrialPhaseIIIb TrialPhase = "IIIb"
	TrialPhaseIV   TrialPhase = "IV"
	TrialPhaseNA   TrialPhase = "N/A" // observational or device studies
)

// TrialType classifies the study design.
type TrialType string

const (
	TrialTypeInterventional TrialType = "interventional"
	TrialTypeObservational  TrialType = "observational"
	TrialTypeExpandedAccess TrialType = "expanded-access"
	TrialTypeRegistry       TrialType = "registry"
)

// InterventionType describes the nature of the experimental intervention.
type InterventionType string

const (
	InterventionTypeDrug        InterventionType = "drug"
	InterventionTypeBiological  InterventionType = "biological"
	InterventionTypeDevice      InterventionType = "device"
	InterventionTypeProcedure   InterventionType = "procedure"
	InterventionTypeRadiation   InterventionType = "radiation"
	InterventionTypeBehavioural InterventionType = "behavioural"
	InterventionTypeDiagnostic  InterventionType = "diagnostic"
	InterventionTypeOther       InterventionType = "other"
)

// BlindingType describes the masking arrangement for the study.
type BlindingType string

const (
	BlindingNone        BlindingType = "open-label"
	BlindingSingle      BlindingType = "single-blind"
	BlindingDouble      BlindingType = "double-blind"
	BlindingTriple      BlindingType = "triple-blind"
	BlindingQuadruple   BlindingType = "quadruple-blind"
)

// AllocationMethod describes how participants are assigned to arms.
type AllocationMethod string

const (
	AllocationRandomised    AllocationMethod = "randomised"
	AllocationNonRandomised AllocationMethod = "non-randomised"
	AllocationSingleArm     AllocationMethod = "single-arm"
)

// ProtocolStatus reflects the lifecycle state of the study protocol.
type ProtocolStatus string

const (
	ProtocolStatusDraft     ProtocolStatus = "draft"
	ProtocolStatusApproved  ProtocolStatus = "approved"  // HDEC approved, not yet recruiting
	ProtocolStatusActive    ProtocolStatus = "active"    // recruiting
	ProtocolStatusSuspended ProtocolStatus = "suspended"
	ProtocolStatusClosed    ProtocolStatus = "closed"    // recruitment closed, follow-up ongoing
	ProtocolStatusCompleted ProtocolStatus = "completed"
	ProtocolStatusWithdrawn ProtocolStatus = "withdrawn" // withdrawn before completion
)

// CriterionType distinguishes inclusion from exclusion eligibility criteria.
type CriterionType string

const (
	CriterionTypeInclusion CriterionType = "inclusion"
	CriterionTypeExclusion CriterionType = "exclusion"
)

// VisitType classifies the purpose of a scheduled protocol visit.
type VisitType string

const (
	VisitTypeScreening    VisitType = "screening"
	VisitTypeBaseline     VisitType = "baseline"
	VisitTypeTreatment    VisitType = "treatment"
	VisitTypeAssessment   VisitType = "assessment"
	VisitTypeFollowUp     VisitType = "follow-up"
	VisitTypeEndOfStudy   VisitType = "end-of-study"
	VisitTypeUnscheduled  VisitType = "unscheduled"
)

// protocolHandler manages the study protocol library including arms, eligibility
// criteria, visit schedules, and protocol amendments.
// HDEC approval number and ANZCTR registration are required before activation.
type protocolHandler struct {
	handlerDeps
}

func (h *protocolHandler) List(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *protocolHandler) Create(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *protocolHandler) Get(w http.ResponseWriter, r *http.Request)            { notImplemented(w, r) }
func (h *protocolHandler) Update(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *protocolHandler) Activate(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *protocolHandler) Close(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *protocolHandler) ListArms(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *protocolHandler) CreateArm(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *protocolHandler) GetEligibility(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *protocolHandler) UpdateEligibility(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *protocolHandler) GetSchedule(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
func (h *protocolHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *protocolHandler) ListAmendments(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *protocolHandler) CreateAmendment(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
