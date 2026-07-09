package translate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PhillipC05/tpt-healthcare/core/fhir/r4"
)

func timePtr(t time.Time) *time.Time { return &t }
func boolPtr(b bool) *bool           { return &b }

func TestPatientR4ToR5_RoundTrip(t *testing.T) {
	r4p := &r4.Patient{
		ResourceType: "Patient",
		ID:           "patient-123",
		Active:       boolPtr(true),
		Gender:       "male",
		BirthDate:    "1990-01-15",
		Name: []r4.HumanName{{
			Use:    "official",
			Text:   "John Smith",
			Family: "Smith",
			Given:  []string{"John", "Michael"},
			Prefix: []string{"Mr"},
		}},
		Identifier: []r4.Identifier{{
			Use:    "official",
			System: "https://standards.digital.health.nz/ns/nhi-id",
			Value:  "ZAC1234",
		}},
		Telecom: []r4.ContactPoint{{
			System: "phone",
			Value:  "021-555-0123",
			Use:    "mobile",
		}},
		Address: []r4.Address{{
			Use:        "home",
			Text:       "123 Test Street, Auckland",
			Line:       []string{"123 Test Street"},
			City:       "Auckland",
			PostalCode: "1010",
			Country:    "NZ",
		}},
	}

	r5p := PatientR4ToR5(r4p)
	require.NotNil(t, r5p)
	assert.Equal(t, "patient-123", r5p.ID)
	assert.Equal(t, "male", r5p.Gender)
	assert.Equal(t, "1990-01-15", r5p.BirthDate)
	require.Len(t, r5p.Name, 1)
	assert.Equal(t, "Smith", r5p.Name[0].Family)
	assert.Equal(t, []string{"John", "Michael"}, r5p.Name[0].Given)
	require.Len(t, r5p.Identifier, 1)
	assert.Equal(t, "ZAC1234", r5p.Identifier[0].Value)
	require.Len(t, r5p.Telecom, 1)
	assert.Equal(t, "021-555-0123", r5p.Telecom[0].Value)
	require.Len(t, r5p.Address, 1)
	assert.Equal(t, "Auckland", r5p.Address[0].City)

	r4p2 := PatientR5ToR4(r5p)
	require.NotNil(t, r4p2)
	assert.Equal(t, r4p.ID, r4p2.ID)
	assert.Equal(t, r4p.Gender, r4p2.Gender)
	assert.Equal(t, r4p.BirthDate, r4p2.BirthDate)
	require.Len(t, r4p2.Name, 1)
	assert.Equal(t, r4p.Name[0].Family, r4p2.Name[0].Family)
}

func TestPatientR4ToR5_NilInput(t *testing.T) {
	assert.Nil(t, PatientR4ToR5(nil))
}

func TestPatientR5ToR4_NilInput(t *testing.T) {
	assert.Nil(t, PatientR5ToR4(nil))
}

func TestPractitionerR4ToR5_RoundTrip(t *testing.T) {
	r4p := &r4.Practitioner{
		ResourceType: "Practitioner",
		ID:           "pract-456",
		Active:       boolPtr(true),
		Gender:       "female",
		Name: []r4.HumanName{{
			Use:    "official",
			Family: "Jones",
			Given:  []string{"Sarah"},
		}},
		Identifier: []r4.Identifier{{
			System: "https://standards.digital.health.nz/ns/hpi-person-id",
			Value:  "CPN9876543210",
		}},
		Qualification: []r4.PractitionerQualification{{
			Code: r4.CodeableConcept{
				Text: "General Practitioner",
				Coding: []r4.Coding{{
					System: "https://standards.digital.health.nz/ns/hpi-qualification",
					Code:   "GP",
				}},
			},
		}},
	}

	r5p := PractitionerR4ToR5(r4p)
	require.NotNil(t, r5p)
	assert.Equal(t, "pract-456", r5p.ID)
	require.Len(t, r5p.Name, 1)
	assert.Equal(t, "Jones", r5p.Name[0].Family)
	require.Len(t, r5p.Qualification, 1)
	assert.Equal(t, "General Practitioner", r5p.Qualification[0].Code.Text)

	r4p2 := PractitionerR5ToR4(r5p)
	require.NotNil(t, r4p2)
	assert.Equal(t, r4p.ID, r4p2.ID)
	require.Len(t, r4p2.Qualification, 1)
	assert.Equal(t, "GP", r4p2.Qualification[0].Code.Coding[0].Code)
}

