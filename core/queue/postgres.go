package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a Repository backed by PostgreSQL.
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepo{pool: pool}
}

func (r *postgresRepo) CreateQueue(ctx context.Context, tenantID uuid.UUID, name string, date time.Time) (*Queue, error) {
	q := &Queue{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      name,
		Date:      date,
		Status:    QueueOpen,
		CreatedAt: time.Now().UTC(),
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO queues (id, tenant_id, name, date, status, created_at)
		VALUES (@id, @tenant_id, @name, @date, @status, @created_at)
	`, pgx.NamedArgs{
		"id": q.ID, "tenant_id": q.TenantID, "name": q.Name,
		"date": q.Date, "status": string(q.Status), "created_at": q.CreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("queue create: %w", err)
	}
	return q, nil
}

func (r *postgresRepo) GetQueue(ctx context.Context, id uuid.UUID) (*Queue, error) {
	var q Queue
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, date, status, created_at
		FROM   queues WHERE id = @id
	`, pgx.NamedArgs{"id": id}).
		Scan(&q.ID, &q.TenantID, &q.Name, &q.Date, &status, &q.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrQueueNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("queue get: %w", err)
	}
	q.Status = QueueStatus(status)
	return &q, nil
}

func (r *postgresRepo) GetOrCreateTodayQueue(ctx context.Context, tenantID uuid.UUID, name string) (*Queue, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var q Queue
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, date, status, created_at
		FROM   queues
		WHERE  tenant_id = @tenant_id AND name = @name AND date = @date
	`, pgx.NamedArgs{"tenant_id": tenantID, "name": name, "date": today}).
		Scan(&q.ID, &q.TenantID, &q.Name, &q.Date, &status, &q.CreatedAt)
	if err == nil {
		q.Status = QueueStatus(status)
		return &q, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("queue get-or-create: %w", err)
	}
	return r.CreateQueue(ctx, tenantID, name, today)
}

func (r *postgresRepo) NextPosition(ctx context.Context, queueID uuid.UUID) (int, error) {
	var max int
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), 0) FROM queue_entries WHERE queue_id = @queue_id
	`, pgx.NamedArgs{"queue_id": queueID}).Scan(&max)
	if err != nil {
		return 0, fmt.Errorf("queue next position: %w", err)
	}
	return max + 1, nil
}

func (r *postgresRepo) CheckIn(ctx context.Context, entry QueueEntry) (*QueueEntry, error) {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CheckedInAt.IsZero() {
		entry.CheckedInAt = time.Now().UTC()
	}
	entry.Status = StatusWaiting
	_, err := r.pool.Exec(ctx, `
		INSERT INTO queue_entries
			(id, queue_id, patient_id, patient_nhi, appointment_id, position, status,
			 checked_in_at, check_in_method, notes)
		VALUES
			(@id, @queue_id, @patient_id, @patient_nhi, @appointment_id, @position, @status,
			 @checked_in_at, @check_in_method, @notes)
	`, pgx.NamedArgs{
		"id": entry.ID, "queue_id": entry.QueueID,
		"patient_id":      entry.PatientID,
		"patient_nhi":     entry.PatientNHI,
		"appointment_id":  entry.AppointmentID,
		"position":        entry.Position,
		"status":          string(entry.Status),
		"checked_in_at":   entry.CheckedInAt,
		"check_in_method": string(entry.CheckInMethod),
		"notes":           entry.Notes,
	})
	if err != nil {
		return nil, fmt.Errorf("queue check-in: %w", err)
	}
	return &entry, nil
}

func (r *postgresRepo) GetEntry(ctx context.Context, id uuid.UUID) (*QueueEntry, error) {
	e, _, err := r.scanEntry(ctx, id)
	return e, err
}

