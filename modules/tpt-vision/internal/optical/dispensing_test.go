package optical

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDispensingOrderValidate(t *testing.T) {
	valid := func() *DispensingOrder {
		return &DispensingOrder{
			PatientNHI:     "ZAB000H",
			ClinicianID:    "CPN123",
			PrescriptionID: "rx-1",
			Frame:          &FrameDetails{FrameType: FrameFullRim},
		}
	}

	t.Run("valid with frame", func(t *testing.T) {
		assert.NoError(t, valid().Validate())
	})

	t.Run("valid with contact lens", func(t *testing.T) {
		o := valid()
		o.Frame = nil
		o.ContactLens = &ContactLensOrder{Type: CLDailyDisposable}
		assert.NoError(t, o.Validate())
	})

	t.Run("missing NHI", func(t *testing.T) {
		o := valid()
		o.PatientNHI = ""
		assert.Error(t, o.Validate())
	})

	t.Run("missing clinician", func(t *testing.T) {
		o := valid()
		o.ClinicianID = ""
		assert.Error(t, o.Validate())
	})

	t.Run("missing prescription", func(t *testing.T) {
		o := valid()
		o.PrescriptionID = ""
		assert.Error(t, o.Validate())
	})

	t.Run("no frame and no contact lens", func(t *testing.T) {
		o := valid()
		o.Frame = nil
		o.ContactLens = nil
		assert.Error(t, o.Validate())
	})

	t.Run("valid with frame and contact lens both set", func(t *testing.T) {
		o := valid()
		o.ContactLens = &ContactLensOrder{Type: CLMonthly}
		assert.NoError(t, o.Validate())
	})

	t.Run("valid with frame details fully populated", func(t *testing.T) {
		o := valid()
		o.Frame = &FrameDetails{
			FrameType:  FrameSemiRim,
			Brand:      "Silhouette",
			Model:      "Aero",
			Colour:     "Black",
			Size:       "54-16-140",
			FramePrice: 399.00,
		}
		assert.NoError(t, o.Validate())
	})
}

func TestSubsidyEligible(t *testing.T) {
	tests := []struct {
		name string
		age  int
		card bool
		want bool
	}{
		{"child under 16", 15, false, true},
		{"age 16 no card", 16, false, false},
		{"adult with card", 30, true, true},
		{"adult no card", 30, false, false},
		{"infant", 0, false, true},
		{"age 1 with card", 1, true, true},
		{"age 65 with card", 65, true, true},
		{"age 65 no card", 65, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SubsidyEligible(tt.age, tt.card))
		})
	}
}

func TestNewDispensingOrder(t *testing.T) {
	o := NewDispensingOrder()
	assert.Equal(t, OrderPending, o.Status)
	assert.NotZero(t, o.OrderDate)
	assert.NotZero(t, o.CreatedAt)
	assert.NotZero(t, o.UpdatedAt)
	assert.Equal(t, o.CreatedAt, o.UpdatedAt)
}

func TestToFHIRMedicationDispense(t *testing.T) {
	o := &DispensingOrder{
		ID:             "disp-1",
		PatientNHI:     "ZAB000H",
		DispenserID:    "D123",
		PrescriptionID: "rx-1",
		Frame:          &FrameDetails{FrameType: FrameFullRim},
		OrderDate:      1700000000000,
		UpdatedAt:      1700000000000,
	}
	disp := o.ToFHIRMedicationDispense()
	assert.Equal(t, "MedicationDispense", disp["resourceType"])
	assert.Equal(t, "disp-1", disp["id"])

	ext := disp["extension"].([]map[string]any)
	assert.GreaterOrEqual(t, len(ext), 4)

	subject := disp["subject"].(map[string]any)
	assert.Equal(t, "Patient/ZAB000H", subject["reference"])

	performer := disp["performer"].([]map[string]any)
	assert.Equal(t, "Practitioner/D123", performer[0]["actor"].(map[string]any)["reference"])

	prescription := disp["authorizingPrescription"].([]map[string]any)
	assert.Equal(t, "MedicationRequest/rx-1", prescription[0]["reference"])
}

