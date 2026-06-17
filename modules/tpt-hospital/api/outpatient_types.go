package api

import "time"

// OutpatientClinic represents a hospital-based specialist outpatient clinic.
type OutpatientClinic struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Specialty        string    `json:"specialty"` // e.g. "cardiology", "orthopaedics", "general surgery"
	LeadClinicianHPI string    `json:"leadClinicianHpi,omitempty"`
	Location         string    `json:"location,omitempty"` // building / room
	Active           bool      `json:"active"`
	TenantID         string    `json:"tenantId"`
	CreatedAt        time.Time `json:"createdAt"`
}

// OutpatientAppointmentStatus tracks an outpatient clinic appointment.
type OutpatientAppointmentStatus string

const (
	OPApptBooked    OutpatientAppointmentStatus = "booked"
	OPApptConfirmed OutpatientAppointmentStatus = "confirmed"
	OPApptAttended  OutpatientAppointmentStatus = "attended"
	OPApptDNAd      OutpatientAppointmentStatus = "did-not-attend"
	OPApptCancelled OutpatientAppointmentStatus = "cancelled"
)

// OutpatientAppointment is a booking in a hospital specialist clinic.
type OutpatientAppointment struct {
	ID           string                      `json:"id"`
	ClinicID     string                      `json:"clinicId"`
	PatientID    string                      `json:"patientId"`
	PatientNHI   string                      `json:"patientNhi"`
	ClinicianHPI string                      `json:"clinicianHpi"`
	Status       OutpatientAppointmentStatus `json:"status"`
	ReferralID   string                      `json:"referralId,omitempty"` // from tpt-doctor
	Reason       string                      `json:"reason"`
	Notes        string                      `json:"notes,omitempty"`
	ScheduledAt  time.Time                   `json:"scheduledAt"`
	AttendedAt   *time.Time                  `json:"attendedAt,omitempty"`
	TenantID     string                      `json:"tenantId"`
	CreatedAt    time.Time                   `json:"createdAt"`
	UpdatedAt    time.Time                   `json:"updatedAt"`
}

// WaitlistPriority classifies clinical urgency on the outpatient waitlist.
type WaitlistPriority string

const (
	WaitlistUrgent    WaitlistPriority = "urgent"      // < 4 weeks
	WaitlistSemUrgent WaitlistPriority = "semi-urgent" // 4–8 weeks
	WaitlistRoutine   WaitlistPriority = "routine"     // > 8 weeks
)

// WaitlistEntry represents a patient waiting for a specialist appointment.
type WaitlistEntry struct {
	ID            string           `json:"id"`
	ClinicID      string           `json:"clinicId"`
	PatientID     string           `json:"patientId"`
	PatientNHI    string           `json:"patientNhi"`
	Priority      WaitlistPriority `json:"priority"`
	Reason        string           `json:"reason"`
	ReferralID    string           `json:"referralId,omitempty"`
	AddedAt       time.Time        `json:"addedAt"`
	TargetDate    *time.Time       `json:"targetDate,omitempty"`
	AppointmentID string           `json:"appointmentId,omitempty"` // set when booked off waitlist
	TenantID      string           `json:"tenantId"`
}

type opAppointmentCreateRequest struct {
	PatientID    string    `json:"patientId"`
	PatientNHI   string    `json:"patientNhi"`
	ClinicianHPI string    `json:"clinicianHpi"`
	ReferralID   string    `json:"referralId,omitempty"`
	Reason       string    `json:"reason"`
	ScheduledAt  time.Time `json:"scheduledAt"`
}

type opAppointmentUpdateRequest struct {
	ClinicianHPI string     `json:"clinicianHpi,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	ScheduledAt  *time.Time `json:"scheduledAt,omitempty"`
}

type waitlistAddRequest struct {
	ClinicID   string           `json:"clinicId"`
	PatientID  string           `json:"patientId"`
	PatientNHI string           `json:"patientNhi"`
	Priority   WaitlistPriority `json:"priority"`
	Reason     string           `json:"reason"`
	ReferralID string           `json:"referralId,omitempty"`
	TargetDate *time.Time       `json:"targetDate,omitempty"`
}
