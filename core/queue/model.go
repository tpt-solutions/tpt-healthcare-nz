package queue

import (
	"time"

	"github.com/google/uuid"
)

// Status values for a Queue.
type QueueStatus string

const (
	QueueOpen   QueueStatus = "open"
	QueuePaused QueueStatus = "paused"
	QueueClosed QueueStatus = "closed"
)

// EntryStatus values for a QueueEntry.
type EntryStatus string

const (
	StatusWaiting    EntryStatus = "waiting"
	StatusCalled     EntryStatus = "called"
	StatusInProgress EntryStatus = "in_progress"
	StatusDone       EntryStatus = "done"
	StatusSkipped    EntryStatus = "skipped"
	StatusLeft       EntryStatus = "left"
)

// IsTerminal returns true if no further state transitions are expected.
func (s EntryStatus) IsTerminal() bool {
	return s == StatusDone || s == StatusSkipped || s == StatusLeft
}

// CheckInMethod records how a patient joined the queue.
type CheckInMethod string

const (
	CheckInPortal    CheckInMethod = "portal"
	CheckInKiosk     CheckInMethod = "kiosk"
	CheckInStaff     CheckInMethod = "staff"
	CheckInEmergency CheckInMethod = "emergency"
)

// Queue represents one clinic's waiting list for a given day.
type Queue struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Date      time.Time
	Status    QueueStatus
	CreatedAt time.Time
}

// QueueEntry is a single patient's place in a queue.
type QueueEntry struct {
	ID            uuid.UUID
	QueueID       uuid.UUID
	PatientID     *uuid.UUID // nil for anonymous emergency check-ins
	PatientNHI    string     // encrypted NHI index
	AppointmentID *uuid.UUID
	Position      int
	Status        EntryStatus
	CheckedInAt   time.Time
	CalledAt      *time.Time
	DoneAt        *time.Time
	WaitMinutes   *int
	CheckInMethod CheckInMethod
	RoomHint      string // e.g. "Room 2", set by staff when calling
	Notes         string
}

// Location is an ephemeral GPS fix for a waiting patient.
type Location struct {
	EntryID    uuid.UUID
	Lat        float64
	Lng        float64
	AccuracyM  float32
	UpdatedAt  time.Time
}

// EntryWithLocation bundles an entry and its optional current location.
type EntryWithLocation struct {
	Entry    QueueEntry
	Location *Location // nil if patient has not shared location
}

// Event type constants published to core/events.Bus.
const (
	EventCheckedIn       = "queue.entry.checked_in"
	EventCalled          = "queue.entry.called"
	EventLocationUpdated = "queue.entry.location_updated"
	EventDone            = "queue.entry.done"
	EventSkipped         = "queue.entry.skipped"
	EventLeft            = "queue.entry.left"
)

// CheckedInPayload is the event data for EventCheckedIn.
type CheckedInPayload struct {
	EntryID       uuid.UUID `json:"entryId"`
	QueueID       uuid.UUID `json:"queueId"`
	Position      int       `json:"position"`
	EstWaitMins   int       `json:"estimatedWaitMinutes"`
}

// CalledPayload is the event data for EventCalled.
type CalledPayload struct {
	EntryID   uuid.UUID `json:"entryId"`
	QueueID   uuid.UUID `json:"queueId"`
	PatientID uuid.UUID `json:"patientId"`
	RoomHint  string    `json:"room,omitempty"`
}

// LocationPayload is the event data for EventLocationUpdated (staff stream only).
type LocationPayload struct {
	EntryID   uuid.UUID `json:"entryId"`
	QueueID   uuid.UUID `json:"queueId"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	AccuracyM float32   `json:"accuracyM,omitempty"`
}
