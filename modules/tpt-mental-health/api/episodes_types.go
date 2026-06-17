package api

import "time"

// EpisodeType classifies the setting of a mental health episode.
type EpisodeType string

const (
	EpisodeInpatient    EpisodeType = "inpatient"
	EpisodeCommunity    EpisodeType = "community"
	EpisodeCrisis       EpisodeType = "crisis"
	EpisodeDayProgramme EpisodeType = "day-programme"
)

// EpisodeStatus tracks the lifecycle of an episode.
type EpisodeStatus string

const (
	EpisodeActive      EpisodeStatus = "active"
	EpisodeOnHold      EpisodeStatus = "on-hold"
	EpisodeCompleted   EpisodeStatus = "completed"
	EpisodeTransferred EpisodeStatus = "transferred"
	EpisodeDeceased    EpisodeStatus = "deceased"
)

// RiskLevel rates the patient's current risk of harm.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskVeryHigh RiskLevel = "very-high"
)

// Episode represents a mental health episode of care.
type Episode struct {
	ID                 string        `json:"id"`
	PatientID          string        `json:"patientId"`
	PatientNHI         string        `json:"patientNhi"`
	TenantID           string        `json:"tenantId"`
	ResponsibleHPI     string        `json:"responsibleHpi"`
	EpisodeType        EpisodeType   `json:"episodeType"`
	Status             EpisodeStatus `json:"status"`
	AdmissionReason    string        `json:"admissionReason,omitempty"` // decrypted
	PrimaryDiagnosis   string        `json:"primaryDiagnosis"`
	SecondaryDiagnoses []string      `json:"secondaryDiagnoses"`
	WardOrTeam         string        `json:"wardOrTeam"`
	BedNumber          string        `json:"bedNumber,omitempty"`
	AdmittedAt         *time.Time    `json:"admittedAt,omitempty"`
	DischargedAt       *time.Time    `json:"dischargedAt,omitempty"`
	ExtraSensitive     bool          `json:"extraSensitive"`
	CreatedAt          time.Time     `json:"createdAt"`
	UpdatedAt          time.Time     `json:"updatedAt"`
}

// WardRound represents a clinical contact entry within an inpatient episode.
type WardRound struct {
	ID             string         `json:"id"`
	EpisodeID      string         `json:"episodeId"`
	PatientID      string         `json:"patientId"`
	PatientNHI     string         `json:"patientNhi"`
	TenantID       string         `json:"tenantId"`
	ClinicianHPI   string         `json:"clinicianHpi"`
	Notes          string         `json:"notes,omitempty"` // decrypted
	MentalState    map[string]any `json:"mentalState"`
	RiskLevel      RiskLevel      `json:"riskLevel"`
	Plans          string         `json:"plans,omitempty"` // decrypted
	ExtraSensitive bool           `json:"extraSensitive"`
	OccurredAt     time.Time      `json:"occurredAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// episodeCreateRequest is the body for POST /api/v1/episodes.
type episodeCreateRequest struct {
	PatientID          string      `json:"patientId"`
	PatientNHI         string      `json:"patientNhi"`
	ResponsibleHPI     string      `json:"responsibleHpi"`
	EpisodeType        EpisodeType `json:"episodeType"`
	AdmissionReason    string      `json:"admissionReason"`
	PrimaryDiagnosis   string      `json:"primaryDiagnosis,omitempty"`
	SecondaryDiagnoses []string    `json:"secondaryDiagnoses,omitempty"`
	WardOrTeam         string      `json:"wardOrTeam,omitempty"`
	BedNumber          string      `json:"bedNumber,omitempty"`
	AdmittedAt         *time.Time  `json:"admittedAt,omitempty"`
}

// episodeUpdateRequest is the body for PUT /api/v1/episodes/{id}.
type episodeUpdateRequest struct {
	ResponsibleHPI     string        `json:"responsibleHpi,omitempty"`
	Status             EpisodeStatus `json:"status,omitempty"`
	PrimaryDiagnosis   string        `json:"primaryDiagnosis,omitempty"`
	SecondaryDiagnoses []string      `json:"secondaryDiagnoses,omitempty"`
	WardOrTeam         string        `json:"wardOrTeam,omitempty"`
	BedNumber          string        `json:"bedNumber,omitempty"`
}

// dischargeRequest is the body for POST /api/v1/episodes/{id}/discharge.
type dischargeRequest struct {
	DischargedAt     time.Time     `json:"dischargedAt"`
	DischargeSummary string        `json:"dischargeSummary,omitempty"`
	Status           EpisodeStatus `json:"status,omitempty"` // defaults to "completed"
}

// wardRoundCreateRequest is the body for POST /api/v1/episodes/{id}/ward-rounds.
type wardRoundCreateRequest struct {
	ClinicianHPI string         `json:"clinicianHpi"`
	Notes        string         `json:"notes,omitempty"`
	MentalState  map[string]any `json:"mentalState,omitempty"`
	RiskLevel    RiskLevel      `json:"riskLevel,omitempty"`
	Plans        string         `json:"plans,omitempty"`
	OccurredAt   *time.Time     `json:"occurredAt,omitempty"`
}
