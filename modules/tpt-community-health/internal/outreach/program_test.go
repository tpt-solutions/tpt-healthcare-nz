package outreach

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ABC1235 passes old-format NHI Luhn checksum.
const validNHI = "ABC1235"

func TestProgramValidate(t *testing.T) {
	valid := func() *Program {
		return &Program{
			PracticeID:  "PRAC001",
			ProgramName: "Mobile Health Clinic",
			ProgramType: ProgramMobileClinic,
			StartDate:   1700000000000,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing practice ID", func(t *testing.T) {
		p := valid()
		p.PracticeID = ""
		assert.Error(t, p.Validate())
	})

	t.Run("missing program name", func(t *testing.T) {
		p := valid()
		p.ProgramName = ""
		assert.Error(t, p.Validate())
	})

	t.Run("missing program type", func(t *testing.T) {
		p := valid()
		p.ProgramType = ""
		assert.Error(t, p.Validate())
	})

	t.Run("missing start date", func(t *testing.T) {
		p := valid()
		p.StartDate = 0
		assert.Error(t, p.Validate())
	})
}

func TestNewProgram(t *testing.T) {
	p := NewProgram()
	assert.Equal(t, ProgramActive, p.Status)
	assert.NotZero(t, p.CreatedAt)
	assert.NotZero(t, p.UpdatedAt)
	assert.Equal(t, p.CreatedAt, p.UpdatedAt)
}

func TestEventValidate(t *testing.T) {
	valid := func() *Event {
		return &Event{
			ProgramID:       "prog-1",
			EventName:       "Free Health Screening",
			EventType:       EventScreening,
			ScheduledDate:   1700000000000,
			LocationAddress: "Community Centre, Wellington",
			Clinicians:      []string{"CPN123"},
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing program ID", func(t *testing.T) {
		e := valid()
		e.ProgramID = ""
		assert.Error(t, e.Validate())
	})

	t.Run("missing event name", func(t *testing.T) {
		e := valid()
		e.EventName = ""
		assert.Error(t, e.Validate())
	})

	t.Run("missing event type", func(t *testing.T) {
		e := valid()
		e.EventType = ""
		assert.Error(t, e.Validate())
	})

	t.Run("missing scheduled date", func(t *testing.T) {
		e := valid()
		e.ScheduledDate = 0
		assert.Error(t, e.Validate())
	})

	t.Run("missing location", func(t *testing.T) {
		e := valid()
		e.LocationAddress = ""
		assert.Error(t, e.Validate())
	})

	t.Run("no clinicians", func(t *testing.T) {
		e := valid()
		e.Clinicians = nil
		assert.Error(t, e.Validate())
	})
}

func TestNewEvent(t *testing.T) {
	e := NewEvent()
	assert.Equal(t, EventPlanned, e.Status)
	assert.Equal(t, 120, e.EstimatedDuration)
	assert.NotNil(t, e.Clinicians)
	assert.Empty(t, e.Clinicians)
	assert.NotZero(t, e.CreatedAt)
	assert.NotZero(t, e.UpdatedAt)
}

func TestAttendeeValidate(t *testing.T) {
	valid := func() *Attendee {
		return &Attendee{
			EventID:      "event-1",
			AttendeeName: "Jane Smith",
			PatientNHI:   validNHI,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing event ID", func(t *testing.T) {
		a := valid()
		a.EventID = ""
		assert.Error(t, a.Validate())
	})

	t.Run("missing attendee name", func(t *testing.T) {
		a := valid()
		a.AttendeeName = ""
		assert.Error(t, a.Validate())
	})

	t.Run("invalid NHI", func(t *testing.T) {
		a := valid()
		a.PatientNHI = "ZZZ9999"
		assert.Error(t, a.Validate())
	})

	t.Run("empty NHI is ok for community member", func(t *testing.T) {
		a := valid()
		a.PatientNHI = ""
		assert.NoError(t, a.Validate())
	})
}

func TestNewAttendee(t *testing.T) {
	a := NewAttendee()
	assert.Equal(t, AttendeePatient, a.AttendeeType)
	assert.False(t, a.ConsentGiven)
	assert.NotZero(t, a.CreatedAt)
	assert.NotZero(t, a.UpdatedAt)
}

