package hl7

import (
	"fmt"
	"strings"
	"time"
)

// ADTEvent represents an HL7 v2 ADT (Admit/Transfer/Discharge) message.
type ADTEvent struct {
	Trigger            string     // A01, A02, A03, A04, A08
	PatientID          string     // NHI
	PatientName        string
	DOB                string     // YYYYMMDD
	Gender             string     // M, F, O
	AdmitDateTime      time.Time
	DischargeDateTime  *time.Time
	AttendingDoctor    string     // HPI CPN
	AssignedWard       string
	AssignedBed        string
	PatientClass       string     // Inpatient, Emergency, Outpatient
	AdmissionType      string     // Emergency, Elective, Urgent, Newborn
	DischargeDisposition string
}

// BuildADT generates a raw HL7 v2 ADT message string from an ADTEvent.
func BuildADT(evt ADTEvent) string {
	var segs []string

	// MSH — Message Header
	segs = append(segs, fmt.Sprintf("MSH|^~\\&|TPT|NZ|LIS|NZ|%s||ADT^%s|MSG00001|P|2.5.1|||AL|NE",
		time.Now().Format("20060102150405"), evt.Trigger))

	// EVN — Event Type
	segs = append(segs, fmt.Sprintf("EVN|%s|%s", evt.Trigger, time.Now().Format("20060102150405")))

	// PID — Patient Identification
	segs = append(segs, fmt.Sprintf("PID|1||%s||%s||%s|%s|||||||||||%s",
		evt.PatientID, evt.PatientName, evt.DOB, evt.Gender, evt.PatientID))

	// PV1 — Patient Visit
	if evt.AssignedWard != "" {
		segs = append(segs, fmt.Sprintf("PV1|1|%s|||%s^%s|||||||||||||||||||||||||||||||||||||||||%s",
			evt.PatientClass, evt.AssignedWard, evt.AssignedBed, evt.AttendingDoctor))
	} else {
		segs = append(segs, fmt.Sprintf("PV1|1|%s|||||||||||||||||||||%s",
			evt.PatientClass, evt.AttendingDoctor))
	}

	return strings.Join(segs, "\r")
}

// BuildADTAdmit creates an ADT^A01 (Admit) message.
func BuildADTAdmit(patientID, name, dob, gender, ward, bed, attending string) string {
	return BuildADT(ADTEvent{
		Trigger:         "A01",
		PatientID:       patientID,
		PatientName:     name,
		DOB:             dob,
		Gender:          gender,
		AdmitDateTime:   time.Now(),
		AssignedWard:    ward,
		AssignedBed:     bed,
		PatientClass:    "Inpatient",
		AdmissionType:   "Emergency",
		AttendingDoctor: attending,
	})
}

// BuildADTTransfer creates an ADT^A02 (Transfer) message.
func BuildADTTransfer(patientID, name, dob, gender, newWard, newBed, attending string) string {
	return BuildADT(ADTEvent{
		Trigger:         "A02",
		PatientID:       patientID,
		PatientName:     name,
		DOB:             dob,
		Gender:          gender,
		AdmitDateTime:   time.Now(),
		AssignedWard:    newWard,
		AssignedBed:     newBed,
		PatientClass:    "Inpatient",
		AttendingDoctor: attending,
	})
}

// BuildADTDischarge creates an ADT^A03 (Discharge) message.
func BuildADTDischarge(patientID, name, dob, gender, disposition, attending string) string {
	now := time.Now()
	return BuildADT(ADTEvent{
		Trigger:              "A03",
		PatientID:            patientID,
		PatientName:          name,
		DOB:                  dob,
		Gender:               gender,
		AdmitDateTime:        now,
		DischargeDateTime:    &now,
		PatientClass:         "Inpatient",
		AttendingDoctor:      attending,
		DischargeDisposition: disposition,
	})
}

// BuildADTUpdate creates an ADT^A08 (Update) message.
func BuildADTUpdate(patientID, name, dob, gender, attending string) string {
	return BuildADT(ADTEvent{
		Trigger:         "A08",
		PatientID:       patientID,
		PatientName:     name,
		DOB:             dob,
		Gender:          gender,
		AdmitDateTime:   time.Now(),
		PatientClass:    "Inpatient",
		AttendingDoctor: attending,
	})
}
