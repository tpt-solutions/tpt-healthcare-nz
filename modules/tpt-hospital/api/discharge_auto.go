package api

import (
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Auto-populate Discharge Summary from Admission/Coding/Pharmacy Data
// ---------------------------------------------------------------------------

// DischargeSummaryData holds the source data for auto-populating a discharge summary.
type DischargeSummaryData struct {
	Admission       Admission            `json:"admission"`
	Codes           []ClinicalCode       `json:"codes"`
	Medications     []InpatientMedication `json:"medications"`
}

// AutoPopulateDischargeSummary generates a discharge summary from admission data,
// clinical coding, and current medication chart.
func AutoPopulateDischargeSummary(data DischargeSummaryData) DischargeSummary {
	summary := DischargeSummary{
		AdmissionID:      data.Admission.ID,
		PatientID:        data.Admission.PatientID,
		AdmissionDate:    data.Admission.AdmittedAt,
		DischargeDate:    time.Now(),
		PrimaryDiagnosis: data.Admission.PrimaryDiagnosis,
		AuthorHPI:        data.Admission.AdmittingClinicianHPI,
	}

	// Build procedure and diagnosis lists from coding
	var procedures []string
	var secondaryDiags []string
	for _, code := range data.Codes {
		switch code.CodeType {
		case CodeTypeAdditionalDiagnosis:
			secondaryDiags = append(secondaryDiags, fmt.Sprintf("%s (%s)", code.Description, code.Code))
		case CodeTypePrincipalProcedure, CodeTypeAdditionalProcedure:
			procedures = append(procedures, fmt.Sprintf("%s (%s)", code.Description, code.Code))
		}
	}
	summary.SecondaryDiagnoses = secondaryDiags
	summary.ProceduresPerformed = procedures

	// Auto-populate discharge medications from active inpatient meds
	var meds []string
	for _, med := range data.Medications {
		if med.Status == InpatientMedStatusActive {
			meds = append(meds, fmt.Sprintf("%s %s %s %s", med.GenericName, med.Dose, med.Route, med.Frequency))
		}
	}
	summary.Medications = meds

	// Set follow-up from admission
	if data.Admission.DischargeDestination != "" {
		summary.FollowUpPlan = fmt.Sprintf("Follow up with GP within 7 days. Discharged to: %s", data.Admission.DischargeDestination)
	}

	// Build clinical summary
	var summaryParts []string
	if data.Admission.AdmissionReason != "" {
		summaryParts = append(summaryParts, "Admitted for: "+data.Admission.AdmissionReason)
	}
	if data.Admission.PrimaryDiagnosis != "" {
		summaryParts = append(summaryParts, "Primary diagnosis: "+data.Admission.PrimaryDiagnosis)
	}
	if len(secondaryDiags) > 0 {
		summaryParts = append(summaryParts, "Secondary diagnoses: "+strings.Join(secondaryDiags, "; "))
	}
	if len(procedures) > 0 {
		summaryParts = append(summaryParts, "Procedures: "+strings.Join(procedures, "; "))
	}
	summary.ClinicalSummary = strings.Join(summaryParts, ". ")

	summary.CreatedAt = time.Now()
	return summary
}

// Validate checks that a discharge summary has minimum required content.
func (s *DischargeSummary) Validate() error {
	if s.AdmissionID == "" {
		return fmt.Errorf("discharge: admission ID is required")
	}
	if s.PatientID == "" {
		return fmt.Errorf("discharge: patient ID is required")
	}
	if s.DischargeDate.IsZero() {
		return fmt.Errorf("discharge: discharge date is required")
	}
	return nil
}

// GPTransmissionReady checks if a discharge summary has all fields needed
// for GP transmission via GP2GP.
func (s *DischargeSummary) GPTransmissionReady() bool {
	return s.PatientID != "" &&
		s.AdmissionID != "" &&
		!s.DischargeDate.IsZero() &&
		s.PrimaryDiagnosis != ""
}