func TestPractitionerR4ToR5_NilInput(t *testing.T) {
	assert.Nil(t, PractitionerR4ToR5(nil))
}

func TestPractitionerR5ToR4_NilInput(t *testing.T) {
	assert.Nil(t, PractitionerR5ToR4(nil))
}

func TestCodingR4ToR5_RoundTrip(t *testing.T) {
	r4c := r4.Coding{System: "http://loinc.org", Code: "12345-6", Display: "Test"}
	r5c := codingR4ToR5(r4c)
	assert.Equal(t, r4c.System, r5c.System)
	assert.Equal(t, r4c.Code, r5c.Code)
	assert.Equal(t, r4c.Display, r5c.Display)

	r4c2 := codingR5ToR4(r5c)
	assert.Equal(t, r4c.System, r4c2.System)
	assert.Equal(t, r4c.Code, r4c2.Code)
}

func TestIdentifierR4ToR5_RoundTrip(t *testing.T) {
	r4id := r4.Identifier{
		Use:    "official",
		System: "https://standards.digital.health.nz/ns/nhi-id",
		Value:  "ZAC1234",
		Type:   r4.CodeableConcept{Text: "NHI"},
		Period: &r4.Period{Start: timePtr(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))},
	}

	r5id := identifierR4ToR5(r4id)
	assert.Equal(t, r4id.Use, r5id.Use)
	assert.Equal(t, r4id.System, r5id.System)
	assert.Equal(t, r4id.Value, r5id.Value)
	assert.Equal(t, r4id.Type.Text, r5id.Type.Text)
	require.NotNil(t, r5id.Period)

	r4id2 := identifierR5ToR4(r5id)
	assert.Equal(t, r4id.Value, r4id2.Value)
}

func TestPeriodR4ToR5_RoundTrip(t *testing.T) {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2020, 12, 31, 0, 0, 0, 0, time.UTC)
	r4p := &r4.Period{Start: &start, End: &end}
	r5p := periodR4ToR5(r4p)
	require.NotNil(t, r5p)
	assert.Equal(t, start, *r5p.Start)
	assert.Equal(t, end, *r5p.End)

	r4p2 := periodR5ToR4(r5p)
	require.NotNil(t, r4p2)
	assert.Equal(t, start, *r4p2.Start)
}

func TestPeriodR4ToR5_Nil(t *testing.T) {
	assert.Nil(t, periodR4ToR5(nil))
	assert.Nil(t, periodR5ToR4(nil))
}

func TestNameR4ToR5_RoundTrip(t *testing.T) {
	r4n := r4.HumanName{
		Use:    "official",
		Text:   "Dr Jane Doe",
		Family: "Doe",
		Given:  []string{"Jane"},
		Prefix: []string{"Dr"},
		Suffix: []string{"PhD"},
	}

	r5n := nameR4ToR5(r4n)
	assert.Equal(t, r4n.Use, r5n.Use)
	assert.Equal(t, r4n.Text, r5n.Text)
	assert.Equal(t, r4n.Family, r5n.Family)
	assert.Equal(t, r4n.Given, r5n.Given)

	r4n2 := nameR5ToR4(r5n)
	assert.Equal(t, r4n.Family, r4n2.Family)
}

