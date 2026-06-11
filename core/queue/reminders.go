package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/push"
	"github.com/PhillipC05/tpt-healthcare/core/sms"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// upcomingAppt is a minimal appointment projection needed for reminder dispatch.
type upcomingAppt struct {
	ID              uuid.UUID
	PatientID       uuid.UUID
	PractitionerHPI string
	StartTime       time.Time
	Reminder24hSent bool
	Reminder1hSent  bool
}

// ReminderArgs is the River job payload for the appointment reminder worker.
// The job is scheduled as a periodic River job running every minute.
type ReminderArgs struct{}

func (ReminderArgs) Kind() string { return "queue.appointment_reminders" }

// ReminderWorker is a River worker that dispatches push (and SMS fallback)
// reminders for upcoming appointments.
// River handles scheduling, retries, and at-least-once delivery.
type ReminderWorker struct {
	river.WorkerDefaults[ReminderArgs]
	pool        *pgxpool.Pool
	notifier    *push.Notifier
	smsProvider sms.Provider
	enc         *encryption.Cipher
	logger      *slog.Logger
}

// NewReminderWorker creates a ReminderWorker.
func NewReminderWorker(pool *pgxpool.Pool, notifier *push.Notifier, logger *slog.Logger) *ReminderWorker {
	return &ReminderWorker{pool: pool, notifier: notifier, logger: logger}
}

// WithSMS attaches an SMS provider for fallback delivery when push notifications
// are unavailable. enc must be the same cipher used to encrypt patient records.
func (w *ReminderWorker) WithSMS(provider sms.Provider, enc *encryption.Cipher) *ReminderWorker {
	w.smsProvider = provider
	w.enc = enc
	return w
}

// Work is invoked by the River scheduler every minute. It fetches appointments
// in the 24h or 1h reminder windows and dispatches notifications.
func (w *ReminderWorker) Work(ctx context.Context, _ *river.Job[ReminderArgs]) error {
	now := time.Now().UTC()

	// 24h window: appointments starting between 23h50m and 24h10m from now.
	window24Low := now.Add(23*time.Hour + 50*time.Minute)
	window24High := now.Add(24*time.Hour + 10*time.Minute)

	// 1h window: appointments starting between 50m and 70m from now.
	window1Low := now.Add(50 * time.Minute)
	window1High := now.Add(70 * time.Minute)

	appts, err := w.fetchDue(ctx, window24Low, window24High, window1Low, window1High)
	if err != nil {
		return fmt.Errorf("reminder fetch: %w", err)
	}

	for _, appt := range appts {
		w.dispatch(ctx, appt, now)
	}
	return nil
}

func (w *ReminderWorker) fetchDue(ctx context.Context, w24Low, w24High, w1Low, w1High time.Time) ([]upcomingAppt, error) {
	rows, err := w.pool.Query(ctx, `
		SELECT id, patient_id, practitioner_hpi, start_time, reminder_24h_sent, reminder_1h_sent
		FROM   appointments
		WHERE  status NOT IN ('cancelled', 'noshow', 'fulfilled')
		  AND  (
		    (start_time BETWEEN @w24low AND @w24high AND reminder_24h_sent = false)
		    OR
		    (start_time BETWEEN @w1low AND @w1high  AND reminder_1h_sent  = false)
		  )
	`, pgx.NamedArgs{
		"w24low": w24Low, "w24high": w24High,
		"w1low": w1Low, "w1high": w1High,
	})
	if err != nil {
		return nil, fmt.Errorf("reminder fetch: %w", err)
	}
	defer rows.Close()

	var result []upcomingAppt
	for rows.Next() {
		var a upcomingAppt
		if err := rows.Scan(&a.ID, &a.PatientID, &a.PractitionerHPI, &a.StartTime, &a.Reminder24hSent, &a.Reminder1hSent); err != nil {
			return nil, fmt.Errorf("reminder scan: %w", err)
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (w *ReminderWorker) dispatch(ctx context.Context, appt upcomingAppt, now time.Time) {
	timeUntil := appt.StartTime.Sub(now)
	local := appt.StartTime.In(nzLocation())

	var notif push.Notification
	var markField string

	switch {
	case timeUntil > time.Hour && !appt.Reminder24hSent:
		notif = push.Notification{
			Title: "Appointment reminder",
			Body:  fmt.Sprintf("You have an appointment tomorrow at %s", local.Format("3:04 PM")),
			Tag:   "appt-reminder-24h",
			URL:   "/appointments",
		}
		markField = "reminder_24h_sent"

	case timeUntil <= time.Hour && !appt.Reminder1hSent:
		notif = push.Notification{
			Title: "Appointment in 1 hour",
			Body:  fmt.Sprintf("Your appointment is at %s — check in via the app when you arrive", local.Format("3:04 PM")),
			Tag:   "appt-reminder-1h",
			URL:   "/appointments",
		}
		markField = "reminder_1h_sent"

	default:
		return
	}

	if err := w.notifier.Send(ctx, appt.PatientID, notif); err != nil {
		w.logger.WarnContext(ctx, "appointment reminder push failed",
			slog.String("apptID", appt.ID.String()),
			slog.String("error", err.Error()),
		)
		// Fall back to SMS when a provider is configured.
		if w.smsProvider != nil && w.enc != nil {
			if mobile, mobErr := fetchPatientMobile(ctx, w.pool, w.enc, appt.PatientID); mobErr == nil {
				if _, smsErr := w.smsProvider.Send(ctx, sms.Message{
					To:        mobile,
					Body:      notif.Body,
					Reference: "appt-reminder-" + appt.ID.String(),
				}); smsErr != nil {
					w.logger.WarnContext(ctx, "appointment reminder SMS fallback failed",
						slog.String("apptID", appt.ID.String()),
						slog.String("error", smsErr.Error()),
					)
				}
			}
		}
		return
	}

	// Mark sent so River does not re-dispatch on the next run.
	if _, err := w.pool.Exec(ctx,
		fmt.Sprintf("UPDATE appointments SET %s = true WHERE id = $1", markField),
		appt.ID,
	); err != nil {
		w.logger.WarnContext(ctx, "reminder mark-sent failed",
			slog.String("apptID", appt.ID.String()),
			slog.String("error", err.Error()),
		)
	}
}

// Run starts a self-contained River client that executes this worker on a
// periodic schedule (every minute). It blocks until ctx is cancelled.
func (w *ReminderWorker) Run(ctx context.Context) {
	workers := river.NewWorkers()
	river.AddWorker(workers, w)

	riverClient, err := river.NewClient(riverpgxv5.New(w.pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 1},
		},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(time.Minute),
				func() (river.JobArgs, *river.InsertOpts) { return ReminderArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		},
	})
	if err != nil {
		w.logger.ErrorContext(ctx, "reminder worker: init", slog.String("error", err.Error()))
		return
	}
	if err := riverClient.Start(ctx); err != nil {
		w.logger.ErrorContext(ctx, "reminder worker: start", slog.String("error", err.Error()))
		return
	}
	<-ctx.Done()
	if err := riverClient.Stop(context.Background()); err != nil {
		w.logger.WarnContext(ctx, "reminder worker: stop", slog.String("error", err.Error()))
	}
}

// nzLocation returns the New Zealand/Auckland timezone, falling back to UTC.
func nzLocation() *time.Location {
	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		return time.UTC
	}
	return loc
}
