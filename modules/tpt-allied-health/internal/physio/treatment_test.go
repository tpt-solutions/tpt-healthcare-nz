package physio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const validNHI = "ABC1235"

func TestTreatmentPlanValidate(t *testing.T) {
	valid := func() *TreatmentPlan {
		return &TreatmentPlan{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
			Diagnosis:   "ACL tear",
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

	t.Run("missing diagnosis", func(t *testing.T) {
		p := valid()
		p.Diagnosis = ""
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
	p.AddGoal(TreatmentGoal{Description: "Full ROM"})
	assert.Len(t, p.Goals, 1)
	assert.Equal(t, "goal-1", p.Goals[0].ID)
	assert.NotZero(t, p.Goals[0].CreatedAt)

	p.AddGoal(TreatmentGoal{Description: "Return to sport"})
	assert.Equal(t, "goal-2", p.Goals[1].ID)
}

func TestAddIntervention(t *testing.T) {
	p := &TreatmentPlan{}
	p.AddIntervention(Intervention{Description: "Manual therapy"})
	assert.Len(t, p.Interventions, 1)
	assert.Equal(t, "intervention-1", p.Interventions[0].ID)
}

func TestAddOutcomeMeasure(t *testing.T) {
	p := &TreatmentPlan{}
	p.AddOutcomeMeasure(OutcomeMeasure{Name: "NDI", Score: 30})
	assert.Len(t, p.OutcomeMeasures, 1)
	assert.Equal(t, "measure-1", p.OutcomeMeasures[0].ID)
}

func TestNewTreatmentPlan(t *testing.T) {
	p := NewTreatmentPlan()
	assert.Equal(t, PlanStatusDraft, p.Status)
	assert.NotZero(t, p.CreatedAt)
	assert.NotZero(t, p.UpdatedAt)
	assert.Equal(t, p.CreatedAt, p.UpdatedAt)
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

func TestPhysioConstants(t *testing.T) {
	// Treatment types
	assert.Equal(t, TreatmentType("manual_therapy"), TreatmentManualTherapy)
	assert.Equal(t, TreatmentType("exercise_therapy"), TreatmentExerciseTherapy)
	assert.Equal(t, TreatmentType("electrotherapy"), TreatmentElectrotherapy)
	assert.Equal(t, TreatmentType("hydrotherapy"), TreatmentHydrotherapy)
	assert.Equal(t, TreatmentType("dry_needling"), TreatmentDryNeedling)
	assert.Equal(t, TreatmentType("vestibular"), TreatmentVestibular)

	// Body regions
	assert.Equal(t, BodyRegion("cervical_spine"), RegionCervicalSpine)
	assert.Equal(t, BodyRegion("lumbar_spine"), RegionLumbarSpine)
	assert.Equal(t, BodyRegion("shoulder"), RegionShoulder)
	assert.Equal(t, BodyRegion("knee"), RegionKnee)
	assert.Equal(t, BodyRegion("ankle_foot"), RegionAnkleFoot)
	assert.Equal(t, BodyRegion("multiple"), RegionMultiple)

	// Plan statuses
	assert.Equal(t, PlanStatus("draft"), PlanStatusDraft)
	assert.Equal(t, PlanStatus("active"), PlanStatusActive)
	assert.Equal(t, PlanStatus("completed"), PlanStatusCompleted)
	assert.Equal(t, PlanStatus("on_hold"), PlanStatusOnHold)

	// Goal statuses
	assert.Equal(t, GoalStatus("achieved"), GoalStatusAchieved)
	assert.Equal(t, GoalStatus("not_achieved"), GoalStatusNotAchieved)
	assert.Equal(t, GoalStatus("modified"), GoalStatusModified)

	// Intervention statuses
	assert.Equal(t, InterventionStatus("planned"), InterventionPlanned)
	assert.Equal(t, InterventionStatus("cancelled"), InterventionCancelled)
}
