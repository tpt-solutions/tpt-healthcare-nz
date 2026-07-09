package ot

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
			Type:        AssessmentFunctionalCapacity,
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
	a.AddRecommendation(Recommendation{Description: "Walking frame"})
	assert.Len(t, a.Recommendations, 1)
	assert.Equal(t, "rec-1", a.Recommendations[0].ID)
}

func TestAddOutcomeMeasure(t *testing.T) {
	a := &Assessment{}
	a.AddOutcomeMeasure(OutcomeMeasure{Name: "COPM", Score: 5.0})
	assert.Len(t, a.OutcomeMeasures, 1)
	assert.Equal(t, "measure-1", a.OutcomeMeasures[0].ID)
}

func TestNewAssessment(t *testing.T) {
	a := NewAssessment()
	assert.Equal(t, AssessmentScheduled, a.Status)
	assert.NotZero(t, a.CreatedAt)
	assert.NotZero(t, a.UpdatedAt)
}

func TestInterventionPlanValidate(t *testing.T) {
	valid := func() *InterventionPlan {
		return &InterventionPlan{
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
	p := &InterventionPlan{}
	p.AddGoal(InterventionGoal{Description: "Independence in ADLs"})
	assert.Len(t, p.Goals, 1)
	assert.Equal(t, "goal-1", p.Goals[0].ID)
}

func TestAddIntervention(t *testing.T) {
	p := &InterventionPlan{}
	p.AddIntervention(PlannedIntervention{Description: "ADL retraining"})
	assert.Len(t, p.Interventions, 1)
	assert.Equal(t, "intervention-1", p.Interventions[0].ID)
}

func TestNewInterventionPlan(t *testing.T) {
	p := NewInterventionPlan()
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

func TestOTConstants(t *testing.T) {
	// Assessment types
	assert.Equal(t, AssessmentType("functional_capacity"), AssessmentFunctionalCapacity)
	assert.Equal(t, AssessmentType("adl"), AssessmentADL)
	assert.Equal(t, AssessmentType("home_safety"), AssessmentHomeSafety)
	assert.Equal(t, AssessmentType("worksite"), AssessmentWorksite)
	assert.Equal(t, AssessmentType("cognitive"), AssessmentCognitive)
	assert.Equal(t, AssessmentType("driving"), AssessmentDriving)
	assert.Equal(t, AssessmentType("wheelchair_seating"), AssessmentWheelchair)
	assert.Equal(t, AssessmentType("assistive_technology"), AssessmentAssistiveTech)
	assert.Equal(t, AssessmentType("vocational"), AssessmentVocational)

	// Intervention types
	assert.Equal(t, InterventionType("adl_retraining"), InterventionADLRetraining)
	assert.Equal(t, InterventionType("cognitive_rehab"), InterventionCognitiveRehab)
	assert.Equal(t, InterventionType("home_modification"), InterventionHomeModification)
	assert.Equal(t, InterventionType("equipment_prescription"), InterventionEquipmentPrescription)
	assert.Equal(t, InterventionType("falls_prevention"), InterventionFallsPrevention)
	assert.Equal(t, InterventionType("driver_rehab"), InterventionDriverRehab)
	assert.Equal(t, InterventionType("paediatric_play"), InterventionPaediatricPlay)

	// Recommendation statuses (OT-specific: Ordered, Delivered)
	assert.Equal(t, RecommendationStatus("ordered"), RecommendationOrdered)
	assert.Equal(t, RecommendationStatus("delivered"), RecommendationDelivered)

	// Plan statuses
	assert.Equal(t, PlanStatus("draft"), PlanStatusDraft)
	assert.Equal(t, PlanStatus("active"), PlanStatusActive)
	assert.Equal(t, PlanStatus("under_review"), PlanStatusUnderReview)
}
