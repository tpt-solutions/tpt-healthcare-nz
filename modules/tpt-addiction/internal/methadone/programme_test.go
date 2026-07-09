package methadone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTakeHomeDays(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 0},
		{2, 1},
		{3, 3},
		{4, 5},
		{5, 7},
		{0, 0},
		{6, 0},
		{-1, 0},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.want, TakeHomeDays(tt.level))
		})
	}
}

func TestProgrammePhases(t *testing.T) {
	assert.Equal(t, ProgrammePhase("induction"), PhaseInduction)
	assert.Equal(t, ProgrammePhase("stabilisation"), PhaseStabilisation)
	assert.Equal(t, ProgrammePhase("maintenance"), PhaseMaintenance)
	assert.Equal(t, ProgrammePhase("tapering"), PhaseTapering)
	assert.Equal(t, ProgrammePhase("discharged"), PhaseDischarged)
}

func TestProgrammeStruct(t *testing.T) {
	now := time.Now()
	end := now.AddDate(0, 6, 0)
	target := 80.0
	p := Programme{
		ID:               "prog-1",
		PatientNHI:       "ABC1235",
		ClinicianID:      "CPN123",
		PracticeID:       "PRAC001",
		StartDate:        now,
		EndDate:          &end,
		Phase:            PhaseStabilisation,
		SubstancePrimary: "heroin",
		InitialDoseMg:    30.0,
		CurrentDoseMg:    60.0,
		TargetDoseMg:     &target,
		TakeHomeLevel:    3,
		TakeHomeMaxDays:  3,
		Pregnancy:        false,
		NextReviewDate:   now.AddDate(0, 1, 0),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	assert.Equal(t, "prog-1", p.ID)
	assert.Equal(t, "ABC1235", p.PatientNHI)
	assert.Equal(t, PhaseStabilisation, p.Phase)
	assert.Equal(t, "heroin", p.SubstancePrimary)
	assert.Equal(t, 30.0, p.InitialDoseMg)
	assert.Equal(t, 60.0, p.CurrentDoseMg)
	assert.Equal(t, &target, p.TargetDoseMg)
	assert.Equal(t, 3, p.TakeHomeLevel)
	assert.False(t, p.Pregnancy)
}

func TestDoseRecordStruct(t *testing.T) {
	now := time.Now()
	d := DoseRecord{
		ID:              "dose-1",
		ProgrammeID:     "prog-1",
		AdministeredAt:  now,
		DoseMg:          60.0,
		Formulation:     "liquid",
		WitnessedBy:     "Nurse Jane",
		DispensedBy:     "Pharmacist Bob",
		PharmacistCheck: true,
		Status:          "administered",
		TakeHome:        false,
		CreatedAt:       now,
	}
	assert.Equal(t, "dose-1", d.ID)
	assert.Equal(t, 60.0, d.DoseMg)
	assert.Equal(t, "liquid", d.Formulation)
	assert.True(t, d.PharmacistCheck)
	assert.False(t, d.TakeHome)
	assert.Equal(t, "administered", d.Status)
}

func TestTakeHomeApprovalStruct(t *testing.T) {
	now := time.Now()
	expires := now.AddDate(0, 3, 0)
	ta := TakeHomeApproval{
		ID:          "tha-1",
		ProgrammeID: "prog-1",
		ApprovedBy:  "CPN123",
		ApprovedAt:  now,
		Level:       3,
		MaxDays:     3,
		ExpiresAt:   &expires,
		CreatedAt:   now,
	}
	assert.Equal(t, "tha-1", ta.ID)
	assert.Equal(t, 3, ta.Level)
	assert.Equal(t, 3, ta.MaxDays)
	assert.NotNil(t, ta.ExpiresAt)
}

func TestUrineScreenStruct(t *testing.T) {
	now := time.Now()
	level := "positive"
	us := UrineScreen{
		ID:          "us-1",
		ProgrammeID: "prog-1",
		CollectedAt: now,
		CollectedBy: "Nurse Jane",
		Results: []DrugResult{
			{DrugName: "methadone", Detected: true, Level: &level, Expected: true},
			{DrugName: "cannabis", Detected: true, Expected: false},
			{DrugName: "amphetamines", Detected: false, Expected: false},
		},
		MSSAResult:    "conforming",
		CreatedAt:     now,
	}
	assert.Equal(t, "us-1", us.ID)
	assert.Len(t, us.Results, 3)
	assert.Equal(t, "methadone", us.Results[0].DrugName)
	assert.True(t, us.Results[0].Detected)
	assert.True(t, us.Results[0].Expected)
	assert.False(t, us.Results[2].Detected)
	assert.Equal(t, "conforming", us.MSSAResult)
}

func TestTakeHomeDaysNZMSSALevels(t *testing.T) {
	// Verify the NZ MSSA take-home schedule
	// Level 1: daily supervised (0 days)
	// Level 2: 1 day take-home
	// Level 3: 3 days take-home
	// Level 4: 5 days take-home
	// Level 5: 7 days take-home (maximum)
	assert.Equal(t, 0, TakeHomeDays(1))
	assert.Equal(t, 1, TakeHomeDays(2))
	assert.Equal(t, 3, TakeHomeDays(3))
	assert.Equal(t, 5, TakeHomeDays(4))
	assert.Equal(t, 7, TakeHomeDays(5))
}
