package hl7

import (
	"fmt"
	"strings"
	"time"
)

// ORUResult represents an HL7 v2 ORU (Observation Result) message.
type ORUResult struct {
	PlacerOrderID   string
	FillerOrderID   string
	PatientID       string // NHI
	PatientName     string
	DOB             string
	Gender          string
	ResultStatus    string // F = final, P = preliminary, C = corrected
	ObservationCode string // LOINC code
	ObservationText string
	Value           string
	Units           string
	ReferenceRange  string
	AbnormalFlag    string // H, L, HH, LL, N, A
	ResultDateTime  time.Time
	PerformingLab   string
	OrderingBy      string
	ResultsNotes    string
}

// BuildORU generates a raw HL7 v2 ORU^R01 message string.
func BuildORU(result ORUResult) string {
	var segs []string

	status := result.ResultStatus
	if status == "" {
		status = "F"
	}
	abnormal := result.AbnormalFlag
	if abnormal == "" {
		abnormal = "N"
	}

	// MSH
	segs = append(segs, fmt.Sprintf("MSH|^~\\&|LIS|NZ|TPT|NZ|%s||ORU^R01|MSG00001|P|2.5.1|||AL|NE",
		time.Now().Format("20060102150405")))

	// PID
	segs = append(segs, fmt.Sprintf("PID|1||%s||%s||%s|%s",
		result.PatientID, result.PatientName, result.DOB, result.Gender))

	// OBR
	segs = append(segs, fmt.Sprintf("OBR|1|%s|%s|||||||||||||%s|||%s|||%s",
		result.PlacerOrderID, result.FillerOrderID,
		result.OrderingBy, result.ResultDateTime.Format("20060102150405"),
		status))

	// OBX
	segs = append(segs, fmt.Sprintf("OBX|1|%s|%s||%s|%s|%s|%s|%s|||||%s",
		obsType(result.Value), result.ObservationCode, result.ObservationText,
		result.Value, result.Units, result.ReferenceRange, abnormal, status))

	// NTE
	if result.ResultsNotes != "" {
		segs = append(segs, fmt.Sprintf("NTE|||%s", result.ResultsNotes))
	}

	return strings.Join(segs, "\r")
}

// BuildORULabResult creates a lab result message.
func BuildORULabResult(placerID, fillerID, patientID, name, dob, gender, code, text, value, units, refRange, abnormal string) string {
	return BuildORU(ORUResult{
		PlacerOrderID:   placerID,
		FillerOrderID:   fillerID,
		PatientID:       patientID,
		PatientName:     name,
		DOB:             dob,
		Gender:          gender,
		ResultStatus:    "F",
		ObservationCode: code,
		ObservationText: text,
		Value:           value,
		Units:           units,
		ReferenceRange:  refRange,
		AbnormalFlag:    abnormal,
		ResultDateTime:  time.Now(),
	})
}

func obsType(value string) string {
	for _, c := range value {
		if (c < '0' || c > '9') && c != '.' && c != '-' && c != '+' {
			return "ST"
		}
	}
	return "NM"
}
