package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/events"
	"github.com/PhillipC05/tpt-healthcare/core/push"
	"github.com/PhillipC05/tpt-healthcare/core/sms"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service orchestrates queue operations, publishing domain events and push notifications.
type Service struct {
	repo        Repository
	bus         *events.Bus
	notifier    *push.Notifier
	audit       *audit.Trail
	smsProvider sms.Provider
	pool        *pgxpool.Pool
	enc         *encryption.Cipher
	logger      *slog.Logger
}

// NewService creates a queue Service.
func NewService(repo Repository, bus *events.Bus, notifier *push.Notifier, audit *audit.Trail, logger *slog.Logger) *Service {
	return &Service{repo: repo, bus: bus, notifier: notifier, audit: audit, logger: logger}
}

// WithSMS attaches an SMS provider for fallback delivery of "called" notifications
// when push is unavailable. pool and enc are used to look up the patient's mobile.
func (s *Service) WithSMS(provider sms.Provider, pool *pgxpool.Pool, enc *encryption.Cipher) *Service {
	s.smsProvider = provider
	s.pool = pool
	s.enc = enc
	return s
}

// CheckIn adds a patient to the queue identified by queueID. patientNHI must
// be the encrypted NHI index (same format as patients.nhi_index).
func (s *Service) CheckIn(ctx context.Context, queueID uuid.UUID, patientID *uuid.UUID, patientNHI string, apptID *uuid.UUID, method CheckInMethod) (*QueueEntry, int, error) {
	pos, err := s.repo.NextPosition(ctx, queueID)
	if err != nil {
		return nil, 0, fmt.Errorf("service check-in: %w", err)
	}

	avgWait, _ := s.repo.AverageWaitMinutes(ctx, queueID)
	estWait := avgWait * (pos)

	entry := QueueEntry{
		QueueID:       queueID,
		PatientID:     patientID,
		PatientNHI:    patientNHI,
		AppointmentID: apptID,
		Position:      pos,
		CheckInMethod: method,
	}
	created, err := s.repo.CheckIn(ctx, entry)
	if err != nil {
		return nil, 0, fmt.Errorf("service check-in: %w", err)
	}

	payload, _ := json.Marshal(CheckedInPayload{
		EntryID:     created.ID,
		QueueID:     queueID,
		Position:    pos,
		EstWaitMins: estWait,
	})
	s.bus.PublishAsync(ctx, events.Event{
		ID:            uuid.New(),
		Type:          EventCheckedIn,
		AggregateID:   created.ID.String(),
		AggregateType: "QueueEntry",
		Payload:       json.RawMessage(payload),
		OccurredAt:    time.Now().UTC(),
	})

	return created, estWait, nil
}

// CallNext marks the earliest waiting entry as "called" and fires a push notification
// to the patient's registered devices.
func (s *Service) CallNext(ctx context.Context, queueID uuid.UUID, roomHint string) (*QueueEntry, error) {
	entry, err := s.repo.CallNext(ctx, queueID, roomHint)
	if err != nil {
		return nil, fmt.Errorf("service call-next: %w", err)
	}

	payload, _ := json.Marshal(CalledPayload{
		EntryID:   entry.ID,
		QueueID:   queueID,
		PatientID: ptrOrNil(entry.PatientID),
		RoomHint:  roomHint,
	})
	s.bus.PublishAsync(ctx, events.Event{
		ID:            uuid.New(),
		Type:          EventCalled,
		AggregateID:   entry.ID.String(),
		AggregateType: "QueueEntry",
		Payload:       json.RawMessage(payload),
		OccurredAt:    time.Now().UTC(),
	})

	// Notify the patient they have been called. Push is primary; SMS is fallback.
	if entry.PatientID != nil {
		body := "Please come to reception"
		if roomHint != "" {
			body = "Please head to " + roomHint
		}
		go func(patientID uuid.UUID, body string) {
			bgCtx := context.Background()
			if err := s.notifier.Send(bgCtx, patientID, push.Notification{
				Title:           "Your turn!",
				Body:            body,
				Tag:             "queue-called",
				URL:             "/waiting",
				RequireInteract: true,
			}); err != nil {
				s.logger.Warn("push notification failed for called patient",
					slog.String("entryID", entry.ID.String()),
					slog.String("error", err.Error()),
				)
				// Fall back to SMS when configured.
				if s.smsProvider != nil && s.pool != nil && s.enc != nil {
					if mobile, mobErr := fetchPatientMobile(bgCtx, s.pool, s.enc, patientID); mobErr == nil {
						if _, smsErr := s.smsProvider.Send(bgCtx, sms.Message{
							To:        mobile,
							Body:      "TPT Health: " + body,
							Reference: "queue-called-" + entry.ID.String(),
						}); smsErr != nil {
							s.logger.Warn("SMS fallback failed for called patient",
								slog.String("entryID", entry.ID.String()),
								slog.String("error", smsErr.Error()),
							)
						}
					}
				}
			}
		}(*entry.PatientID, body)
	}

	return entry, nil
}

// UpdateStatus transitions a queue entry to a new status.
// For terminal statuses the patient's location is purged in the same transaction.
func (s *Service) UpdateStatus(ctx context.Context, entryID uuid.UUID, status EntryStatus) (*QueueEntry, error) {
	entry, err := s.repo.UpdateEntryStatus(ctx, entryID, status)
	if err != nil {
		return nil, fmt.Errorf("service update status: %w", err)
	}

	evType := ""
	switch status {
	case StatusDone:
		evType = EventDone
	case StatusSkipped:
		evType = EventSkipped
	case StatusLeft:
		evType = EventLeft
	}
	if evType != "" {
		payload, _ := json.Marshal(map[string]string{"entryId": entryID.String()})
		s.bus.PublishAsync(ctx, events.Event{
			ID:            uuid.New(),
			Type:          evType,
			AggregateID:   entryID.String(),
			AggregateType: "QueueEntry",
			Payload:       json.RawMessage(payload),
			OccurredAt:    time.Now().UTC(),
		})
	}

	return entry, nil
}

// UpdateLocation saves an ephemeral GPS fix for a waiting patient.
func (s *Service) UpdateLocation(ctx context.Context, queueID uuid.UUID, entryID uuid.UUID, lat, lng float64, accuracyM float32) error {
	loc := Location{
		EntryID:   entryID,
		Lat:       lat,
		Lng:       lng,
		AccuracyM: accuracyM,
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.repo.UpsertLocation(ctx, loc); err != nil {
		return fmt.Errorf("service update location: %w", err)
	}

	payload, _ := json.Marshal(LocationPayload{
		EntryID:   entryID,
		QueueID:   queueID,
		Lat:       lat,
		Lng:       lng,
		AccuracyM: accuracyM,
	})
	s.bus.PublishAsync(ctx, events.Event{
		ID:            uuid.New(),
		Type:          EventLocationUpdated,
		AggregateID:   entryID.String(),
		AggregateType: "QueueEntry",
		Payload:       json.RawMessage(payload),
		OccurredAt:    time.Now().UTC(),
	})
	return nil
}

func ptrOrNil(p *uuid.UUID) uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return *p
}
