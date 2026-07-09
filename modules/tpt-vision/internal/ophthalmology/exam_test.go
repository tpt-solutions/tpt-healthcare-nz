package ophthalmology

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOphthalmicExamValidate(t *testing.T) {
	valid := func() *OphthalmicExam {
		return &OphthalmicExam{
			PatientNHI:  "ABC1235",
			ClinicianID: "CPN123",
			ExamDate:    1700000000000,
		}
	}

	t.Run("valid", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		e := valid()
		e.PatientNHI = ""
		assert.Error(t, e.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		e := valid()
		e.ClinicianID = ""
		assert.Error(t, e.Validate())
	})

	t.Run("missing exam date", func(t *testing.T) {
		e := valid()
		e.ExamDate = 0
		assert.Error(t, e.Validate())
	})

	t.Run("valid with all optional fields", func(t *testing.T) {
		e := valid()
		e.ExamType = ExamComprehensive
		e.VADistanceRight = "6/6"
		e.VADistanceLeft = "6/9"
		e.Diagnosis = "Normal"
		assert.NoError(t, e.Validate())
	})
}

func TestNewExam(t *testing.T) {
	e := NewExam()
	assert.NotZero(t, e.ExamDate)
	assert.NotZero(t, e.CreatedAt)
	assert.NotZero(t, e.UpdatedAt)
	assert.Equal(t, e.CreatedAt, e.UpdatedAt)
}

func TestToFHIRDiagnosticReport(t *testing.T) {
	e := &OphthalmicExam{
		ID:              "exam-1",
		PatientNHI:      "ABC1235",
		ClinicianID:     "CPN123",
		ExamType:        ExamComprehensive,
		ExamDate:        1700000000000,
		CreatedAt:       1700000000000,
		VADistanceRight: "6/6",
		IOP:             []IOPReading{{Method: TonoGoldmann, RightEye: 15, LeftEye: 14}},
		Diagnosis:       "Normal eye exam",
		FollowUpDays:    365,
	}
	report := e.ToFHIRDiagnosticReport()
	assert.Equal(t, "DiagnosticReport", report["resourceType"])
	assert.Equal(t, "exam-1", report["id"])
	assert.Equal(t, "final", report["status"])

	results := report["result"].([]map[string]any)
	assert.GreaterOrEqual(t, len(results), 2)

	ext := report["extension"].([]map[string]any)
	assert.GreaterOrEqual(t, len(ext), 3)

	subject := report["subject"].(map[string]any)
	assert.Equal(t, "Patient/ABC1235", subject["reference"])

	performer := report["performer"].([]map[string]any)
	assert.Equal(t, "Practitioner/CPN123", performer[0]["reference"])

	assert.Equal(t, "Normal eye exam", report["conclusion"])
}

func TestToFHIRDiagnosticReport_Minimal(t *testing.T) {
	e := &OphthalmicExam{
		ID:          "exam-2",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		ExamDate:    1700000000000,
		CreatedAt:   1700000000000,
	}
	report := e.ToFHIRDiagnosticReport()
	results := report["result"].([]map[string]any)
	assert.GreaterOrEqual(t, len(results), 2)
}

func TestToFHIRDiagnosticReport_WithVisualFieldsAndOCT(t *testing.T) {
	e := &OphthalmicExam{
		ID:                "exam-3",
		PatientNHI:        "ABC1235",
		ClinicianID:       "CPN123",
		ExamType:          ExamGlaucoma,
		ExamDate:          1700000000000,
		CreatedAt:         1700000000000,
		VisualFieldsRight: "normal",
		VisualFieldsLeft:  "constricted",
		OCTRight:          "RNFL normal",
		OCTLeft:           "RNFL thinning superiorly",
	}
	report := e.ToFHIRDiagnosticReport()
	results := report["result"].([]map[string]any)

	// Should have anterior, posterior, visual fields, and OCT
	assert.GreaterOrEqual(t, len(results), 4)

	// Check visual fields and OCT references
	refStrs := make([]string, len(results))
	for i, r := range results {
		refStrs[i] = r["reference"].(string)
	}
	foundVF := false
	foundOCT := false
	for _, s := range refStrs {
		if s == "Observation/exam-3-visual-fields" {
			foundVF = true
		}
		if s == "Observation/exam-3-oct" {
			foundOCT = true
		}
	}
	assert.True(t, foundVF, "visual fields reference should exist")
	assert.True(t, foundOCT, "OCT reference should exist")
}

