package districtnursing

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ABC1235 passes old-format NHI Luhn checksum.
const validNHI = "ABC1235"

func TestCarePlanValidate(t *testing.T) {
	valid := func() *CarePlan {
		return &CarePlan{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
			PlanName:    "Wound care plan",
			PlanType:    PlanWoundCare,
			StartDate:   1700000000000,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		p := valid()
		p.PatientNHI = ""
		assert.Error(t, p.Validate())
	})

	t.Run("invalid NHI", func(t *testing.T) {
		p := valid()
		p.PatientNHI = "ZZZ9999"
		assert.Error(t, p.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		p := valid()
		p.ClinicianID = ""
		assert.Error(t, p.Validate())
	})

	t.Run("missing plan name", func(t *testing.T) {
		p := valid()
		p.PlanName = ""
		assert.Error(t, p.Validate())
	})

	t.Run("missing plan type", func(t *testing.T) {
		p := valid()
		p.PlanType = ""
		assert.Error(t, p.Validate())
	})

	t.Run("missing start date", func(t *testing.T) {
		p := valid()
		p.StartDate = 0
		assert.Error(t, p.Validate())
	})
}

func TestNewCarePlan(t *testing.T) {
	p := NewCarePlan()
	assert.Equal(t, PlanDraft, p.Status)
	assert.Equal(t, RiskLow, p.RiskLevel)
	assert.False(t, p.ConsentGiven)
	assert.False(t, p.DHBFunded)
	assert.NotNil(t, p.Goals)
	assert.Empty(t, p.Goals)
	assert.NotZero(t, p.CreatedAt)
	assert.NotZero(t, p.UpdatedAt)
	assert.Equal(t, p.CreatedAt, p.UpdatedAt)
}

func TestNursingVisitValidate(t *testing.T) {
	valid := func() *NursingVisit {
		return &NursingVisit{
			CarePlanID:  "cp-1",
			PatientNHI:  validNHI,
			ClinicianID: "RN001",
			VisitDate:   1700000000000,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing care plan ID", func(t *testing.T) {
		v := valid()
		v.CarePlanID = ""
		assert.Error(t, v.Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		v := valid()
		v.PatientNHI = ""
		assert.Error(t, v.Validate())
	})

	t.Run("invalid NHI", func(t *testing.T) {
		v := valid()
		v.PatientNHI = "ZZZ9999"
		assert.Error(t, v.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		v := valid()
		v.ClinicianID = ""
		assert.Error(t, v.Validate())
	})

	t.Run("missing visit date", func(t *testing.T) {
		v := valid()
		v.VisitDate = 0
		assert.Error(t, v.Validate())
	})
}

func TestNewNursingVisit(t *testing.T) {
	v := NewNursingVisit()
	assert.Equal(t, VisitStatusScheduled, v.VisitStatus)
	assert.NotNil(t, v.WoundAssessments)
	assert.Empty(t, v.WoundAssessments)
	assert.NotNil(t, v.MedicationsAdministered)
	assert.Empty(t, v.MedicationsAdministered)
	assert.NotNil(t, v.PatientEducation)
	assert.Empty(t, v.PatientEducation)
	assert.NotNil(t, v.Concerns)
	assert.Empty(t, v.Concerns)
	assert.NotZero(t, v.CreatedAt)
	assert.NotZero(t, v.UpdatedAt)
}

func TestDistrictNursingConstants(t *testing.T) {
	// Plan types
	assert.Equal(t, PlanType("wound_care"), PlanWoundCare)
	assert.Equal(t, PlanType("palliative"), PlanPalliative)
	assert.Equal(t, PlanType("diabetes"), PlanDiabetes)
	assert.Equal(t, PlanType("heart_failure"), PlanHeartFailure)
	assert.Equal(t, PlanType("copd"), PlanCOPD)
	assert.Equal(t, PlanType("post_surgical"), PlanPostSurgical)
	assert.Equal(t, PlanType("post_acute"), PlanPostAcute)
	assert.Equal(t, PlanType("medication_management"), PlanMedication)

	// Plan statuses
	assert.Equal(t, PlanStatus("draft"), PlanDraft)
	assert.Equal(t, PlanStatus("active"), PlanActive)
	assert.Equal(t, PlanStatus("under_review"), PlanUnderReview)
	assert.Equal(t, PlanStatus("completed"), PlanCompleted)
	assert.Equal(t, PlanStatus("suspended"), PlanSuspended)

	// Risk levels
	assert.Equal(t, RiskLevel("low"), RiskLow)
	assert.Equal(t, RiskLevel("moderate"), RiskModerate)
	assert.Equal(t, RiskLevel("high"), RiskHigh)
	assert.Equal(t, RiskLevel("very_high"), RiskVeryHigh)

	// Visit types
	assert.Equal(t, VisitType("scheduled"), VisitScheduled)
	assert.Equal(t, VisitType("unscheduled"), VisitUnscheduled)
	assert.Equal(t, VisitType("urgent"), VisitUrgent)

	// Visit statuses
	assert.Equal(t, VisitStatus("scheduled"), VisitStatusScheduled)
	assert.Equal(t, VisitStatus("in_progress"), VisitStatusInProgress)
	assert.Equal(t, VisitStatus("completed"), VisitStatusCompleted)
	assert.Equal(t, VisitStatus("cancelled"), VisitStatusCancelled)

	// Admin routes
	assert.Equal(t, AdminRoute("oral"), RouteOral)
	assert.Equal(t, AdminRoute("im"), RouteIM)
	assert.Equal(t, AdminRoute("iv"), RouteIV)
	assert.Equal(t, AdminRoute("sc"), RouteSC)
	assert.Equal(t, AdminRoute("topical"), RouteTopical)
	assert.Equal(t, AdminRoute("inhalation"), RouteInhalation)

	// Admin statuses
	assert.Equal(t, AdminStatus("scheduled"), AdminScheduled)
	assert.Equal(t, AdminStatus("administered"), AdminAdministered)
	assert.Equal(t, AdminStatus("refused"), AdminRefused)
	assert.Equal(t, AdminStatus("omitted"), AdminOmitted)
	assert.Equal(t, AdminStatus("held"), AdminHeld)

	// Wound causes
	assert.Equal(t, WoundCause("pressure_injury"), WoundPressure)
	assert.Equal(t, WoundCause("venous"), WoundVenous)
	assert.Equal(t, WoundCause("arterial"), WoundArterial)
	assert.Equal(t, WoundCause("diabetic"), WoundDiabetic)
	assert.Equal(t, WoundCause("surgical"), WoundSurgical)
	assert.Equal(t, WoundCause("trauma"), WoundTrauma)
}

func TestVitalSignsStruct(t *testing.T) {
	vs := VitalSigns{
		Temperature:            36.8,
		BloodPressureSystolic:  120,
		BloodPressureDiastolic: 80,
		HeartRate:              72,
		SpO2:                   98.5,
		PainScore:              2,
		WeightKg:               75.5,
		RespiratoryRate:        16,
		BloodGlucose:           5.6,
	}
	assert.Equal(t, 36.8, vs.Temperature)
	assert.Equal(t, 120, vs.BloodPressureSystolic)
	assert.Equal(t, 80, vs.BloodPressureDiastolic)
	assert.Equal(t, 72, vs.HeartRate)
	assert.Equal(t, 98.5, vs.SpO2)
	assert.Equal(t, 2, vs.PainScore)
	assert.Equal(t, 75.5, vs.WeightKg)
	assert.Equal(t, 16, vs.RespiratoryRate)
	assert.Equal(t, 5.6, vs.BloodGlucose)
}

func TestWoundAssessmentStruct(t *testing.T) {
	wa := WoundAssessment{
		WoundSite:     "left ankle",
		WoundCause:    WoundVenous,
		LengthCM:      3.5,
		WidthCM:       2.0,
		DepthCM:       0.5,
		TissueType:    "granulating",
		ExudateAmount: "moderate",
		Odour:         false,
		Debridement:   false,
		PhotosTaken:   true,
	}
	assert.Equal(t, "left ankle", wa.WoundSite)
	assert.Equal(t, WoundVenous, wa.WoundCause)
	assert.Equal(t, 3.5, wa.LengthCM)
	assert.Equal(t, "granulating", wa.TissueType)
	assert.False(t, wa.Odour)
	assert.True(t, wa.PhotosTaken)
}

func TestMedicationAdminStruct(t *testing.T) {
	ma := MedicationAdmin{
		MedicationName:       "Metformin",
		Dose:                 "500mg",
		Route:                RouteOral,
		Frequency:            "BD",
		AdministrationStatus: AdminAdministered,
	}
	assert.Equal(t, "Metformin", ma.MedicationName)
	assert.Equal(t, RouteOral, ma.Route)
	assert.Equal(t, AdminAdministered, ma.AdministrationStatus)
}

func TestInfectionSignsStruct(t *testing.T) {
	is := InfectionSigns{
		Erythema:        true,
		Oedema:          false,
		Heat:            true,
		Pain:            true,
		PurulentExudate: false,
		Odour:           false,
	}
	assert.True(t, is.Erythema)
	assert.False(t, is.Oedema)
	assert.True(t, is.Heat)
	assert.True(t, is.Pain)
	assert.False(t, is.PurulentExudate)
	assert.False(t, is.Odour)
}