func TestToFHIRMedicationDispense_ContactLens(t *testing.T) {
	o := &DispensingOrder{
		ID:             "disp-2",
		PatientNHI:     "ZAB000H",
		DispenserID:    "D456",
		PrescriptionID: "rx-2",
		ContactLens:    &ContactLensOrder{Type: CLDailyDisposable, Brand: "Acuvue"},
		OrderDate:      1700000000000,
		UpdatedAt:      1700000000000,
	}
	disp := o.ToFHIRMedicationDispense()
	assert.Equal(t, "MedicationDispense", disp["resourceType"])

	ext := disp["extension"].([]map[string]any)
	// First extension should be contact_lens type
	assert.Equal(t, "contact_lens", ext[0]["valueCode"])
}

func TestToFHIRMedicationDispense_StatusMapping(t *testing.T) {
	statusTests := []struct {
		status OrderStatus
		want   string
	}{
		{OrderPending, "preparation"},
		{OrderLabSent, "in-progress"},
		{OrderInLab, "in-progress"},
		{OrderReceived, "completed"},
		{OrderReady, "completed"},
		{OrderCollected, "completed"},
		{OrderCancelled, "cancelled"},
		{OrderWarranty, "completed"},
	}
	for _, tt := range statusTests {
		t.Run(string(tt.status), func(t *testing.T) {
			o := &DispensingOrder{
				ID:         "disp-st",
				PatientNHI: "ZAB000H",
				Status:     tt.status,
				Frame:      &FrameDetails{FrameType: FrameFullRim},
				OrderDate:  1700000000000,
				UpdatedAt:  1700000000000,
			}
			disp := o.ToFHIRMedicationDispense()
			assert.Equal(t, tt.want, disp["status"])
		})
	}
}

func TestToFHIRMedicationDispense_WithNotes(t *testing.T) {
	o := &DispensingOrder{
		ID:             "disp-3",
		PatientNHI:     "ZAB000H",
		DispenserID:    "D789",
		PrescriptionID: "rx-3",
		Frame:          &FrameDetails{FrameType: FrameRimless},
		Notes:          "Patient prefers thin lenses",
		OrderDate:      1700000000000,
		UpdatedAt:      1700000000000,
	}
	disp := o.ToFHIRMedicationDispense()
	notes := disp["note"].([]map[string]any)
	assert.Len(t, notes, 1)
	assert.Equal(t, "Patient prefers thin lenses", notes[0]["text"])
}

func TestDispensingOrderConstants(t *testing.T) {
	assert.Equal(t, OrderStatus("pending"), OrderPending)
	assert.Equal(t, OrderStatus("lab_sent"), OrderLabSent)
	assert.Equal(t, OrderStatus("in_lab"), OrderInLab)
	assert.Equal(t, OrderStatus("received"), OrderReceived)
	assert.Equal(t, OrderStatus("ready_for_collection"), OrderReady)
	assert.Equal(t, OrderStatus("collected"), OrderCollected)
	assert.Equal(t, OrderStatus("cancelled"), OrderCancelled)
	assert.Equal(t, OrderStatus("warranty_claim"), OrderWarranty)

	assert.Equal(t, FrameType("full_rim"), FrameFullRim)
	assert.Equal(t, FrameType("semi_rimless"), FrameSemiRim)
	assert.Equal(t, FrameType("rimless"), FrameRimless)
	assert.Equal(t, FrameType("childrens"), FrameChildrens)
	assert.Equal(t, FrameType("safety"), FrameSafety)

	assert.Equal(t, LensType("single_vision"), LensSingleVision)
	assert.Equal(t, LensType("bifocal"), LensBifocal)
	assert.Equal(t, LensType("progressive"), LensProgressive)

	assert.Equal(t, ContactLensType("daily_disposable"), CLDailyDisposable)
	assert.Equal(t, ContactLensType("monthly"), CLMonthly)
	assert.Equal(t, ContactLensType("rgp"), CLRGP)
	assert.Equal(t, ContactLensType("ortho_k"), CLOrthoK)
	assert.Equal(t, ContactLensType("scleral"), CLScleral)
}