func TestReferralValidate(t *testing.T) {
	valid := func() *Referral {
		return &Referral{
			EventID:        "event-1",
			PatientNHI:     validNHI,
			ReferredBy:     "Dr. Smith",
			ReferralType:   ReferralGP,
			ReferralReason: "Elevated blood pressure",
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing event ID", func(t *testing.T) {
		r := valid()
		r.EventID = ""
		assert.Error(t, r.Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		r := valid()
		r.PatientNHI = ""
		assert.Error(t, r.Validate())
	})

	t.Run("invalid NHI", func(t *testing.T) {
		r := valid()
		r.PatientNHI = "ZZZ9999"
		assert.Error(t, r.Validate())
	})

	t.Run("missing referred by", func(t *testing.T) {
		r := valid()
		r.ReferredBy = ""
		assert.Error(t, r.Validate())
	})

	t.Run("missing referral type", func(t *testing.T) {
		r := valid()
		r.ReferralType = ""
		assert.Error(t, r.Validate())
	})

	t.Run("missing referral reason", func(t *testing.T) {
		r := valid()
		r.ReferralReason = ""
		assert.Error(t, r.Validate())
	})
}

func TestNewReferral(t *testing.T) {
	r := NewReferral()
	assert.Equal(t, "routine", r.Urgency)
	assert.Equal(t, "pending", r.Status)
	assert.NotZero(t, r.ReferralDate)
	assert.NotZero(t, r.CreatedAt)
	assert.NotZero(t, r.UpdatedAt)
}

func TestScreeningValidate(t *testing.T) {
	valid := func() *Screening {
		return &Screening{
			EventID:       "event-1",
			PatientNHI:    validNHI,
			ClinicianID:   "CPN123",
			ScreeningType: ScreeningBloodPressure,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing event ID", func(t *testing.T) {
		s := valid()
		s.EventID = ""
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

	t.Run("missing clinician ID", func(t *testing.T) {
		s := valid()
		s.ClinicianID = ""
		assert.Error(t, s.Validate())
	})

	t.Run("missing screening type", func(t *testing.T) {
		s := valid()
		s.ScreeningType = ""
		assert.Error(t, s.Validate())
	})
}

func TestNewScreening(t *testing.T) {
	s := NewScreening()
	assert.NotZero(t, s.ScreeningDate)
	assert.NotZero(t, s.CreatedAt)
	assert.NotZero(t, s.UpdatedAt)
}

func TestOutreachConstants(t *testing.T) {
	// Program types
	assert.Equal(t, ProgramType("mobile_clinic"), ProgramMobileClinic)
	assert.Equal(t, ProgramType("vaccination"), ProgramVaccination)
	assert.Equal(t, ProgramType("screening"), ProgramScreening)
	assert.Equal(t, ProgramType("health_promotion"), ProgramHealthPromotion)
	assert.Equal(t, ProgramType("chronic_disease_support"), ProgramChronicDisease)

	// Program statuses
	assert.Equal(t, ProgramStatus("active"), ProgramActive)
	assert.Equal(t, ProgramStatus("paused"), ProgramPaused)
	assert.Equal(t, ProgramStatus("completed"), ProgramCompleted)
	assert.Equal(t, ProgramStatus("discontinued"), ProgramDiscontinued)

	// Event types
	assert.Equal(t, EventType("clinic"), EventClinic)
	assert.Equal(t, EventType("screening"), EventScreening)
	assert.Equal(t, EventType("education"), EventEducation)
	assert.Equal(t, EventType("vaccination"), EventVaccination)

	// Event statuses
	assert.Equal(t, EventStatus("planned"), EventPlanned)
	assert.Equal(t, EventStatus("confirmed"), EventConfirmed)
	assert.Equal(t, EventStatus("in_progress"), EventInProgress)
	assert.Equal(t, EventStatus("completed"), EventCompleted)
	assert.Equal(t, EventStatus("cancelled"), EventCancelled)

	// Attendee types
	assert.Equal(t, AttendeeType("patient"), AttendeePatient)
	assert.Equal(t, AttendeeType("carer"), AttendeeCarer)
	assert.Equal(t, AttendeeType("community_member"), AttendeeCommunityMember)
	assert.Equal(t, AttendeeType("staff"), AttendeeStaff)

	// Referral types
	assert.Equal(t, ReferralType("gp"), ReferralGP)
	assert.Equal(t, ReferralType("specialist"), ReferralSpecialist)
	assert.Equal(t, ReferralType("mental_health"), ReferralMentalHealth)
	assert.Equal(t, ReferralType("social_services"), ReferralSocial)
	assert.Equal(t, ReferralType("housing"), ReferralHousing)

	// Screening types
	assert.Equal(t, ScreeningType("blood_pressure"), ScreeningBloodPressure)
	assert.Equal(t, ScreeningType("diabetes"), ScreeningDiabetes)
	assert.Equal(t, ScreeningType("cervical"), ScreeningCervical)
	assert.Equal(t, ScreeningType("bowel"), ScreeningBowel)
	assert.Equal(t, ScreeningType("hearing"), ScreeningHearing)
	assert.Equal(t, ScreeningType("vision"), ScreeningVision)

	// Result categories
	assert.Equal(t, ResultCategory("normal"), ResultNormal)
	assert.Equal(t, ResultCategory("abnormal"), ResultAbnormal)
	assert.Equal(t, ResultCategory("borderline"), ResultBorderline)
	assert.Equal(t, ResultCategory("inconclusive"), ResultInconclusive)
}