func (r *postgresRepo) scanEntry(ctx context.Context, id uuid.UUID) (*QueueEntry, *Location, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT e.id, e.queue_id, e.patient_id, e.patient_nhi, e.appointment_id, e.position,
		       e.status, e.checked_in_at, e.called_at, e.done_at, e.wait_minutes,
		       e.check_in_method, e.room_hint, e.notes,
		       l.lat, l.lng, l.accuracy_m, l.updated_at
		FROM   queue_entries e
		LEFT JOIN queue_entry_locations l ON l.entry_id = e.id
		WHERE  e.id = @id
	`, pgx.NamedArgs{"id": id})

	var e QueueEntry
	var loc Location
	var hasLoc bool
	var status, method string
	err := row.Scan(
		&e.ID, &e.QueueID, &e.PatientID, &e.PatientNHI, &e.AppointmentID, &e.Position,
		&status, &e.CheckedInAt, &e.CalledAt, &e.DoneAt, &e.WaitMinutes,
		&method, &e.RoomHint, &e.Notes,
		nilFloat64(&loc.Lat, &hasLoc), nilFloat64(&loc.Lng, nil), nilFloat32(&loc.AccuracyM), nilTime(&loc.UpdatedAt),
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrEntryNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("queue scan entry: %w", err)
	}
	e.Status = EntryStatus(status)
	e.CheckInMethod = CheckInMethod(method)
	loc.EntryID = e.ID
	if hasLoc {
		return &e, &loc, nil
	}
	return &e, nil, nil
}

func (r *postgresRepo) CallNext(ctx context.Context, queueID uuid.UUID, roomHint string) (*QueueEntry, error) {
	now := time.Now().UTC()
	var e QueueEntry
	var status string
	err := r.pool.QueryRow(ctx, `
		UPDATE queue_entries
		SET    status = 'called', called_at = @now, room_hint = @room_hint
		WHERE  id = (
			SELECT id FROM queue_entries
			WHERE  queue_id = @queue_id AND status = 'waiting'
			ORDER  BY position
			LIMIT  1
		)
		RETURNING id, queue_id, patient_id, patient_nhi, appointment_id, position,
		          status, checked_in_at, called_at, done_at, wait_minutes,
		          check_in_method, room_hint, notes
	`, pgx.NamedArgs{"queue_id": queueID, "now": now, "room_hint": roomHint}).
		Scan(&e.ID, &e.QueueID, &e.PatientID, &e.PatientNHI, &e.AppointmentID, &e.Position,
			&status, &e.CheckedInAt, &e.CalledAt, &e.DoneAt, &e.WaitMinutes,
			new(string), &e.RoomHint, &e.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoWaitingEntries
	}
	if err != nil {
		return nil, fmt.Errorf("queue call-next: %w", err)
	}
	e.Status = EntryStatus(status)
	return &e, nil
}

func (r *postgresRepo) UpdateEntryStatus(ctx context.Context, entryID uuid.UUID, status EntryStatus) (*QueueEntry, error) {
	now := time.Now().UTC()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("queue update status begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var e QueueEntry
	var dbStatus string
	checkedAt := e.CheckedInAt
	err = tx.QueryRow(ctx, `
		UPDATE queue_entries
		SET    status  = @status,
		       done_at = CASE WHEN @status IN ('done','skipped','left') THEN @now ELSE done_at END,
		       wait_minutes = CASE WHEN @status = 'done'
		                     THEN EXTRACT(EPOCH FROM (@now - checked_in_at))::INT / 60
		                     ELSE wait_minutes END
		WHERE  id = @id
		RETURNING id, queue_id, patient_id, patient_nhi, appointment_id, position,
		          status, checked_in_at, called_at, done_at, wait_minutes,
		          check_in_method, room_hint, notes
	`, pgx.NamedArgs{"id": entryID, "status": string(status), "now": now}).
		Scan(&e.ID, &e.QueueID, &e.PatientID, &e.PatientNHI, &e.AppointmentID, &e.Position,
			&dbStatus, &checkedAt, &e.CalledAt, &e.DoneAt, &e.WaitMinutes,
			new(string), &e.RoomHint, &e.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEntryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("queue update status: %w", err)
	}
	e.Status = EntryStatus(dbStatus)
	e.CheckedInAt = checkedAt

	// Remove location data as soon as the entry reaches a terminal status (HIPC Rule 6).
	if EntryStatus(dbStatus).IsTerminal() {
		if _, err := tx.Exec(ctx, `DELETE FROM queue_entry_locations WHERE entry_id = @id`,
			pgx.NamedArgs{"id": entryID}); err != nil {
			return nil, fmt.Errorf("queue purge location: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("queue update status commit: %w", err)
	}
	return &e, nil
}

func (r *postgresRepo) UpsertLocation(ctx context.Context, loc Location) error {
	// Verify the entry is still active before accepting a location update.
	var status string
	err := r.pool.QueryRow(ctx, `SELECT status FROM queue_entries WHERE id = @id`,
		pgx.NamedArgs{"id": loc.EntryID}).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEntryNotFound
	}
	if err != nil {
		return fmt.Errorf("queue upsert location check: %w", err)
	}
	if EntryStatus(status).IsTerminal() {
		return ErrEntryNotActive
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO queue_entry_locations (entry_id, lat, lng, accuracy_m, updated_at)
		VALUES (@entry_id, @lat, @lng, @accuracy_m, now())
		ON CONFLICT (entry_id)
		DO UPDATE SET lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		              accuracy_m = EXCLUDED.accuracy_m, updated_at = now()
	`, pgx.NamedArgs{
		"entry_id": loc.EntryID, "lat": loc.Lat, "lng": loc.Lng, "accuracy_m": loc.AccuracyM,
	})
	if err != nil {
		return fmt.Errorf("queue upsert location: %w", err)
	}
	return nil
}

