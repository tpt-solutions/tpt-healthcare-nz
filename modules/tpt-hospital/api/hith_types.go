package api

import "time"

// Vitals holds basic clinical observations recorded during a HITH visit.
type Vitals struct {
	SystolicBP  *float64 `json:"systolicBp,omitempty"`
	DiastolicBP *float64 `json:"diastolicBp,omitempty"`
	HeartRate   *float64 `json:"heartRate,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"` // °C
	SpO2        *float64 `json:"spo2,omitempty"`        // %
	RespRate    *float64 `json:"respRate,omitempty"`    // breaths/min
	Weight      *float64 `json:"weight,omitempty"`      // kg
}

// HITHEpisodeStatus tracks the patient's HITH care episode.
type HITHEpisodeStatus string

const (
	HITHStatusActive    HITHEpisodeStatus = "active"
	HITHStatusSuspended HITHEpisodeStatus = "suspended" // temporarily readmitted to hospital
	HITHStatusCompleted HITHEpisodeStatus = "completed"
	HITHStatusWithdrawn HITHEpisodeStatus = "withdrawn"
)

// HITHVisitType distinguishes the nature of the visit.
type HITHVisitType string

const (
	HITHVisitNursing    HITHVisitType = "nursing"
	HITHVisitMedical    HITHVisitType = "medical"
	HITHVisitPhysio     HITHVisitType = "physiotherapy"
	HITHVisitTelehealth HITHVisitType = "telehealth"
	HITHVisitPath       HITHVisitType = "pathology-collection"
)

// HITHEpisode is an active Hospital in the Home care episode.
type HITHEpisode struct {
	ID                string            `json:"id"`
	PatientID         string            `json:"patientId"`
	PatientNHI        string            `json:"patientNhi"`
	LinkedAdmissionID string            `json:"linkedAdmissionId,omitempty"` // original inpatient admission
	LeadClinicianHPI  string            `json:"leadClinicianHpi"`
	Status            HITHEpisodeStatus `json:"status"`
	Diagnosis         string            `json:"diagnosis"` // primary condition being treated at home
	CareGoals         []string          `json:"careGoals"`
	DailyVisitFreq    string            `json:"dailyVisitFrequency"` // e.g. "once", "twice", "bd"
	HomeAddress       string            `json:"homeAddress"`
	EmergencyContact  string            `json:"emergencyContact,omitempty"`
	PatientConsented  bool              `json:"patientConsented"`
	TenantID          string            `json:"tenantId"`
	StartDate         time.Time         `json:"startDate"`
	ExpectedEndDate   *time.Time        `json:"expectedEndDate,omitempty"`
	ActualEndDate     *time.Time        `json:"actualEndDate,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
}

// HITHVisit is a single nursing or medical visit during a HITH episode.
type HITHVisit struct {
	ID             string        `json:"id"`
	EpisodeID      string        `json:"episodeId"`
	CliniciandHPI  string        `json:"clinicianHpi"`
	VisitType      HITHVisitType `json:"visitType"`
	Vitals         Vitals        `json:"vitals"`
	ClinicalNotes  string        `json:"clinicalNotes,omitempty"`
	Escalated      bool          `json:"escalated"`
	EscalationNote string        `json:"escalationNote,omitempty"`
	NextVisitDate  *time.Time    `json:"nextVisitDate,omitempty"`
	TenantID       string        `json:"tenantId"`
	VisitedAt      time.Time     `json:"visitedAt"`
	CreatedAt      time.Time     `json:"createdAt"`
}

type hithEpisodeCreateRequest struct {
	PatientID         string     `json:"patientId"`
	PatientNHI        string     `json:"patientNhi"`
	LinkedAdmissionID string     `json:"linkedAdmissionId,omitempty"`
	LeadClinicianHPI  string     `json:"leadClinicianHpi"`
	Diagnosis         string     `json:"diagnosis"`
	CareGoals         []string   `json:"careGoals,omitempty"`
	DailyVisitFreq    string     `json:"dailyVisitFrequency"`
	HomeAddress       string     `json:"homeAddress"`
	EmergencyContact  string     `json:"emergencyContact,omitempty"`
	PatientConsented  bool       `json:"patientConsented"`
	StartDate         time.Time  `json:"startDate"`
	ExpectedEndDate   *time.Time `json:"expectedEndDate,omitempty"`
}

type hithEpisodeUpdateRequest struct {
	LeadClinicianHPI string            `json:"leadClinicianHpi,omitempty"`
	Status           HITHEpisodeStatus `json:"status,omitempty"`
	CareGoals        []string          `json:"careGoals,omitempty"`
	DailyVisitFreq   string            `json:"dailyVisitFrequency,omitempty"`
	ExpectedEndDate  *time.Time        `json:"expectedEndDate,omitempty"`
}

type hithVisitRequest struct {
	CliniciandHPI  string        `json:"clinicianHpi"`
	VisitType      HITHVisitType `json:"visitType"`
	Vitals         Vitals        `json:"vitals,omitempty"`
	ClinicalNotes  string        `json:"clinicalNotes,omitempty"`
	Escalated      bool          `json:"escalated,omitempty"`
	EscalationNote string        `json:"escalationNote,omitempty"`
	NextVisitDate  *time.Time    `json:"nextVisitDate,omitempty"`
}
