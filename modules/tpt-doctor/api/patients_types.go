package api

import (
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
)

// patientRecord is the internal representation stored in the database.
// PHI fields (nhiEncrypted, name, dob) are AES-256-GCM encrypted at rest.
type patientRecord struct {
	ID           string    `json:"id"`
	NHIEncrypted []byte    `json:"-"`
	NHI          string    `json:"nhi,omitempty"` // plaintext, only populated after decryption
	TenantID     string    `json:"tenantId"`
	FHIRResource []byte    `json:"-"` // encrypted FHIR Patient JSON
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// patientResponse is the API response for a patient resource.
type patientResponse struct {
	ID        string      `json:"id"`
	NHI       string      `json:"nhi"`
	TenantID  string      `json:"tenantId"`
	Patient   *r5.Patient `json:"patient"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// patientCreateRequest is the request body for POST /api/v1/patients, matching
// the flat registration form submitted by the clinic UI (NewPatientPage). The
// NHI may be left blank when the Ministry has not yet assigned one.
type patientCreateRequest struct {
	NHI                          string `json:"nhi"`
	FirstName                    string `json:"firstName"`
	LastName                     string `json:"lastName"`
	DateOfBirth                  string `json:"dateOfBirth"` // YYYY-MM-DD
	Gender                       string `json:"gender"`
	Ethnicity                    string `json:"ethnicity"`
	Phone                        string `json:"phone"`
	Email                        string `json:"email"`
	AddressLine1                 string `json:"addressLine1"`
	AddressLine2                 string `json:"addressLine2"`
	Suburb                       string `json:"suburb"`
	City                         string `json:"city"`
	Postcode                     string `json:"postcode"`
	EmergencyContactName         string `json:"emergencyContactName"`
	EmergencyContactPhone        string `json:"emergencyContactPhone"`
	EmergencyContactRelationship string `json:"emergencyContactRelationship"`
}

// toFHIRPatient builds a FHIR R5 Patient resource from the flat registration
// form fields.
func (req *patientCreateRequest) toFHIRPatient() *r5.Patient {
	active := true
	patient := &r5.Patient{
		ResourceType: "Patient",
		Active:       &active,
		Name: []r5.HumanName{{
			Use:    "official",
			Family: req.LastName,
			Given:  []string{req.FirstName},
		}},
		Gender:    req.Gender,
		BirthDate: req.DateOfBirth,
	}

	if req.NHI != "" {
		patient.Identifier = []r5.Identifier{{
			System: r5.NHISystemURL,
			Value:  req.NHI,
		}}
	}

	if req.Phone != "" {
		patient.Telecom = append(patient.Telecom, r5.ContactPoint{System: "phone", Value: req.Phone, Use: "mobile"})
	}
	if req.Email != "" {
		patient.Telecom = append(patient.Telecom, r5.ContactPoint{System: "email", Value: req.Email})
	}

	if req.AddressLine1 != "" || req.Suburb != "" || req.City != "" {
		var lines []string
		if req.AddressLine1 != "" {
			lines = append(lines, req.AddressLine1)
		}
		if req.AddressLine2 != "" {
			lines = append(lines, req.AddressLine2)
		}
		patient.Address = []r5.Address{{
			Line:       lines,
			District:   req.Suburb,
			City:       req.City,
			PostalCode: req.Postcode,
			Country:    "NZ",
		}}
	}

	if req.Ethnicity != "" {
		patient.Extension = append(patient.Extension, r5.Extension{
			URL:         r5.NZEthnicityExtURL,
			ValueString: req.Ethnicity,
		})
	}

	if req.EmergencyContactName != "" {
		contact := r5.PatientContact{
			Name: &r5.HumanName{Text: req.EmergencyContactName},
		}
		if req.EmergencyContactRelationship != "" {
			contact.Relationship = []r5.CodeableConcept{{Text: req.EmergencyContactRelationship}}
		}
		if req.EmergencyContactPhone != "" {
			contact.Telecom = []r5.ContactPoint{{System: "phone", Value: req.EmergencyContactPhone}}
		}
		patient.Contact = append(patient.Contact, contact)
	}

	return patient
}

// patientUpdateRequest is the request body for PUT /api/v1/patients/{id}.
type patientUpdateRequest struct {
	Patient *r5.Patient `json:"patient"`
}

// enrolmentRequest is the request body for POST and PUT /api/v1/patients/{id}/enrolment.
type enrolmentRequest struct {
	// PractitionerHPI is the individual practitioner's HPI Common Person Number (CPN).
	PractitionerHPI string `json:"practitionerHpi"`
	// PracticeHPI is the HPI facility OrgID of the enrolling practice.
	// Required for UpdateEnrolment; ignored by CreateEnrolment which derives the
	// practice from the authenticated tenant.
	PracticeHPI string `json:"practiceHpi"`
	FundingCode string `json:"fundingCode"`
	StartDate   string `json:"startDate"` // YYYY-MM-DD
}

// transferRequest is the body for POST /api/v1/patients/{id}/enrolment/transfer.
type transferRequest struct {
	ToPractitionerHPI string `json:"toPractitionerHpi"`
	FundingCode       string `json:"fundingCode,omitempty"`
	TransferDate      string `json:"transferDate"` // YYYY-MM-DD
	Reason            string `json:"reason,omitempty"`
}
