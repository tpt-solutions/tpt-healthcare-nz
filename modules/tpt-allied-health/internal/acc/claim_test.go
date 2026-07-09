package acc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ABC1235 passes the old-format NHI Luhn checksum: A=1,B=2,C=3,1=1,2=2,3=3
// sum = 1*7+2*6+3*5+1*4+2*3+3*2 = 50, 50%11=6, 11-6=5, check digit=5.
const validNHI = "ABC1235"

func TestClaimValidate(t *testing.T) {
	valid := func() *Claim {
		return &Claim{
			PatientNHI:  validNHI,
			ClinicianID: "CPN123",
			ClaimType:   ClaimTypePhysiotherapy,
			ACCNumber:   "ACC12345",
			InjuryDate:  1700000000000,
			Diagnosis:   "ACL injury",
			BodyRegion:  "knee",
			StartDate:   1700000000000,
			ExpiryDate:  1731536000000,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		c := valid()
		c.PatientNHI = ""
		assert.Error(t, c.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		c := valid()
		c.ClinicianID = ""
		assert.Error(t, c.Validate())
	})

	t.Run("missing claim type", func(t *testing.T) {
		c := valid()
		c.ClaimType = ""
		assert.Error(t, c.Validate())
	})

	t.Run("missing ACC number", func(t *testing.T) {
		c := valid()
		c.ACCNumber = ""
		assert.Error(t, c.Validate())
	})

	t.Run("missing injury date", func(t *testing.T) {
		c := valid()
		c.InjuryDate = 0
		assert.Error(t, c.Validate())
	})

	t.Run("missing diagnosis", func(t *testing.T) {
		c := valid()
		c.Diagnosis = ""
		assert.Error(t, c.Validate())
	})

	t.Run("missing body region", func(t *testing.T) {
		c := valid()
		c.BodyRegion = ""
		assert.Error(t, c.Validate())
	})

	t.Run("missing start date", func(t *testing.T) {
		c := valid()
		c.StartDate = 0
		assert.Error(t, c.Validate())
	})

	t.Run("missing expiry date", func(t *testing.T) {
		c := valid()
		c.ExpiryDate = 0
		assert.Error(t, c.Validate())
	})

	t.Run("invalid NHI checksum", func(t *testing.T) {
		c := valid()
		c.PatientNHI = "ZZZ9999"
		assert.Error(t, c.Validate())
	})

	t.Run("all claim types accepted", func(t *testing.T) {
		types := []ClaimType{
			ClaimTypePhysiotherapy,
			ClaimTypeOccupationalTherapy,
			ClaimTypeSpeechLanguage,
			ClaimTypePodiatry,
		}
		for _, ct := range types {
			c := valid()
			c.ClaimType = ct
			assert.NoError(t, c.Validate(), "claim type %s", ct)
		}
	})
}

func TestCanAddSession(t *testing.T) {
	t.Run("not accepted", func(t *testing.T) {
		c := &Claim{Status: ClaimStatusDraft}
		assert.False(t, c.CanAddSession())
	})

	t.Run("all sessions used", func(t *testing.T) {
		c := &Claim{Status: ClaimStatusAccepted, ApprovedSessions: 6, UsedSessions: 6}
		assert.False(t, c.CanAddSession())
	})

	t.Run("expired", func(t *testing.T) {
		c := &Claim{
			Status:           ClaimStatusAccepted,
			ApprovedSessions: 6,
			UsedSessions:     3,
			ExpiryDate:       1000000000000,
		}
		assert.False(t, c.CanAddSession())
	})

	t.Run("can add", func(t *testing.T) {
		c := &Claim{
			Status:           ClaimStatusAccepted,
			ApprovedSessions: 6,
			UsedSessions:     3,
			ExpiryDate:       2000000000000,
		}
		assert.True(t, c.CanAddSession())
	})

	t.Run("zero expiry means no expiry check", func(t *testing.T) {
		c := &Claim{
			Status:           ClaimStatusAccepted,
			ApprovedSessions: 6,
			UsedSessions:     0,
			ExpiryDate:       0,
		}
		assert.True(t, c.CanAddSession())
	})

	t.Run("submitted status cannot add", func(t *testing.T) {
		c := &Claim{
			Status:           ClaimStatusSubmitted,
			ApprovedSessions: 6,
			UsedSessions:     0,
			ExpiryDate:       2000000000000,
		}
		assert.False(t, c.CanAddSession())
	})

	t.Run("declined status cannot add", func(t *testing.T) {
		c := &Claim{
			Status:           ClaimStatusDeclined,
			ApprovedSessions: 6,
			UsedSessions:     0,
			ExpiryDate:       2000000000000,
		}
		assert.False(t, c.CanAddSession())
	})

	t.Run("one session remaining", func(t *testing.T) {
		c := &Claim{
			Status:           ClaimStatusAccepted,
			ApprovedSessions: 6,
			UsedSessions:     5,
			ExpiryDate:       2000000000000,
		}
		assert.True(t, c.CanAddSession())
	})
}

func TestAddSession(t *testing.T) {
	c := &Claim{}
	c.AddSession()
	assert.Equal(t, 1, c.UsedSessions)
	assert.NotZero(t, c.LastTreatmentDate)

	c.AddSession()
	assert.Equal(t, 2, c.UsedSessions)
	assert.NotZero(t, c.UpdatedAt)
}

func TestNewClaim(t *testing.T) {
	c := NewClaim()
	assert.Equal(t, ClaimStatusDraft, c.Status)
	assert.NotZero(t, c.ClaimDate)
	assert.NotZero(t, c.CreatedAt)
	assert.NotZero(t, c.UpdatedAt)
	assert.Equal(t, c.CreatedAt, c.UpdatedAt)
}

func TestTreatmentSessionValidate(t *testing.T) {
	valid := func() *TreatmentSession {
		return &TreatmentSession{
			ClaimID:         "claim-1",
			PatientNHI:      validNHI,
			ClinicianID:     "CPN123",
			SessionDate:     1700000000000,
			ChargeCode:      "PHY001",
			DurationMinutes: 45,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing claim ID", func(t *testing.T) {
		s := valid()
		s.ClaimID = ""
		assert.Error(t, s.Validate())
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

	t.Run("missing charge code", func(t *testing.T) {
		s := valid()
		s.ChargeCode = ""
		assert.Error(t, s.Validate())
	})

	t.Run("unknown charge code", func(t *testing.T) {
		s := valid()
		s.ChargeCode = "XXXXX"
		assert.Error(t, s.Validate())
	})

	t.Run("zero duration", func(t *testing.T) {
		s := valid()
		s.DurationMinutes = 0
		assert.Error(t, s.Validate())
	})

	t.Run("negative duration", func(t *testing.T) {
		s := valid()
		s.DurationMinutes = -10
		assert.Error(t, s.Validate())
	})

	t.Run("valid with OT code", func(t *testing.T) {
		s := valid()
		s.ChargeCode = "OT001"
		assert.NoError(t, s.Validate())
	})

	t.Run("valid with SLT code", func(t *testing.T) {
		s := valid()
		s.ChargeCode = "SLT001"
		assert.NoError(t, s.Validate())
	})

	t.Run("valid with podiatry code", func(t *testing.T) {
		s := valid()
		s.ChargeCode = "POD001"
		assert.NoError(t, s.Validate())
	})
}

func TestReviewReportValidate(t *testing.T) {
	valid := func() *ReviewReport {
		return &ReviewReport{
			ClaimID:        "claim-1",
			PatientNHI:     validNHI,
			ClinicianID:    "CPN123",
			ReportDate:     1700000000000,
			ReportType:     ReviewTypeProgress,
			Recommendation: RecommendContinue,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing claim ID", func(t *testing.T) {
		r := valid()
		r.ClaimID = ""
		assert.Error(t, r.Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		r := valid()
		r.PatientNHI = ""
		assert.Error(t, r.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		r := valid()
		r.ClinicianID = ""
		assert.Error(t, r.Validate())
	})

	t.Run("missing report date", func(t *testing.T) {
		r := valid()
		r.ReportDate = 0
		assert.Error(t, r.Validate())
	})

	t.Run("missing report type", func(t *testing.T) {
		r := valid()
		r.ReportType = ""
		assert.Error(t, r.Validate())
	})

	t.Run("missing recommendation", func(t *testing.T) {
		r := valid()
		r.Recommendation = ""
		assert.Error(t, r.Validate())
	})

	t.Run("invalid NHI", func(t *testing.T) {
		r := valid()
		r.PatientNHI = "ZZZ9999"
		assert.Error(t, r.Validate())
	})

	t.Run("all review types accepted", func(t *testing.T) {
		types := []ReviewType{
			ReviewTypeInitial,
			ReviewTypeProgress,
			ReviewTypeDischarge,
			ReviewTypeExtension,
			ReviewTypeReassessment,
		}
		for _, rt := range types {
			r := valid()
			r.ReportType = rt
			assert.NoError(t, r.Validate(), "review type %s", rt)
		}
	})

	t.Run("all recommendations accepted", func(t *testing.T) {
		recs := []ReviewRecommendation{
			RecommendContinue,
			RecommendExtend,
			RecommendDischarge,
			RecommendRefer,
			RecommendInvestigate,
		}
		for _, rec := range recs {
			r := valid()
			r.Recommendation = rec
			assert.NoError(t, r.Validate(), "recommendation %s", rec)
		}
	})
}

func TestGetChargeCodesByProfession(t *testing.T) {
	tests := []struct {
		profession string
		minCount   int
	}{
		{"physiotherapy", 6},
		{"occupational_therapy", 6},
		{"speech_language_therapy", 5},
		{"podiatry", 7},
		{"nonexistent", 0},
	}
	for _, tt := range tests {
		t.Run(tt.profession, func(t *testing.T) {
			codes := GetChargeCodesByProfession(tt.profession)
			assert.GreaterOrEqual(t, len(codes), tt.minCount)
			for _, c := range codes {
				assert.Equal(t, tt.profession, c.Profession)
				assert.True(t, c.Active)
				assert.NotEmpty(t, c.Code)
				assert.NotEmpty(t, c.Description)
				assert.Greater(t, c.Rate, 0.0)
			}
		})
	}
}

func TestGetChargeCodeByCode(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		c := GetChargeCodeByCode("PHY001")
		assert.NotNil(t, c)
		assert.Equal(t, "PHY001", c.Code)
		assert.Equal(t, 85.00, c.Rate)
		assert.Equal(t, "physiotherapy", c.Profession)
		assert.Equal(t, "session", c.Unit)
		assert.True(t, c.Active)
	})

	t.Run("not found", func(t *testing.T) {
		assert.Nil(t, GetChargeCodeByCode("XXXXX"))
	})

	t.Run("all profession codes exist", func(t *testing.T) {
		physio := []string{"PHY001", "PHY002", "PHY003", "PHY004", "PHY005", "PHY006"}
		for _, code := range physio {
			c := GetChargeCodeByCode(code)
			assert.NotNil(t, c, "code %s", code)
		}
		ot := []string{"OT001", "OT002", "OT003", "OT004", "OT005", "OT006"}
		for _, code := range ot {
			c := GetChargeCodeByCode(code)
			assert.NotNil(t, c, "code %s", code)
		}
		slt := []string{"SLT001", "SLT002", "SLT003", "SLT004", "SLT005"}
		for _, code := range slt {
			c := GetChargeCodeByCode(code)
			assert.NotNil(t, c, "code %s", code)
		}
		pod := []string{"POD001", "POD002", "POD003", "POD004", "POD005", "POD006", "POD007"}
		for _, code := range pod {
			c := GetChargeCodeByCode(code)
			assert.NotNil(t, c, "code %s", code)
		}
	})
}

func TestNewTreatmentSession(t *testing.T) {
	s := NewTreatmentSession()
	assert.Equal(t, TreatmentStatusPlanned, s.Status)
	assert.NotZero(t, s.CreatedAt)
	assert.NotZero(t, s.UpdatedAt)
	assert.Equal(t, s.CreatedAt, s.UpdatedAt)
	assert.NotNil(t, s.OutcomeMeasures)
	assert.Empty(t, s.OutcomeMeasures)
}

func TestNewReviewReport(t *testing.T) {
	r := NewReviewReport()
	assert.Equal(t, ReviewStatusDraft, r.Status)
	assert.NotZero(t, r.CreatedAt)
	assert.NotZero(t, r.UpdatedAt)
	assert.Equal(t, r.CreatedAt, r.UpdatedAt)
	assert.NotNil(t, r.GoalsAchieved)
	assert.Empty(t, r.GoalsAchieved)
	assert.NotNil(t, r.GoalsOngoing)
	assert.Empty(t, r.GoalsOngoing)
	assert.NotNil(t, r.GoalsNotAchieved)
	assert.Empty(t, r.GoalsNotAchieved)
	assert.NotNil(t, r.OutcomeMeasures)
	assert.Empty(t, r.OutcomeMeasures)
}

func TestAlliedHealthConstants(t *testing.T) {
	// Claim types
	assert.Equal(t, ClaimType("physiotherapy"), ClaimTypePhysiotherapy)
	assert.Equal(t, ClaimType("occupational_therapy"), ClaimTypeOccupationalTherapy)
	assert.Equal(t, ClaimType("speech_language_therapy"), ClaimTypeSpeechLanguage)
	assert.Equal(t, ClaimType("podiatry"), ClaimTypePodiatry)

	// Claim statuses
	assert.Equal(t, ClaimStatus("draft"), ClaimStatusDraft)
	assert.Equal(t, ClaimStatus("submitted"), ClaimStatusSubmitted)
	assert.Equal(t, ClaimStatus("accepted"), ClaimStatusAccepted)
	assert.Equal(t, ClaimStatus("declined"), ClaimStatusDeclined)
	assert.Equal(t, ClaimStatus("under_review"), ClaimStatusUnderReview)
	assert.Equal(t, ClaimStatus("closed"), ClaimStatusClosed)
	assert.Equal(t, ClaimStatus("expired"), ClaimStatusExpired)

	// Treatment statuses
	assert.Equal(t, TreatmentStatus("planned"), TreatmentStatusPlanned)
	assert.Equal(t, TreatmentStatus("active"), TreatmentStatusActive)
	assert.Equal(t, TreatmentStatus("completed"), TreatmentStatusCompleted)
	assert.Equal(t, TreatmentStatus("suspended"), TreatmentStatusSuspended)
	assert.Equal(t, TreatmentStatus("declined"), TreatmentStatusDeclined)

	// Review types
	assert.Equal(t, ReviewType("initial"), ReviewTypeInitial)
	assert.Equal(t, ReviewType("progress"), ReviewTypeProgress)
	assert.Equal(t, ReviewType("discharge"), ReviewTypeDischarge)
	assert.Equal(t, ReviewType("extension"), ReviewTypeExtension)
	assert.Equal(t, ReviewType("reassessment"), ReviewTypeReassessment)

	// Review statuses
	assert.Equal(t, ReviewStatus("draft"), ReviewStatusDraft)
	assert.Equal(t, ReviewStatus("submitted"), ReviewStatusSubmitted)
	assert.Equal(t, ReviewStatus("accepted"), ReviewStatusAccepted)
	assert.Equal(t, ReviewStatus("declined"), ReviewStatusDeclined)
	assert.Equal(t, ReviewStatus("more_info_required"), ReviewStatusMoreInfo)

	// Review recommendations
	assert.Equal(t, ReviewRecommendation("continue"), RecommendContinue)
	assert.Equal(t, ReviewRecommendation("extend"), RecommendExtend)
	assert.Equal(t, ReviewRecommendation("discharge"), RecommendDischarge)
	assert.Equal(t, ReviewRecommendation("refer"), RecommendRefer)
	assert.Equal(t, ReviewRecommendation("investigate"), RecommendInvestigate)
}

func TestStandardChargeCodesCount(t *testing.T) {
	// 6 physio + 6 OT + 5 SLT + 7 podiatry = 24
	assert.Equal(t, 24, len(StandardChargeCodes))
}

func TestOutcomeMeasureStruct(t *testing.T) {
	m := OutcomeMeasure{
		ID:             "om-1",
		Name:           "NDI",
		Domain:         "neck",
		Score:          30.0,
		MaxScore:       50.0,
		Date:           1700000000000,
		Interpretation: "moderate disability",
		CreatedAt:      1700000000000,
	}
	assert.Equal(t, "NDI", m.Name)
	assert.Equal(t, 30.0, m.Score)
	assert.Equal(t, 50.0, m.MaxScore)
}
