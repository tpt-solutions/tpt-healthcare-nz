package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/push"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// ReminderWorker polls for upcoming appointments and sends push reminders.
type ReminderWorker struct {
	pool     *pgxpool.Pool
	notifier *push.Notifier
	logger   *slog.Logger
	interval time.Duration
}

// NewReminderWorker creates a ReminderWorker that polls every interval.
func NewReminderWorker(pool *pgxpool.Pool, notifier *push.Notifier, logger *slog.Logger, interval time.Duration) *ReminderWorker {
	if interval <= 0 {
		interval = time.Minute
	}
	return &ReminderWorker{pool: pool, notifier: notifier, logger: logger, interval: interval}
}

// Run starts the reminder polling loop. Blocks until ctx is cancelled.
func (w *ReminderWorker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "reminder worker started", slog.Duration("interval", w.interval))
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately on start, then on each tick.
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.InfoContext(ctx, "reminder worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *ReminderWorker) tick(ctx context.Context) {
	now := time.Now().UTC()

	// 24h window: appointments starting between 23h50m and 24h10m from now.
	window24Low := now.Add(23*time.Hour + 50*time.Minute)
	window24High := now.Add(24*time.Hour + 10*time.Minute)

	// 1h window: appointments starting between 50m and 70m from now.
	window1Low := now.Add(50 * time.Minute)
	window1High := now.Add(70 * time.Minute)

	appts, err := w.fetchDue(ctx, window24Low, window24High, window1Low, window1High)
	if err != nil {
		w.logger.ErrorContext(ctx, "reminder fetch failed", slog.String("error", err.Error()))
		return
	}

	for _, appt := range appts {
		w.dispatch(ctx, appt, now)
	}
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
		return
	}

	// Mark sent so we don't re-dispatch on the next tick.
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

// nzLocation returns the New Zealand/Auckland timezone, falling back to UTC.
func nzLocation() *time.Location {
	loc, err := time.LoadLocation("Pacific/Auckland")
	if err != nil {
		return time.UTC
	}
	return loc
}
