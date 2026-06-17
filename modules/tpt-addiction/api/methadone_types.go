package api

import (
	"time"

	"github.com/PhillipC05/tpt-healthcare/modules/tpt-addiction/internal/methadone"
)

// Programme is the API representation of an OST programme (maps addiction_programmes).
type Programme struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenantId"`
	PatientNHI       string     `json:"patientNhi"`
	ClinicianID      string     `json:"clinicianId"`
	PracticeID       string     `json:"practiceId"`
	StartDate        time.Time  `json:"startDate"`
	EndDate          *time.Time `json:"endDate,omitempty"`
	Phase            string     `json:"phase"`
	SubstancePrimary string     `json:"substancePrimary"`
	SubstanceOther   string     `json:"substanceOther,omitempty"`
	InitialDoseMg    float64    `json:"initialDoseMg"`
	CurrentDoseMg    float64    `json:"currentDoseMg"`
	TargetDoseMg     *float64   `json:"targetDoseMg,omitempty"`
	TakeHomeLevel    int        `json:"takeHomeLevel"`
	TakeHomeMaxDays  int        `json:"takeHomeMaxDays"`
	Pregnancy        bool       `json:"pregnancy"`
	Comorbidities    []string   `json:"comorbidities"`
	LastUrineDate    *time.Time `json:"lastUrineDate,omitempty"`
	NextReviewDate   time.Time  `json:"nextReviewDate"`
	// PRIMHDReferralID is the identifier issued by PRIMHD when the referral was
	// opened for this patient. Required for activity and discharge reporting.
	PRIMHDReferralID string    `json:"primhdReferralId,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

const progSelectCols = `id, tenant_id, patient_nhi, clinician_id, practice_id,
       start_date, end_date, phase,
       substance_primary, COALESCE(substance_other,''),
       initial_dose_mg, current_dose_mg, target_dose_mg,
       take_home_level, take_home_max_days, pregnancy,
       COALESCE(comorbidities, '{}'), last_urine_date, next_review_date,
       COALESCE(primhd_referral_id,''),
       created_at, updated_at`

func scanProgramme(row interface{ Scan(...any) error }, p *Programme) error {
	return row.Scan(
		&p.ID, &p.TenantID, &p.PatientNHI, &p.ClinicianID, &p.PracticeID,
		&p.StartDate, &p.EndDate, &p.Phase,
		&p.SubstancePrimary, &p.SubstanceOther,
		&p.InitialDoseMg, &p.CurrentDoseMg, &p.TargetDoseMg,
		&p.TakeHomeLevel, &p.TakeHomeMaxDays, &p.Pregnancy,
		&p.Comorbidities, &p.LastUrineDate, &p.NextReviewDate,
		&p.PRIMHDReferralID,
		&p.CreatedAt, &p.UpdatedAt,
	)
}

// DoseRecord is the API representation of a methadone dose (maps methadone_doses).
type DoseRecord struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenantId"`
	ProgrammeID     string    `json:"programmeId"`
	AdministeredAt  time.Time `json:"administeredAt"`
	DoseMg          float64   `json:"doseMg"`
	Formulation     string    `json:"formulation"`
	WitnessedBy     string    `json:"witnessedBy"`
	DispensedBy     string    `json:"dispensedBy"`
	PharmacistCheck bool      `json:"pharmacistCheck"`
	Status          string    `json:"status"`
	Notes           string    `json:"notes,omitempty"`
	TakeHome        bool      `json:"takeHome"`
	CreatedAt       time.Time `json:"createdAt"`
}

const doseSelectCols = `id, tenant_id, programme_id, administered_at, dose_mg,
       formulation, witnessed_by, dispensed_by, pharmacist_check,
       status, COALESCE(notes,''), take_home, created_at`

// TakeHomeApproval is the API representation of methadone_take_home_approvals.
type TakeHomeApproval struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenantId"`
	ProgrammeID   string     `json:"programmeId"`
	ApprovedBy    string     `json:"approvedBy"`
	ApprovedAt    time.Time  `json:"approvedAt"`
	Level         int        `json:"level"`
	MaxDays       int        `json:"maxDays"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokedBy     string     `json:"revokedBy,omitempty"`
	RevokedReason string     `json:"revokedReason,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
}

const takeHomeSelectCols = `id, tenant_id, programme_id, approved_by, approved_at,
       level, max_days, expires_at, revoked_at,
       COALESCE(revoked_by,''), COALESCE(revoked_reason,''), created_at`

// UrineScreen is the API representation of a urine drug screen (maps urine_screens).
type UrineScreen struct {
	ID            string                 `json:"id"`
	TenantID      string                 `json:"tenantId"`
	ProgrammeID   string                 `json:"programmeId"`
	CollectedAt   time.Time             `json:"collectedAt"`
	CollectedBy   string                 `json:"collectedBy"`
	LabName       string                 `json:"labName,omitempty"`
	LabReference  string                 `json:"labReference,omitempty"`
	Results       []methadone.DrugResult `json:"results"`
	MSSAResult    string                 `json:"mssaResult"`
	ClinicalNotes string                 `json:"clinicalNotes,omitempty"`
	CreatedAt     time.Time             `json:"createdAt"`
}