func TestDispenseTypeCode(t *testing.T) {
	t.Run("spectacle order", func(t *testing.T) {
		o := &DispensingOrder{Frame: &FrameDetails{}}
		disp := o.ToFHIRMedicationDispense()
		ext := disp["extension"].([]map[string]any)
		assert.Equal(t, "spectacle", ext[0]["valueCode"])
	})

	t.Run("contact lens order", func(t *testing.T) {
		o := &DispensingOrder{ContactLens: &ContactLensOrder{}}
		disp := o.ToFHIRMedicationDispense()
		ext := disp["extension"].([]map[string]any)
		assert.Equal(t, "contact_lens", ext[0]["valueCode"])
	})
}

func TestToFHIRMedicationDispense_FundingFlags(t *testing.T) {
	o := &DispensingOrder{
		ID:             "disp-fund",
		PatientNHI:     "ZAB000H",
		PrescriptionID: "rx-fund",
		Frame:          &FrameDetails{FrameType: FrameFullRim},
		FundedByACC:    true,
		FundedByDHB:    true,
		TotalPrice:     599.00,
		OrderDate:      1700000000000,
		UpdatedAt:      1700000000000,
	}
	disp := o.ToFHIRMedicationDispense()
	ext := disp["extension"].([]map[string]any)

	// ACC funding flag
	assert.Equal(t, true, ext[1]["valueBoolean"])
	// DHB funding flag
	assert.Equal(t, true, ext[2]["valueBoolean"])
	// Price
	priceExt := ext[3]["valueMoney"].(map[string]any)
	assert.Equal(t, 599.00, priceExt["value"])
	assert.Equal(t, "NZD", priceExt["currency"])
}

func TestNewDispensingOrder_TimestampConsistency(t *testing.T) {
	o := NewDispensingOrder()
	now := time.Now().UnixMilli()
	assert.InDelta(t, now, o.OrderDate, 1000)
	assert.InDelta(t, now, o.CreatedAt, 1000)
	assert.InDelta(t, now, o.UpdatedAt, 1000)
}

func TestMeasurementFields(t *testing.T) {
	m := Measurement{
		PD:                 63.5,
		PDNear:             60.0,
		SegmentHeight:      18.0,
		BackVertexDistance: 12.0,
		PantoscopicTilt:    10.0,
		FaceFormAngle:      5.0,
	}
	assert.Equal(t, 63.5, m.PD)
	assert.Equal(t, 60.0, m.PDNear)
	assert.Equal(t, 18.0, m.SegmentHeight)
	assert.Equal(t, 12.0, m.BackVertexDistance)
	assert.Equal(t, 10.0, m.PantoscopicTilt)
	assert.Equal(t, 5.0, m.FaceFormAngle)
}

func TestContactLensOrderFields(t *testing.T) {
	cl := ContactLensOrder{
		Type:        CLBiWeekly,
		Brand:       "Air Optix",
		BaseCurve:   8.6,
		Diameter:    14.2,
		PowerRight:  -3.00,
		PowerLeft:   -2.75,
		CylRight:    -0.75,
		CylLeft:     -0.50,
		AxisRight:   180,
		AxisLeft:    170,
		Qty:         6,
		PricePerBox: 45.00,
	}
	assert.Equal(t, CLBiWeekly, cl.Type)
	assert.Equal(t, "Air Optix", cl.Brand)
	assert.Equal(t, 8.6, cl.BaseCurve)
	assert.Equal(t, 14.2, cl.Diameter)
	assert.Equal(t, -3.00, cl.PowerRight)
	assert.Equal(t, -2.75, cl.PowerLeft)
	assert.Equal(t, 6, cl.Qty)
	assert.Equal(t, 45.00, cl.PricePerBox)
}
