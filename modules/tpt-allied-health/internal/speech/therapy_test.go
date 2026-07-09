package speech

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
			Type:        AssessmentSpeechSound,
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

	t.Run("all assessment types accepted", func(t *testing.T) {
		types := []AssessmentType{
			AssessmentSpeechSound, AssessmentLanguage, AssessmentFluency,
			AssessmentVoice, AssessmentSwallowing, AssessmentCognitiveComm,
			AssessmentAAC, AssessmentPaediatric, AssessmentAdultNeuro, AssessmentProgress,
		}
		for _, at := range types {
			a := valid()
			a.Type = at
			assert.NoError(t, a.Validate(), "type %s", at)
		}
	})
}

func TestAddRecommendation(t *testing.T) {
	a := &Assessment{}
	a.AddRecommendation(Recommendation{Description: "Weekly therapy"})
	assert.Len(t, a.Recommendations, 1)
	assert.Equal(t, "rec-1", a.Recommendations[0].ID)
	assert.NotZero(t, a.Recommendations[0].CreatedAt)

	a.AddRecommendation(Recommendation{Description: "Monthly review"})
	assert.Equal(t, "rec-2", a.Recommendations[1].ID)
}

func TestAddOutcomeMeasure(t *testing.T) {
	a := &Assessment{}
	a.AddOutcomeMeasure(OutcomeMeasure{Name: "CELF-5", Score: 85})
	assert.Len(t, a.OutcomeMeasures, 1)
	assert.Equal(t, "measure-1", a.OutcomeMeasures[0].ID)
}

func TestNewAssessment(t *testing.T) {
	a := NewAssessment()
	assert.Equal(t, AssessmentScheduled, a.Status)
	assert.NotZero(t, a.CreatedAt)
	assert.NotZero(t, a.UpdatedAt)
	assert.Equal(t, a.CreatedAt, a.UpdatedAt)
}

func TestTherapyPlanValidate(t *testing.T) {
	valid := func() *TherapyPlan {
		return &TherapyPlan{
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
	p := &TherapyPlan{}
	p.AddGoal(TherapyGoal{Description: "Improve articulation"})
	assert.Len(t, p.Goals, 1)
	assert.Equal(t, "goal-1", p.Goals[0].ID)
	assert.NotZero(t, p.Goals[0].CreatedAt)
}

func TestAddIntervention(t *testing.T) {
	p := &TherapyPlan{}
	p.AddIntervention(PlannedIntervention{Description: "Articulation therapy"})
	assert.Len(t, p.Interventions, 1)
	assert.Equal(t, "intervention-1", p.Interventions[0].ID)
}

func TestNewTherapyPlan(t *testing.T) {
	p := NewTherapyPlan()
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

func TestSwallowingAssessmentValidate(t *testing.T) {
	valid := func() *SwallowingAssessment {
		return &SwallowingAssessment{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
			Date:        1700000000000,
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

	t.Run("missing date", func(t *testing.T) {
		s := valid()
		s.Date = 0
		assert.Error(t, s.Validate())
	})
}

func TestNewSwallowingAssessment(t *testing.T) {
	s := NewSwallowingAssessment()
	assert.Equal(t, AssessmentScheduled, s.Status)
	assert.NotZero(t, s.CreatedAt)
	assert.NotZero(t, s.UpdatedAt)
}

func TestSpeechConstants(t *testing.T) {
	// Assessment types
	assert.Equal(t, AssessmentType("speech_sound"), AssessmentSpeechSound)
	assert.Equal(t, AssessmentType("language"), AssessmentLanguage)
	assert.Equal(t, AssessmentType("fluency"), AssessmentFluency)
	assert.Equal(t, AssessmentType("voice"), AssessmentVoice)
	assert.Equal(t, AssessmentType("swallowing"), AssessmentSwallowing)
	assert.Equal(t, AssessmentType("cognitive_communication"), AssessmentCognitiveComm)
	assert.Equal(t, AssessmentType("aac"), AssessmentAAC)
	assert.Equal(t, AssessmentType("paediatric"), AssessmentPaediatric)
	assert.Equal(t, AssessmentType("adult_neurological"), AssessmentAdultNeuro)
	assert.Equal(t, AssessmentType("progress_review"), AssessmentProgress)

	// Assessment statuses
	assert.Equal(t, AssessmentStatus("scheduled"), AssessmentScheduled)
	assert.Equal(t, AssessmentStatus("in_progress"), AssessmentInProgress)
	assert.Equal(t, AssessmentStatus("completed"), AssessmentCompleted)
	assert.Equal(t, AssessmentStatus("cancelled"), AssessmentCancelled)
	assert.Equal(t, AssessmentStatus("on_hold"), AssessmentOnHold)

	// Plan statuses
	assert.Equal(t, PlanStatus("draft"), PlanStatusDraft)
	assert.Equal(t, PlanStatus("active"), PlanStatusActive)
	assert.Equal(t, PlanStatus("completed"), PlanStatusCompleted)
	assert.Equal(t, PlanStatus("discontinued"), PlanStatusDiscontinued)

	// Goal statuses
	assert.Equal(t, GoalStatus("not_started"), GoalStatusNotStarted)
	assert.Equal(t, GoalStatus("in_progress"), GoalStatusInProgress)
	assert.Equal(t, GoalStatus("achieved"), GoalStatusAchieved)

	// Intervention statuses
	assert.Equal(t, InterventionStatus("planned"), InterventionPlanned)
	assert.Equal(t, InterventionStatus("active"), InterventionActive)
	assert.Equal(t, InterventionStatus("completed"), InterventionCompleted)

	// Recommendation priorities
	assert.Equal(t, RecommendationPriority("urgent"), PriorityUrgent)
	assert.Equal(t, RecommendationPriority("high"), PriorityHigh)
	assert.Equal(t, RecommendationPriority("routine"), PriorityRoutine)
}