func TestAddressR4ToR5_RoundTrip(t *testing.T) {
	r4a := r4.Address{
		Use:        "home",
		Text:       "123 Test St",
		Line:       []string{"123 Test St"},
		City:       "Wellington",
		PostalCode: "6011",
		Country:    "NZ",
	}

	r5a := addressR4ToR5(r4a)
	assert.Equal(t, r4a.Use, r5a.Use)
	assert.Equal(t, r4a.City, r5a.City)
	assert.Equal(t, r4a.Country, r5a.Country)

	r4a2 := addressR5ToR4(r5a)
	assert.Equal(t, r4a.City, r4a2.City)
}

func TestContactR4ToR5_RoundTrip(t *testing.T) {
	r4c := r4.ContactPoint{System: "phone", Value: "021-555-0123", Use: "mobile", Rank: 1}
	r5c := contactR4ToR5(r4c)
	assert.Equal(t, r4c.Value, r5c.Value)
	assert.Equal(t, r4c.Use, r5c.Use)

	r4c2 := contactR5ToR4(r5c)
	assert.Equal(t, r4c.Value, r4c2.Value)
}

func TestRefR4ToR5_RoundTrip(t *testing.T) {
	r4r := &r4.Reference{
		Reference: "Patient/123",
		Type:      "Patient",
		Display:   "John Smith",
	}

	r5r := refR4ToR5(r4r)
	require.NotNil(t, r5r)
	assert.Equal(t, r4r.Reference, r5r.Reference)
	assert.Equal(t, r4r.Type, r5r.Type)

	r4r2 := refR5ToR4(r5r)
	require.NotNil(t, r4r2)
	assert.Equal(t, r4r.Reference, r4r2.Reference)
}

func TestRefR4ToR5_Nil(t *testing.T) {
	assert.Nil(t, refR4ToR5(nil))
	assert.Nil(t, refR5ToR4(nil))
}

func TestExtensionsR4ToR5_RoundTrip(t *testing.T) {
	r4exts := []r4.Extension{{
		URL:         "http://hl7.org.nz/fhir/StructureDefinition/nz-ethnicity",
		ValueString: "European",
	}}

	r5exts := extensionsR4ToR5(r4exts)
	require.Len(t, r5exts, 1)
	assert.Equal(t, r4exts[0].URL, r5exts[0].URL)
	assert.Equal(t, r4exts[0].ValueString, r5exts[0].ValueString)

	r4exts2 := extensionsR5ToR4(r5exts)
	require.Len(t, r4exts2, 1)
	assert.Equal(t, r4exts[0].ValueString, r4exts2[0].ValueString)
}

func TestExtensionsR4ToR5_Nil(t *testing.T) {
	assert.Nil(t, extensionsR4ToR5(nil))
	assert.Nil(t, extensionsR5ToR4(nil))
}

func TestPatientWithCommunication(t *testing.T) {
	r4p := &r4.Patient{
		ID: "comm-test",
		Communication: []r4.PatientCommunication{{
			Language:  r4.CodeableConcept{Text: "English"},
			Preferred: true,
		}},
	}

	r5p := PatientR4ToR5(r4p)
	require.Len(t, r5p.Communication, 1)
	assert.Equal(t, "English", r5p.Communication[0].Language.Text)
	assert.True(t, r5p.Communication[0].Preferred)
}

func TestPatientWithContact(t *testing.T) {
	name := r4.HumanName{Family: "Smith", Given: []string{"Mary"}}
	addr := r4.Address{City: "Auckland"}
	r4p := &r4.Patient{
		ID: "contact-test",
		Contact: []r4.PatientContact{{
			Name:    &name,
			Gender:  "female",
			Address: &addr,
			Relationship: []r4.CodeableConcept{{
				Text: "Emergency Contact",
			}},
		}},
	}

	r5p := PatientR4ToR5(r4p)
	require.Len(t, r5p.Contact, 1)
	require.NotNil(t, r5p.Contact[0].Name)
	assert.Equal(t, "Smith", r5p.Contact[0].Name.Family)
	require.NotNil(t, r5p.Contact[0].Address)
	assert.Equal(t, "Auckland", r5p.Contact[0].Address.City)
}
