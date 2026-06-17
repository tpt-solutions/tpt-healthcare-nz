package api

import "time"

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
	PICUStatusAdmitted   PICUStatus = "admitted"
	PICUStatusStable     PICUStatus = "stable"
	PICUStatusCritical   PICUStatus = "critical"
	PICUStatusDischarged PICUStatus = "discharged"
)

// DevelopmentalDomain classifies the developmental area being assessed.
type DevelopmentalDomain string

const (
	DevDomainGrossMotor      DevelopmentalDomain = "gross-motor"
	DevDomainFineMotor       DevelopmentalDomain = "fine-motor"
	DevDomainSpeechLanguage  DevelopmentalDomain = "speech-language"
	DevDomainSocialEmotional DevelopmentalDomain = "social-emotional"
	DevDomainCognitive       DevelopmentalDomain = "cognitive"
)

// ChildProtectionStatus tracks the child protection concern lifecycle.
// Flagging and reporting must comply with the Children's Act 2014 (NZ).
type ChildProtectionStatus string

const (
	ChildProtectionNone               ChildProtectionStatus = "none"
	ChildProtectionConcernRaised      ChildProtectionStatus = "concern-raised"
	ChildProtectionNotified           ChildProtectionStatus = "notified"
	ChildProtectionUnderInvestigation ChildProtectionStatus = "under-investigation"
)

type PaediatricAdmission struct {
	ID               string     `json:"id"`
	PatientNHI       string     `json:"patientNhi"`
	ProxyGuardianNHI *string    `json:"proxyGuardianNhi"`
	ClinicianHpi     string     `json:"clinicianHpi"`
	Status           string     `json:"status"`
	AdmissionType    string     `json:"admissionType"`
	AdmissionReason  string     `json:"admissionReason"`
	Ward             string     `json:"ward"`
	BedLabel         string     `json:"bedLabel"`
	AgeYears         *int16     `json:"ageYears"`
	AgeMonths        *int16     `json:"ageMonths"`
	WeightKg         *float64   `json:"weightKg"`
	HeightCm         *float64   `json:"heightCm"`
	TenantID         string     `json:"tenantId"`
	AdmittedAt       time.Time  `json:"admittedAt"`
	DischargedAt     *time.Time `json:"dischargedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type PICUAdmission struct {
	ID                    string     `json:"id"`
	PaediatricAdmissionID string     `json:"paediatricAdmissionId"`
	PatientNHI            string     `json:"patientNhi"`
	ClinicianHpi          string     `json:"clinicianHpi"`
	Status                string     `json:"status"`
	AdmissionReason       string     `json:"admissionReason"`
	AdmissionType         string     `json:"admissionType"`
	RespiratorySupport    string     `json:"respiratorySupport"`
	TpnActive             bool       `json:"tpnActive"`
	InotropesActive       bool       `json:"inotropesActive"`
	BedLabel              string     `json:"bedLabel"`
	TenantID              string     `json:"tenantId"`
	AdmittedAt            time.Time  `json:"admittedAt"`
	DischargedAt          *time.Time `json:"dischargedAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type PaediatricGrowthRecord struct {
	ID                    string    `json:"id"`
	PaediatricAdmissionID string    `json:"paediatricAdmissionId"`
	PatientNHI            string    `json:"patientNhi"`
	ClinicianHpi          string    `json:"clinicianHpi"`
	WeightKg              *float64  `json:"weightKg"`
	HeightCm              *float64  `json:"heightCm"`
	HeadCircumferenceCm   *float64  `json:"headCircumferenceCm"`
	Bmi                   *float64  `json:"bmi"`
	CentileBand           *string   `json:"centileBand"`
	RecordedAt            time.Time `json:"recordedAt"`
	TenantID              string    `json:"tenantId"`
}

type DevelopmentalMilestone struct {
	ID                    string    `json:"id"`
	PaediatricAdmissionID string    `json:"paediatricAdmissionId"`
	PatientNHI            string    `json:"patientNhi"`
	ClinicianHpi          string    `json:"clinicianHpi"`
	Domain                string    `json:"domain"`
	MilestoneDescription  string    `json:"milestoneDescription"`
	ExpectedAgeMonths     *int16    `json:"expectedAgeMonths"`
	Achieved              bool      `json:"achieved"`
	AchievedAt            *string   `json:"achievedAt"`
	ConcernNoted          bool      `json:"concernNoted"`
	Notes                 *string   `json:"notes"`
	AssessedAt            time.Time `json:"assessedAt"`
	TenantID              string    `json:"tenantId"`
}

type ChildProtectionFlag struct {
	ID                    string     `json:"id"`
	PaediatricAdmissionID string     `json:"paediatricAdmissionId"`
	PatientNHI            string     `json:"patientNhi"`
	RaisedByHpi           string     `json:"raisedByHpi"`
	Status                string     `json:"status"`
	ConcernDescription    string     `json:"concernDescription"`
	NotifiedAt            *time.Time `json:"notifiedAt"`
	NotifiedBody          *string    `json:"notifiedBody"`
	CaseReference         *string    `json:"caseReference"`
	ResolvedAt            *time.Time `json:"resolvedAt"`
	Notes                 *string    `json:"notes"`
	TenantID              string     `json:"tenantId"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}
