// Package hospice provides domain models for hospice / palliative patient care.
package hospice

import "time"

// CareSetting identifies where the patient is receiving care.
type CareSetting string

const (
	SettingHome         CareSetting = "home"
	SettingInpatient    CareSetting = "inpatient"
	SettingResidential  CareSetting = "residential"
	SettingHospital     CareSetting = "hospital"
)

// PerformanceStatus is an ECOG / Palliative Performance Scale (PPS) value.
type PerformanceStatus string

const (
	PPS100 PerformanceStatus = "100" // fully ambulatory
	PPS90  PerformanceStatus = "90"
	PPS80  PerformanceStatus = "80"
	PPS70  PerformanceStatus = "70"
	PPS60  PerformanceStatus = "60"
	PPS50  PerformanceStatus = "50"
	PPS40  PerformanceStatus = "40"
	PPS30  PerformanceStatus = "30"
	PPS20  PerformanceStatus = "20"
	PPS10  PerformanceStatus = "10"
	PPS0   PerformanceStatus = "0" // death
)

// Patient represents a patient enrolled in a palliative care programme.
type Patient struct {
	ID                string          `json:"id"`
	PatientNHI        string          `json:"patientNhi"`
	PrimaryDiagnosis  string          `json:"primaryDiagnosis"`
	SecondaryDiagnoses []string       `json:"secondaryDiagnoses,omitempty"`
	PerformanceStatus PerformanceStatus `json:"performanceStatus"`
	CareSetting       CareSetting     `json:"careSetting"`
	LocationID        *string         `json:"locationId,omitempty"`   // inpatient bed / room
	ResponsibleClinicianID string     `json:"responsibleClinicianId"` // palliative specialist or GP
	NurseCoordinatorID   string       `json:"nurseCoordinatorId,omitempty"`
	AdmissionDate     time.Time       `json:"admissionDate"`
	ExpectedDischargeDate *time.Time  `json:"expectedDischargeDate,omitempty"`
	DischargeDate     *time.Time      `json:"dischargeDate,omitempty"`
	DischargeReason   *string         `json:"dischargeReason,omitempty"` // death, transfer, recovered, changed_mind
	AdvanceCarePlanID *string         `json:"advanceCarePlanId,omitempty"`
	GoalsOfCare       []GoalOfCare    `json:"goalsOfCare,omitempty"`
	FamilyContacts    []FamilyContact `json:"familyContacts,omitempty"`
	SpiritualNeeds    string          `json:"spiritualNeeds,omitempty"`
	CulturalNeeds     string          `json:"culturalNeeds,omitempty"`
	PreferredPlaceOfDeath *string     `json:"preferredPlaceOfDeath,omitempty"`
	DNACPRInPlace     bool            `json:"dnacprInPlace"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

// GoalOfCare captures a single goal agreed with the patient / whānau.
type GoalOfCare struct {
	ID          string    `json:"id"`
	Goal        string    `json:"goal"`
	Category    string    `json:"category"` // comfort, symptom_control, dignity, family_support, spiritual
	Priority    int       `json:"priority"`
	Achieved    bool      `json:"achieved"`
	AchievedAt  *time.Time `json:"achievedAt,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// FamilyContact for next-of-kin or key support person.
type FamilyContact struct {
	Name      string `json:"name"`
	Relationship string `json:"relationship"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	IsPrimary bool   `json:"isPrimary"`
	IsEmergencyContact bool `json:"isEmergencyContact"`
}

// VisitRecord documents a palliative care team visit (home, inpatient, or virtual).
type VisitRecord struct {
	ID           string    `json:"id"`
	PatientID    string    `json:"patientId"`
	VisitType    string    `json:"visitType"` // scheduled, urgent, virtual, bereavement
	VisitDate    time.Time `json:"visitDate"`
	ClinicianID  string    `json:"clinicianId"`
	Disciplines  []string  `json:"disciplines,omitempty"` // medical, nursing, social_work, spiritual, volunteer
	Symptoms     []Symptom `json:"symptoms,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	NextReviewDate *time.Time `json:"nextReviewDate,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Symptom records a single symptom assessment.
type Symptom struct {
	Name      string  `json:"name"`
	Severity  int     `json:"severity"` // 0-10
	Intervention string `json:"intervention,omitempty"`
	Resolved  bool    `json:"resolved"`
}