func TestToFHIRDiagnosticReport_WithNearVA(t *testing.T) {
	e := &OphthalmicExam{
		ID:          "exam-4",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		ExamDate:    1700000000000,
		CreatedAt:   1700000000000,
		VANearRight: "N5",
		VANearLeft:  "N6",
	}
	report := e.ToFHIRDiagnosticReport()
	results := report["result"].([]map[string]any)

	// Should have near VA, anterior, posterior
	assert.GreaterOrEqual(t, len(results), 3)

	refStrs := make([]string, len(results))
	for i, r := range results {
		refStrs[i] = r["reference"].(string)
	}
	foundNear := false
	for _, s := range refStrs {
		if s == "Observation/exam-4-va-near" {
			foundNear = true
		}
	}
	assert.True(t, foundNear, "near VA reference should exist")
}

func TestToFHIRDiagnosticReport_FollowUpExtension(t *testing.T) {
	e := &OphthalmicExam{
		ID:           "exam-5",
		PatientNHI:   "ABC1235",
		ClinicianID:  "CPN123",
		ExamDate:     1700000000000,
		CreatedAt:    1700000000000,
		FollowUpDays: 0,
	}
	report := e.ToFHIRDiagnosticReport()
	ext := report["extension"].([]map[string]any)

	// Without follow-up, should have 2 extensions (exam type + referral required)
	assert.Len(t, ext, 2)
}

func TestToFHIRDiagnosticReport_Extensions(t *testing.T) {
	e := &OphthalmicExam{
		ID:               "exam-6",
		PatientNHI:       "ABC1235",
		ClinicianID:      "CPN123",
		ExamType:         ExamFollowUp,
		ExamDate:         1700000000000,
		CreatedAt:        1700000000000,
		ReferralRequired: true,
		FollowUpDays:     90,
	}
	report := e.ToFHIRDiagnosticReport()
	ext := report["extension"].([]map[string]any)

	assert.Len(t, ext, 3)
	assert.Equal(t, "https://nzfhir.org/StructureDefinition/nz-ophthalmic-exam-type", ext[0]["url"])
	assert.Equal(t, "follow_up", ext[0]["valueCode"])
	assert.Equal(t, true, ext[1]["valueBoolean"])
	assert.Equal(t, 90, ext[2]["valueInteger"])
}

func TestExamConstants(t *testing.T) {
	assert.Equal(t, ExamType("comprehensive"), ExamComprehensive)
	assert.Equal(t, ExamType("follow_up"), ExamFollowUp)
	assert.Equal(t, ExamType("glaucoma"), ExamGlaucoma)
	assert.Equal(t, ExamType("retina"), ExamRetina)
	assert.Equal(t, ExamType("cataract"), ExamCataract)
	assert.Equal(t, ExamType("cornea"), ExamCornea)
	assert.Equal(t, ExamType("neuro_ophthalmic"), ExamNeuroOphthalmic)
	assert.Equal(t, ExamType("paediatric"), ExamPaediatric)
	assert.Equal(t, ExamType("emergency"), ExamEmergency)

	assert.Equal(t, TonometryMethod("goldmann_applanation"), TonoGoldmann)
	assert.Equal(t, TonometryMethod("non_contact"), TonoNonContact)
	assert.Equal(t, TonometryMethod("rebound"), TonoRebound)
	assert.Equal(t, TonometryMethod("tonopen"), TonoTonopen)
	assert.Equal(t, TonometryMethod("digital_palpation"), TonoDigital)

	assert.Equal(t, GradingScale(0), GradeNone)
	assert.Equal(t, GradingScale(1), GradeTrace)
	assert.Equal(t, GradingScale(2), GradeMild)
	assert.Equal(t, GradingScale(3), GradeModerate)
	assert.Equal(t, GradingScale(4), GradeSevere)
}

func TestExamLensAndDiscConstants(t *testing.T) {
	assert.Equal(t, LensStatus("phakic"), LensPhakic)
	assert.Equal(t, LensStatus("pseudophakic"), LensPseudophakic)
	assert.Equal(t, LensStatus("aphakic"), LensAphakic)
	assert.Equal(t, LensStatus("pc_iol"), LensPCIOL)
	assert.Equal(t, LensStatus("ac_iol"), LensACIOL)

	assert.Equal(t, OpticDiscAppearance("normal"), DiscNormal)
	assert.Equal(t, OpticDiscAppearance("cupped"), DiscCupped)
	assert.Equal(t, OpticDiscAppearance("pale"), DiscPale)
	assert.Equal(t, OpticDiscAppearance("oedematous"), DiscOedematous)

	assert.Equal(t, MacularStatus("normal"), MaculaNormal)
	assert.Equal(t, MacularStatus("macular_oedema"), MaculaOedema)
	assert.Equal(t, MacularStatus("macular_hole"), MaculaHole)
	assert.Equal(t, MacularStatus("choroidal_neovascularisation"), MaculaCNV)
}
