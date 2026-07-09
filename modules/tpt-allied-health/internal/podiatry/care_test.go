package podiatry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const validNHI = "ABC1235"

func TestAssessmentValidate(t *testing.T) {
	valid := func() *Assessment {
		return &Assessment{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
			Type:        AssessmentDiabeticFoot,
			Date:        1700000000000,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		a := valid()
		a.PatientNHI = ""
		assert.Error(t, a.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		a := valid()
		a.ClinicianID = ""
		assert.Error(t, a.Validate())
	})

	t.Run("missing type", func(t *testing.T) {
		a := valid()
		a.Type = ""
		assert.Error(t, a.Validate())
	})

	t.Run("missing date", func(t *testing.T) {
		a := valid()
		a.Date = 0
		assert.Error(t, a.Validate())
	})
}

func TestAddRecommendation(t *testing.T) {
	a := &Assessment{}
	a.AddRecommendation(Recommendation{Description: "Refer to vascular"})
	assert.Len(t, a.Recommendations, 1)
	assert.Equal(t, "rec-1", a.Recommendations[0].ID)
}

func TestAddOutcomeMeasure(t *testing.T) {
	a := &Assessment{}
	a.AddOutcomeMeasure(OutcomeMeasure{Name: "VPT", Score: 25})
	assert.Len(t, a.OutcomeMeasures, 1)
	assert.Equal(t, "measure-1", a.OutcomeMeasures[0].ID)
}

func TestNewAssessment(t *testing.T) {
	a := NewAssessment()
	assert.Equal(t, AssessmentScheduled, a.Status)
	assert.NotZero(t, a.CreatedAt)
	assert.NotZero(t, a.UpdatedAt)
}

func TestTreatmentPlanValidate(t *testing.T) {
	valid := func() *TreatmentPlan {
		return &TreatmentPlan{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
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

	t.Run("missing clinician", func(t *testing.T) {
		p := valid()
		p.ClinicianID = ""
		assert.Error(t, p.Validate())
	})

	t.Run("missing start date", func(t *testing.T) {
		p := valid()
		p.StartDate = 0
		assert.Error(t, p.Validate())
	})
}

func TestAddGoal(t *testing.T) {
	p := &TreatmentPlan{}
	p.AddGoal(TreatmentGoal{Description: "Wound healing"})
	assert.Len(t, p.Goals, 1)
	assert.Equal(t, "goal-1", p.Goals[0].ID)
}

func TestAddIntervention(t *testing.T) {
	p := &TreatmentPlan{}
	p.AddIntervention(PlannedIntervention{Description: "Offloading"})
	assert.Len(t, p.Interventions, 1)
	assert.Equal(t, "intervention-1", p.Interventions[0].ID)
}

func TestNewTreatmentPlan(t *testing.T) {
	p := NewTreatmentPlan()
	assert.Equal(t, PlanStatusDraft, p.Status)
	assert.NotZero(t, p.CreatedAt)
	assert.NotZero(t, p.UpdatedAt)
}

func TestSessionNoteValidate(t *testing.T) {
	valid := func() *SessionNote {
		return &SessionNote{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
			SessionDate: 1700000000000,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		s := valid()
		s.PatientNHI = ""
		assert.Error(t, s.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		s := valid()
		s.ClinicianID = ""
		assert.Error(t, s.Validate())
	})

	t.Run("missing session date", func(t *testing.T) {
		s := valid()
		s.SessionDate = 0
		assert.Error(t, s.Validate())
	})
}

func TestNewSessionNote(t *testing.T) {
	s := NewSessionNote()
	assert.NotZero(t, s.CreatedAt)
	assert.NotZero(t, s.UpdatedAt)
}

func TestWoundAssessmentValidate(t *testing.T) {
	valid := func() *WoundAssessment {
		return &WoundAssessment{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
			Date:        1700000000000,
			Location:    "left foot",
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		w := valid()
		w.PatientNHI = ""
		assert.Error(t, w.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		w := valid()
		w.ClinicianID = ""
		assert.Error(t, w.Validate())
	})

	t.Run("missing date", func(t *testing.T) {
		w := valid()
		w.Date = 0
		assert.Error(t, w.Validate())
	})

	t.Run("missing location", func(t *testing.T) {
		w := valid()
		w.Location = ""
		assert.Error(t, w.Validate())
	})
}

func TestNewWoundAssessment(t *testing.T) {
	w := NewWoundAssessment()
	assert.Equal(t, AssessmentScheduled, w.Status)
	assert.NotZero(t, w.CreatedAt)
	assert.NotZero(t, w.UpdatedAt)
}

func TestPodiatryConstants(t *testing.T) {
	// Assessment types
	assert.Equal(t, AssessmentType("general"), AssessmentGeneral)
	assert.Equal(t, AssessmentType("diabetic_foot"), AssessmentDiabeticFoot)
	assert.Equal(t, AssessmentType("vascular"), AssessmentVascular)
	assert.Equal(t, AssessmentType("neurological"), AssessmentNeurological)
	assert.Equal(t, AssessmentType("biomechanical"), AssessmentBiomechanical)
	assert.Equal(t, AssessmentType("wound"), AssessmentWound)
	assert.Equal(t, AssessmentType("high_risk_foot"), AssessmentHighRiskFoot)

	// Treatment types
	assert.Equal(t, TreatmentType("nail_care"), TreatmentNailCare)
	assert.Equal(t, TreatmentType("callus_debridement"), TreatmentCallusDebridement)
	assert.Equal(t, TreatmentType("wound_debridement"), TreatmentWoundDebridement)
	assert.Equal(t, TreatmentType("orthotic_therapy"), TreatmentOrthoticTherapy)
	assert.Equal(t, TreatmentType("nail_surgery"), TreatmentNailSurgery)

	// Risk categories
	assert.Equal(t, RiskCategory("low"), RiskCategoryLow)
	assert.Equal(t, RiskCategory("moderate"), RiskCategoryModerate)
	assert.Equal(t, RiskCategory("high"), RiskCategoryHigh)
	assert.Equal(t, RiskCategory("very_high"), RiskCategoryVeryHigh)
	assert.Equal(t, RiskCategory("active"), RiskCategoryActive)

	// Wound types
	assert.Equal(t, WoundType("diabetic_foot_ulcer"), WoundTypeDiabeticFoot)
	assert.Equal(t, WoundType("venous_leg_ulcer"), WoundTypeVenous)
	assert.Equal(t, WoundType("arterial_ulcer"), WoundTypeArterial)
	assert.Equal(t, WoundType("pressure_injury"), WoundTypePressure)
	assert.Equal(t, WoundType("surgical_wound"), WoundTypeSurgical)
	assert.Equal(t, WoundType("trauma_burn"), WoundTypeTrauma)
	assert.Equal(t, WoundType("moisture_associated"), WoundTypeMoisture)

	// Tissue types
	assert.Equal(t, TissueType("necrotic"), TissueNecrotic)
	assert.Equal(t, TissueType("sloughy"), TissueSloughy)
	assert.Equal(t, TissueType("granulating"), TissueGranulating)
	assert.Equal(t, TissueType("epithelialising"), TissueEpithelialising)

	// Exudate levels
	assert.Equal(t, ExudateLevel("none"), ExudateNone)
	assert.Equal(t, ExudateLevel("low"), ExudateLow)
	assert.Equal(t, ExudateLevel("moderate"), ExudateModerate)
	assert.Equal(t, ExudateLevel("high"), ExudateHigh)

	// Vascular statuses
	assert.Equal(t, VascularStatus("normal"), VascularNormal)
	assert.Equal(t, VascularStatus("reduced"), VascularReduced)
	assert.Equal(t, VascularStatus("absent"), VascularAbsent)

	// Neurological statuses
	assert.Equal(t, NeurologicalStatus("intact"), NeurologicalIntact)
	assert.Equal(t, NeurologicalStatus("reduced"), NeurologicalReduced)
	assert.Equal(t, NeurologicalStatus("absent"), NeurologicalAbsent)
}
