package acc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaimValidate(t *testing.T) {
	valid := func() *Claim {
		return &Claim{
			PatientNHI:  "ABC1235",
			ClinicianID: "CPN123",
			Injury:      InjuryDetails{AccidentDate: 1700000000000, InjuryType: "corneal abrasion"},
			Items:       []ClaimItem{{Amount: 100}},
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

	t.Run("missing accident date", func(t *testing.T) {
		c := valid()
		c.Injury.AccidentDate = 0
		assert.Error(t, c.Validate())
	})

	t.Run("missing injury type", func(t *testing.T) {
		c := valid()
		c.Injury.InjuryType = ""
		assert.Error(t, c.Validate())
	})

	t.Run("no items", func(t *testing.T) {
		c := valid()
		c.Items = nil
		assert.Error(t, c.Validate())
	})

	t.Run("valid with all fields", func(t *testing.T) {
		c := valid()
		c.ClaimType = ClaimEyeExam
		c.Status = StatusSubmitted
		c.Provider = ProviderOptometrist
		assert.NoError(t, c.Validate())
	})
}

func TestAddItem(t *testing.T) {
	c := &Claim{}
	c.AddItem(ClaimItem{Amount: 100, GSTAmount: 15})
	assert.Len(t, c.Items, 1)
	assert.Equal(t, 1, c.Items[0].LineNumber)
	assert.Equal(t, 100.0, c.TotalClaimed)
	assert.Equal(t, 15.0, c.GSTTotal)
	assert.Equal(t, 115.0, c.TotalIncGST)

	c.AddItem(ClaimItem{Amount: 200, GSTAmount: 30})
	assert.Len(t, c.Items, 2)
	assert.Equal(t, 2, c.Items[1].LineNumber)
	assert.Equal(t, 300.0, c.TotalClaimed)
	assert.Equal(t, 45.0, c.GSTTotal)
	assert.Equal(t, 345.0, c.TotalIncGST)
}

func TestAddItem_ThreeItems(t *testing.T) {
	c := &Claim{}
	c.AddItem(ClaimItem{Amount: 50, GSTAmount: 7.50})
	c.AddItem(ClaimItem{Amount: 75, GSTAmount: 11.25})
	c.AddItem(ClaimItem{Amount: 100, GSTAmount: 15.00})
	assert.Len(t, c.Items, 3)
	assert.Equal(t, 1, c.Items[0].LineNumber)
	assert.Equal(t, 2, c.Items[1].LineNumber)
	assert.Equal(t, 3, c.Items[2].LineNumber)
	assert.Equal(t, 225.0, c.TotalClaimed)
	assert.Equal(t, 33.75, c.GSTTotal)
	assert.Equal(t, 258.75, c.TotalIncGST)
}

func TestRecalculate_WithAmountPaid(t *testing.T) {
	c := &Claim{AmountPaid: 100}
	c.AddItem(ClaimItem{Amount: 200, GSTAmount: 30})
	assert.Equal(t, 130.0, c.Outstanding)
}

func TestRecalculate_FullyPaid(t *testing.T) {
	c := &Claim{AmountPaid: 115}
	c.AddItem(ClaimItem{Amount: 100, GSTAmount: 15})
	assert.Equal(t, 0.0, c.Outstanding)
}

func TestRecalculate_Overpaid(t *testing.T) {
	c := &Claim{AmountPaid: 200}
	c.AddItem(ClaimItem{Amount: 100, GSTAmount: 15})
	assert.Equal(t, -85.0, c.Outstanding)
}

func TestNewClaim(t *testing.T) {
	c := NewClaim()
	assert.Equal(t, StatusDraft, c.Status)
	assert.NotZero(t, c.CreatedAt)
	assert.NotZero(t, c.UpdatedAt)
	assert.Equal(t, c.CreatedAt, c.UpdatedAt)
}

func TestToFHIRClaim(t *testing.T) {
	c := &Claim{
		ID:          "claim-1",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		ClaimType:   ClaimEyeExam,
		Status:      StatusDraft,
		Items:       []ClaimItem{{LineNumber: 1, Amount: 100, GSTAmount: 15, TotalAmount: 115}},
		CreatedAt:   1700000000000,
		UpdatedAt:   1700000000000,
	}
	c.TotalIncGST = 115.0
	claim := c.ToFHIRClaim()
	assert.Equal(t, "Claim", claim["resourceType"])
	assert.Equal(t, "claim-1", claim["id"])
	assert.Equal(t, "active", claim["status"])
	assert.Equal(t, "claim", claim["use"])

	items := claim["item"].([]map[string]any)
	assert.Len(t, items, 1)

	ext := claim["extension"].([]map[string]any)
	assert.GreaterOrEqual(t, len(ext), 5)

	patient := claim["patient"].(map[string]any)
	assert.Equal(t, "Patient/ABC1235", patient["reference"])

	provider := claim["provider"].(map[string]any)
	assert.Equal(t, "Practitioner/CPN123", provider["reference"])

	total := claim["total"].(map[string]any)
	assert.Equal(t, 115.0, total["value"])
	assert.Equal(t, "NZD", total["currency"])
}

func TestToFHIRClaim_WithOptionalExtensions(t *testing.T) {
	c := &Claim{
		ID:            "claim-2",
		PatientNHI:    "ABC1235",
		ClinicianID:   "CPN123",
		ClaimType:     ClaimSpectacleAfterInjury,
		Status:        StatusSubmitted,
		Provider:      ProviderOphthalmologist,
		Injury:        InjuryDetails{AccidentDate: 1700000000000, InjuryType: "blunt trauma", AccNumber: "ACC999", LodgedBy: "GP"},
		Items:         []ClaimItem{{LineNumber: 1, Amount: 200, GSTAmount: 30, TotalAmount: 230}},
		SubmittedDate: 1700100000000,
		ResponseDate:  1700200000000,
		DeclineReason: "insufficient evidence",
		CreatedAt:     1700000000000,
		UpdatedAt:     1700000000000,
	}
	c.TotalIncGST = 230.0
	claim := c.ToFHIRClaim()
	ext := claim["extension"].([]map[string]any)

	// 5 base + submitted date + response date + decline reason = 8
	assert.Len(t, ext, 8)
}

func TestToFHIRClaim_AccidentDetails(t *testing.T) {
	c := &Claim{
		ID:          "claim-3",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		Injury: InjuryDetails{
			AccidentDate: 1700000000000,
			InjuryType:   "foreign body",
			InjuryCause:  "metal shaving in eye",
		},
		Items:     []ClaimItem{{Amount: 100}},
		CreatedAt: 1700000000000,
		UpdatedAt: 1700000000000,
	}
	claim := c.ToFHIRClaim()
	accident := claim["accident"].(map[string]any)
	assert.NotNil(t, accident["date"])
	accidentType := accident["type"].(map[string]any)
	codings := accidentType["coding"].([]map[string]any)
	assert.Equal(t, "foreign body", codings[0]["code"])
	assert.Equal(t, "metal shaving in eye", accidentType["text"])
}

func TestProcedureDescriptions(t *testing.T) {
	assert.Greater(t, len(ProcedureDescriptions), 0)
	assert.Equal(t, 8, len(ProcedureDescriptions))

	codes := []ProcedureCode{
		ProcComprehensiveExam,
		ProcIntermediateExam,
		ProcVisualField,
		ProcOCT,
		ProcContactLensRemove,
		ProcContactLensPatch,
		ProcSpectacleReplace,
		ProcCLReplace,
	}
	for _, code := range codes {
		desc, ok := ProcedureDescriptions[code]
		assert.True(t, ok, "code %s should have description", code)
		assert.NotEmpty(t, desc)
	}
}

func TestClaimConstants(t *testing.T) {
	assert.Equal(t, ClaimType("eye_examination"), ClaimEyeExam)
	assert.Equal(t, ClaimType("spectacle_after_injury"), ClaimSpectacleAfterInjury)
	assert.Equal(t, ClaimType("contact_lens_injury"), ClaimContactLensInjury)
	assert.Equal(t, ClaimType("surgical_correction"), ClaimSurgicalCorrection)
	assert.Equal(t, ClaimType("follow_up"), ClaimFollowUp)

	assert.Equal(t, ClaimStatus("draft"), StatusDraft)
	assert.Equal(t, ClaimStatus("ready_to_submit"), StatusReadyToSubmit)
	assert.Equal(t, ClaimStatus("submitted"), StatusSubmitted)
	assert.Equal(t, ClaimStatus("accepted"), StatusAccepted)
	assert.Equal(t, ClaimStatus("partially_paid"), StatusPartiallyPaid)
	assert.Equal(t, ClaimStatus("declined"), StatusDeclined)
	assert.Equal(t, ClaimStatus("requires_info"), StatusRequiresInfo)
	assert.Equal(t, ClaimStatus("appealed"), StatusAppealed)

	assert.Equal(t, TreatmentProvider("optometrist"), ProviderOptometrist)
	assert.Equal(t, TreatmentProvider("ophthalmologist"), ProviderOphthalmologist)
	assert.Equal(t, TreatmentProvider("optical_dispenser"), ProviderOpticalDispenser)
	assert.Equal(t, TreatmentProvider("gp"), ProviderGP)
}
