package homevisit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ABC1235 passes old-format NHI Luhn checksum.
const validNHI = "ABC1235"

func TestHomeVisitValidate(t *testing.T) {
	valid := func() *HomeVisit {
		return &HomeVisit{
			PatientNHI:    validNHI,
			ClinicianID:   "CPN123",
			PracticeID:    "PRAC001",
			ScheduledDate: 1700000000000,
			Address:       "123 Main St, Auckland",
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
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

	t.Run("missing scheduled date", func(t *testing.T) {
		v := valid()
		v.ScheduledDate = 0
		assert.Error(t, v.Validate())
	})

	t.Run("missing address", func(t *testing.T) {
		v := valid()
		v.Address = ""
		assert.Error(t, v.Validate())
	})

	t.Run("missing practice ID", func(t *testing.T) {
		v := valid()
		v.PracticeID = ""
		assert.Error(t, v.Validate())
	})
}

func TestNewHomeVisit(t *testing.T) {
	v := NewHomeVisit()
	assert.Equal(t, VisitScheduled, v.Status)
	assert.Equal(t, PriorityRoutine, v.Priority)
	assert.Equal(t, 30, v.EstimatedDuration)
	assert.NotZero(t, v.CreatedAt)
	assert.NotZero(t, v.UpdatedAt)
	assert.Equal(t, v.CreatedAt, v.UpdatedAt)
}

func TestHomeVisitNoteValidate(t *testing.T) {
	valid := func() *HomeVisitNote {
		return &HomeVisitNote{
			HomeVisitID: "visit-1",
			PatientNHI:  validNHI,
			Narrative:   "Patient reported improvement in wound healing.",
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing visit ID", func(t *testing.T) {
		n := valid()
		n.HomeVisitID = ""
		assert.Error(t, n.Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		n := valid()
		n.PatientNHI = ""
		assert.Error(t, n.Validate())
	})

	t.Run("invalid NHI", func(t *testing.T) {
		n := valid()
		n.PatientNHI = "ZZZ9999"
		assert.Error(t, n.Validate())
	})

	t.Run("missing narrative", func(t *testing.T) {
		n := valid()
		n.Narrative = ""
		assert.Error(t, n.Validate())
	})
}

func TestNewHomeVisitNote(t *testing.T) {
	n := NewHomeVisitNote()
	assert.NotZero(t, n.CreatedAt)
	assert.NotZero(t, n.UpdatedAt)
}

func TestSafetyCheckValidate(t *testing.T) {
	valid := func() *SafetyCheck {
		return &SafetyCheck{
			HomeVisitID: "visit-1",
			PatientNHI:  validNHI,
			CheckedBy:   "RN001",
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing visit ID", func(t *testing.T) {
		s := valid()
		s.HomeVisitID = ""
		assert.Error(t, s.Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		s := valid()
		s.PatientNHI = ""
		assert.Error(t, s.Validate())
	})

	t.Run("invalid NHI", func(t *testing.T) {
		s := valid()
		s.PatientNHI = "ZZZ9999"
		assert.Error(t, s.Validate())
	})

	t.Run("missing checked by", func(t *testing.T) {
		s := valid()
		s.CheckedBy = ""
		assert.Error(t, s.Validate())
	})
}

func TestNewSafetyCheck(t *testing.T) {
	s := NewSafetyCheck()
	assert.True(t, s.FireSafetyOK)
	assert.True(t, s.SmokeAlarmsOK)
	assert.True(t, s.MedicationStorageOK)
	assert.NotZero(t, s.CheckDate)
	assert.NotZero(t, s.CreatedAt)
}

func TestEquipmentCheckValidate(t *testing.T) {
	valid := func() *EquipmentCheck {
		return &EquipmentCheck{
			HomeVisitID:   "visit-1",
			EquipmentName: "Oxygen concentrator",
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing visit ID", func(t *testing.T) {
		e := valid()
		e.HomeVisitID = ""
		assert.Error(t, e.Validate())
	})

	t.Run("missing equipment name", func(t *testing.T) {
		e := valid()
		e.EquipmentName = ""
		assert.Error(t, e.Validate())
	})
}

func TestNewEquipmentCheck(t *testing.T) {
	e := NewEquipmentCheck()
	assert.Equal(t, EquipmentFunctioning, e.Status)
	assert.NotZero(t, e.CheckedAt)
	assert.NotZero(t, e.CreatedAt)
	assert.NotZero(t, e.UpdatedAt)
}

func TestHomeVisitConstants(t *testing.T) {
	// Visit types
	assert.Equal(t, VisitType("wound_care"), VisitWoundCare)
	assert.Equal(t, VisitType("medication_review"), VisitMedicationReview)
	assert.Equal(t, VisitType("post_acute"), VisitPostAcute)
	assert.Equal(t, VisitType("palliative"), VisitPalliative)
	assert.Equal(t, VisitType("assessment"), VisitAssessment)
	assert.Equal(t, VisitType("follow_up"), VisitFollowUp)
	assert.Equal(t, VisitType("diabetes_care"), VisitDiabetesCare)
	assert.Equal(t, VisitType("respiratory"), VisitRespiratory)
	assert.Equal(t, VisitType("rehabilitation"), VisitRehabilitation)
	assert.Equal(t, VisitType("postnatal"), VisitPostnatal)

	// Visit statuses
	assert.Equal(t, VisitStatus("scheduled"), VisitScheduled)
	assert.Equal(t, VisitStatus("in_transit"), VisitInTransit)
	assert.Equal(t, VisitStatus("arrived"), VisitArrived)
	assert.Equal(t, VisitStatus("in_progress"), VisitInProgress)
	assert.Equal(t, VisitStatus("completed"), VisitCompleted)
	assert.Equal(t, VisitStatus("cancelled"), VisitCancelled)
	assert.Equal(t, VisitStatus("rescheduled"), VisitRescheduled)
	assert.Equal(t, VisitStatus("no_show"), VisitNoShow)

	// Priorities
	assert.Equal(t, Priority("urgent"), PriorityUrgent)
	assert.Equal(t, Priority("high"), PriorityHigh)
	assert.Equal(t, Priority("routine"), PriorityRoutine)
	assert.Equal(t, Priority("low"), PriorityLow)

	// Transport modes
	assert.Equal(t, TransportMode("car"), TransportCar)
	assert.Equal(t, TransportMode("bike"), TransportBike)
	assert.Equal(t, TransportMode("public_transport"), TransportPublic)
	assert.Equal(t, TransportMode("walking"), TransportWalking)
	assert.Equal(t, TransportMode("company_vehicle"), TransportCompanyVehicle)

	// Note types
	assert.Equal(t, NoteType("subjective"), NoteSubjective)
	assert.Equal(t, NoteType("objective"), NoteObjective)
	assert.Equal(t, NoteType("assessment"), NoteAssessment)
	assert.Equal(t, NoteType("plan"), NotePlan)
	assert.Equal(t, NoteType("supplementary"), NoteSupplementary)

	// Equipment statuses
	assert.Equal(t, EquipmentStatus("functioning"), EquipmentFunctioning)
	assert.Equal(t, EquipmentStatus("needs_service"), EquipmentNeedsService)
	assert.Equal(t, EquipmentStatus("broken"), EquipmentBroken)
	assert.Equal(t, EquipmentStatus("missing"), EquipmentMissing)
}

func TestOutcomeCategories(t *testing.T) {
	assert.Equal(t, OutcomeCategory("wound_healing"), OutcomeWoundHealing)
	assert.Equal(t, OutcomeCategory("medication_adherence"), OutcomeMedicationAdherence)
	assert.Equal(t, OutcomeCategory("functional_improvement"), OutcomeFunctionalImprovement)
	assert.Equal(t, OutcomeCategory("safety"), OutcomeSafety)
	assert.Equal(t, OutcomeCategory("referral"), OutcomeReferral)
	assert.Equal(t, OutcomeCategory("education"), OutcomeEducation)
}
