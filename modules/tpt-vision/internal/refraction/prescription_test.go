package refraction

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrescriptionValidate(t *testing.T) {
	valid := func() *Prescription {
		return &Prescription{
			PatientNHI:  "ABC1235",
			ClinicianID: "CPN123",
			IssuedDate:  1700000000000,
			RightEye:    EyePrescription{Sphere: -2.00},
			LeftEye:     EyePrescription{Sphere: -1.75},
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

	t.Run("missing issued date", func(t *testing.T) {
		p := valid()
		p.IssuedDate = 0
		assert.Error(t, p.Validate())
	})

	t.Run("invalid sphere step", func(t *testing.T) {
		p := valid()
		p.RightEye.Sphere = -2.10
		assert.Error(t, p.Validate())
	})

	t.Run("invalid cylinder step", func(t *testing.T) {
		p := valid()
		p.RightEye.Cylinder = -0.30
		assert.Error(t, p.Validate())
	})

	t.Run("cylinder without axis", func(t *testing.T) {
		p := valid()
		p.RightEye.Cylinder = -1.00
		p.RightEye.Axis = 0
		assert.Error(t, p.Validate())
	})

	t.Run("axis out of range", func(t *testing.T) {
		p := valid()
		p.RightEye.Cylinder = -1.00
		p.RightEye.Axis = 181
		assert.Error(t, p.Validate())
	})

	t.Run("prism without direction", func(t *testing.T) {
		p := valid()
		p.RightEye.Prism = 2.0
		p.RightEye.PrismDir = ""
		assert.Error(t, p.Validate())
	})

	t.Run("prism with direction is ok", func(t *testing.T) {
		p := valid()
		p.RightEye.Prism = 2.0
		p.RightEye.PrismDir = BaseInn
		assert.NoError(t, p.Validate())
	})

	t.Run("zero cylinder with axis is ok", func(t *testing.T) {
		p := valid()
		p.RightEye.Cylinder = 0
		p.RightEye.Axis = 90
		assert.NoError(t, p.Validate())
	})

	t.Run("left eye invalid sphere", func(t *testing.T) {
		p := valid()
		p.LeftEye.Sphere = -1.10
		assert.Error(t, p.Validate())
	})

	t.Run("left eye cylinder without axis", func(t *testing.T) {
		p := valid()
		p.LeftEye.Cylinder = -2.00
		p.LeftEye.Axis = 0
		assert.Error(t, p.Validate())
	})

	t.Run("left eye axis boundary 1 is ok", func(t *testing.T) {
		p := valid()
		p.LeftEye.Cylinder = -1.00
		p.LeftEye.Axis = 1
		assert.NoError(t, p.Validate())
	})

	t.Run("left eye axis boundary 180 is ok", func(t *testing.T) {
		p := valid()
		p.LeftEye.Cylinder = -1.00
		p.LeftEye.Axis = 180
		assert.NoError(t, p.Validate())
	})

	t.Run("all prism directions accepted", func(t *testing.T) {
		dirs := []PrismDirection{BaseIn, BaseDown, BaseInn, BaseOut}
		for _, dir := range dirs {
			p := valid()
			p.RightEye.Prism = 1.5
			p.RightEye.PrismDir = dir
			assert.NoError(t, p.Validate(), "direction %s", dir)
		}
	})

	t.Run("valid 0.25D step sphere", func(t *testing.T) {
		p := valid()
		p.RightEye.Sphere = -3.75
		p.LeftEye.Sphere = 2.25
		assert.NoError(t, p.Validate())
	})

	t.Run("valid 0.25D step cylinder", func(t *testing.T) {
		p := valid()
		p.RightEye.Cylinder = -2.25
		p.RightEye.Axis = 45
		assert.NoError(t, p.Validate())
	})
}

func TestSphericalEquivalent(t *testing.T) {
	tests := []struct {
		name   string
		sphere float64
		cyl    float64
		want   float64
	}{
		{"plano", 0, 0, 0},
		{"sphere only", -2.00, 0, -2.00},
		{"with cylinder", -2.00, -1.50, -2.75},
		{"rounding", -1.00, -0.50, -1.25},
		{"positive sphere with cylinder", 3.00, -1.00, 2.50},
		{"large cylinder", -6.00, -3.00, -7.50},
		{"half-step cylinder rounding", -1.00, -1.00, -1.50},
		{"zero sphere non-zero cyl", 0, -2.00, -1.00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &EyePrescription{Sphere: tt.sphere, Cylinder: tt.cyl}
			got := e.SphericalEquivalent()
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestFormatSphere(t *testing.T) {
	tests := []struct {
		sphere float64
		want   string
	}{
		{0, "PL"},
		{2.00, "+2.00"},
		{-1.25, "-1.25"},
		{0.50, "+0.50"},
		{-6.00, "-6.00"},
		{0.25, "+0.25"},
		{-0.25, "-0.25"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			e := &EyePrescription{Sphere: tt.sphere}
			assert.Equal(t, tt.want, e.FormatSphere())
		})
	}
}

func TestSnellenToLogMAR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"6/6", "6/6", 0.0, false},
		{"6/12", "6/12", 0.3010, false},
		{"6/60", "6/60", 1.0, false},
		{"6/9", "6/9", 0.1761, false},
		{"6/5", "6/5", -0.07918, false},
		{"20/20 equivalent", "6/6", 0.0, false},
		{"empty", "", 0, true},
		{"no slash", "66", 0, true},
		{"zero numerator", "0/6", 0, true},
		{"zero denominator", "6/0", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SnellenToLogMAR(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestNewPrescription(t *testing.T) {
	p := NewPrescription()
	assert.True(t, p.IsCurrent)
	assert.NotZero(t, p.IssuedDate)
	assert.Greater(t, p.ExpiryDate, p.IssuedDate)
	assert.NotZero(t, p.CreatedAt)
	assert.NotZero(t, p.UpdatedAt)
	assert.Equal(t, p.CreatedAt, p.UpdatedAt)
}

func TestToFHIRObservation(t *testing.T) {
	p := &Prescription{
		ID:          "rx-1",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		Type:        Spectacle,
		Distance:    Distance,
		IssuedDate:  1700000000000,
		ExpiryDate:  1731536000000,
		RightEye:    EyePrescription{Sphere: -2.00, Cylinder: -0.50, Axis: 180},
		LeftEye:     EyePrescription{Sphere: -1.75},
	}
	obs := p.ToFHIRObservation()
	assert.Equal(t, "Observation", obs["resourceType"])
	assert.Equal(t, "rx-1", obs["id"])
	assert.Equal(t, "final", obs["status"])

	components := obs["component"].([]map[string]any)
	assert.Len(t, components, 2)

	ext := obs["extension"].([]map[string]any)
	assert.GreaterOrEqual(t, len(ext), 3)

	subject := obs["subject"].(map[string]any)
	assert.Equal(t, "Patient/ABC1235", subject["reference"])

	performer := obs["performer"].([]map[string]any)
	assert.Equal(t, "Practitioner/CPN123", performer[0]["reference"])
}

func TestToFHIRObservation_WithCylinderSubComponents(t *testing.T) {
	p := &Prescription{
		ID:          "rx-2",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		IssuedDate:  1700000000000,
		ExpiryDate:  1731536000000,
		RightEye: EyePrescription{
			Sphere:   -3.00,
			Cylinder: -1.25,
			Axis:     90,
		},
		LeftEye: EyePrescription{Sphere: -2.00},
	}
	obs := p.ToFHIRObservation()
	components := obs["component"].([]map[string]any)
	rightComp := components[0]

	subComps := rightComp["component"].([]map[string]any)
	assert.GreaterOrEqual(t, len(subComps), 2)
}

func TestToFHIRObservation_WithPrismAndADD(t *testing.T) {
	p := &Prescription{
		ID:          "rx-3",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		IssuedDate:  1700000000000,
		ExpiryDate:  1731536000000,
		RightEye: EyePrescription{
			Sphere:   -2.00,
			Prism:    2.0,
			PrismDir: BaseInn,
			ADD:      2.50,
		},
		LeftEye: EyePrescription{Sphere: -2.00},
	}
	obs := p.ToFHIRObservation()
	components := obs["component"].([]map[string]any)
	rightComp := components[0]

	subComps := rightComp["component"].([]map[string]any)
	assert.GreaterOrEqual(t, len(subComps), 2)
}

func TestToFHIRObservation_WithVisualAcuityAndMethod(t *testing.T) {
	p := &Prescription{
		ID:          "rx-4",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		IssuedDate:  1700000000000,
		ExpiryDate:  1731536000000,
		RightEye: EyePrescription{
			Sphere:       -2.00,
			VisualAcuity: "6/6",
			Method:       MethodSubjective,
		},
		LeftEye: EyePrescription{Sphere: -2.00},
	}
	obs := p.ToFHIRObservation()
	components := obs["component"].([]map[string]any)
	rightComp := components[0]

	subComps := rightComp["component"].([]map[string]any)
	assert.GreaterOrEqual(t, len(subComps), 2)
}

func TestToFHIRObservation_WithNotes(t *testing.T) {
	p := &Prescription{
		ID:          "rx-5",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		IssuedDate:  1700000000000,
		ExpiryDate:  1731536000000,
		RightEye:    EyePrescription{Sphere: -2.00, Notes: "right eye note"},
		LeftEye:     EyePrescription{Sphere: -1.75, Notes: "left eye note"},
	}
	obs := p.ToFHIRObservation()
	notes := obs["note"].([]map[string]any)
	assert.Len(t, notes, 2)
	assert.Equal(t, "Right eye: right eye note", notes[0]["text"])
	assert.Equal(t, "Left eye: left eye note", notes[1]["text"])
}

func TestToFHIRObservation_Extensions(t *testing.T) {
	p := &Prescription{
		ID:          "rx-6",
		PatientNHI:  "ABC1235",
		ClinicianID: "CPN123",
		Distance:    Near,
		IssuedDate:  1700000000000,
		ExpiryDate:  1731536000000,
		IsCurrent:   true,
		RightEye:    EyePrescription{Sphere: -1.00},
		LeftEye:     EyePrescription{Sphere: -1.00},
	}
	obs := p.ToFHIRObservation()
	ext := obs["extension"].([]map[string]any)
	assert.Len(t, ext, 3)

	assert.Equal(t, "https://nzfhir.org/StructureDefinition/nz-vision-prescription-distance", ext[0]["url"])
	assert.Equal(t, "near", ext[0]["valueCode"])

	assert.Equal(t, "https://nzfhir.org/StructureDefinition/nz-vision-prescription-expiry", ext[1]["url"])

	assert.Equal(t, "https://nzfhir.org/StructureDefinition/nz-vision-prescription-current", ext[2]["url"])
	assert.Equal(t, true, ext[2]["valueBoolean"])
}

func TestPrescriptionConstants(t *testing.T) {
	assert.Equal(t, Eye("right"), EyeRight)
	assert.Equal(t, Eye("left"), EyeLeft)
	assert.Equal(t, PrescriptionType("spectacle"), Spectacle)
	assert.Equal(t, PrescriptionType("contact"), Contact)
	assert.Equal(t, DistanceType("distance"), Distance)
	assert.Equal(t, DistanceType("near"), Near)
	assert.Equal(t, DistanceType("intermediate"), Intermediate)
	assert.Equal(t, PrismDirection("BU"), BaseIn)
	assert.Equal(t, PrismDirection("BD"), BaseDown)
	assert.Equal(t, PrismDirection("BI"), BaseInn)
	assert.Equal(t, PrismDirection("BO"), BaseOut)
}
