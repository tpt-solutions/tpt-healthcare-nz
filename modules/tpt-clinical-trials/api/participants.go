package api

import "net/http"

// ParticipantStatus reflects the current status of a trial participant.
type ParticipantStatus string

const (
	ParticipantStatusScreened        ParticipantStatus = "screened"
	ParticipantStatusScreenFailed    ParticipantStatus = "screen-failed"
	ParticipantStatusEnrolled        ParticipantStatus = "enrolled"
	ParticipantStatusRandomised      ParticipantStatus = "randomised"
	ParticipantStatusActive          ParticipantStatus = "active"
	ParticipantStatusCompleted       ParticipantStatus = "completed"
	ParticipantStatusWithdrawn       ParticipantStatus = "withdrawn"
	ParticipantStatusLostToFollowUp  ParticipantStatus = "lost-to-follow-up"
	ParticipantStatusDeceased        ParticipantStatus = "deceased"
)

// WithdrawalReason documents why a participant left the trial.
type WithdrawalReason string

const (
	WithdrawalReasonPatientRequest       WithdrawalReason = "patient-request"
	WithdrawalReasonAdverseEvent         WithdrawalReason = "adverse-event"
	WithdrawalReasonDiseaseProgression   WithdrawalReason = "disease-progression"
	WithdrawalReasonProtocolViolation    WithdrawalReason = "protocol-violation"
	WithdrawalReasonInvestigatorDecision WithdrawalReason = "investigator-decision"
	WithdrawalReasonSponsorDecision      WithdrawalReason = "sponsor-decision"
	WithdrawalReasonDeceased             WithdrawalReason = "deceased"
	WithdrawalReasonOther                WithdrawalReason = "other"
)

// ConsentStatus tracks the state of the participant's informed consent.
type ConsentStatus string

const (
	ConsentStatusPending    ConsentStatus = "pending"
	ConsentStatusObtained   ConsentStatus = "obtained"
	ConsentStatusReconsented ConsentStatus = "reconsented" // after amendment
	ConsentStatusWithdrawn  ConsentStatus = "withdrawn"
)

// RandomisationMethod describes the algorithm used to allocate participants.
type RandomisationMethod string

const (
	RandomisationSimple        RandomisationMethod = "simple"
	RandomisationBlock         RandomisationMethod = "block"
	RandomisationStratified    RandomisationMethod = "stratified"
	RandomisationMinimisation  RandomisationMethod = "minimisation"
	RandomisationAdaptive      RandomisationMethod = "adaptive"
)

// ScreenFailReason records why a candidate did not meet eligibility criteria.
type ScreenFailReason string

const (
	ScreenFailInclusionNotMet   ScreenFailReason = "inclusion-criteria-not-met"
	ScreenFailExclusionMet      ScreenFailReason = "exclusion-criteria-met"
	ScreenFailPatientDeclined   ScreenFailReason = "patient-declined"
	ScreenFailInvestigatorDecision ScreenFailReason = "investigator-decision"
	ScreenFailOther             ScreenFailReason = "other"
)

// participantHandler manages participant screening, enrolment, randomisation,
// consent documentation, withdrawal, and completion.
// All participant NHI values are stored deterministically encrypted per HIPC Rule 12.
type participantHandler struct {
	handlerDeps
}

func (h *participantHandler) List(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *participantHandler) Screen(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *participantHandler) Get(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *participantHandler) Enrol(w http.ResponseWriter, r *http.Request)        { notImplemented(w, r) }
func (h *participantHandler) Randomise(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
func (h *participantHandler) Withdraw(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *participantHandler) Complete(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *participantHandler) GetConsent(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *participantHandler) UpdateConsent(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *participantHandler) ScreeningLog(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