func (r *postgresRepo) ListEntries(ctx context.Context, queueID uuid.UUID) ([]EntryWithLocation, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.queue_id, e.patient_id, e.patient_nhi, e.appointment_id, e.position,
		       e.status, e.checked_in_at, e.called_at, e.done_at, e.wait_minutes,
		       e.check_in_method, e.room_hint, e.notes,
		       l.lat, l.lng, l.accuracy_m, l.updated_at
		FROM   queue_entries e
		LEFT JOIN queue_entry_locations l ON l.entry_id = e.id
		WHERE  e.queue_id = @queue_id
		ORDER  BY e.position
	`, pgx.NamedArgs{"queue_id": queueID})
	if err != nil {
		return nil, fmt.Errorf("queue list entries: %w", err)
	}
	defer rows.Close()

	var results []EntryWithLocation
	for rows.Next() {
		var e QueueEntry
		var loc Location
		var hasLoc bool
		var status, method string
		if err := rows.Scan(
			&e.ID, &e.QueueID, &e.PatientID, &e.PatientNHI, &e.AppointmentID, &e.Position,
			&status, &e.CheckedInAt, &e.CalledAt, &e.DoneAt, &e.WaitMinutes,
			&method, &e.RoomHint, &e.Notes,
			nilFloat64(&loc.Lat, &hasLoc), nilFloat64(&loc.Lng, nil), nilFloat32(&loc.AccuracyM), nilTime(&loc.UpdatedAt),
		); err != nil {
			return nil, fmt.Errorf("queue list scan: %w", err)
		}
		e.Status = EntryStatus(status)
		e.CheckInMethod = CheckInMethod(method)
		loc.EntryID = e.ID
		ewl := EntryWithLocation{Entry: e}
		if hasLoc {
			ewl.Location = &loc
		}
		results = append(results, ewl)
	}
	return results, rows.Err()
}

func (r *postgresRepo) AverageWaitMinutes(ctx context.Context, queueID uuid.UUID) (int, error) {
	var avg int
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(wait_minutes)::INT, 15)
		FROM   queue_entries
		WHERE  queue_id = @queue_id AND status = 'done' AND done_at > now() - INTERVAL '2 hours'
	`, pgx.NamedArgs{"queue_id": queueID}).Scan(&avg)
	if err != nil {
		return 15, fmt.Errorf("queue avg wait: %w", err)
	}
	return avg, nil
}

// Nullable scan helpers — pgx does not auto-scan NULL into *float64 without these.
func nilFloat64(dest *float64, flag *bool) any {
	return pgx.ScanArgFunc(func(src any) error {
		if src == nil {
			if flag != nil {
				*flag = false
			}
			return nil
		}
		if flag != nil {
			*flag = true
		}
		switch v := src.(type) {
		case float64:
			*dest = v
		case float32:
			*dest = float64(v)
		}
		return nil
	})
}

func nilFloat32(dest *float32) any {
	return pgx.ScanArgFunc(func(src any) error {
		if src == nil {
			return nil
		}
		switch v := src.(type) {
		case float64:
			*dest = float32(v)
		case float32:
			*dest = v
		}
		return nil
	})
}

func nilTime(dest *time.Time) any {
	return pgx.ScanArgFunc(func(src any) error {
		if src == nil {
			return nil
		}
		if t, ok := src.(time.Time); ok {
			*dest = t
		}
		return nil
	})
}
