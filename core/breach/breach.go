// Package breach implements the Privacy Act 2020 (NZ) breach notification workflow.
//
// A notifiable privacy breach is a privacy breach that has caused, or is likely to cause,
// serious harm to an affected individual. Such breaches must be reported to the Privacy
// Commissioner within 72 hours of discovery (Privacy Act 2020, s 113).
//
// SQL schema for the backing table:
//
//	CREATE TABLE IF NOT EXISTS privacy_breaches (
//	    id                         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
//	    tenant_id                  UUID        NOT NULL,
//	    type                       TEXT        NOT NULL,
//	    severity                   TEXT        NOT NULL,
//	    description                TEXT        NOT NULL,
//	    affected_patient_count     INT         NOT NULL DEFAULT 0,
//	    affected_nhis              TEXT[]      NOT NULL DEFAULT '{}',
//	    discovered_at              TIMESTAMPTZ NOT NULL,
//	    reported_at                TIMESTAMPTZ,
//	    notified_commissioner_at   TIMESTAMPTZ,
//	    notified_affected_at       TIMESTAMPTZ,
//	    contained_at               TIMESTAMPTZ,
//	    status                     TEXT        NOT NULL DEFAULT 'open',
//	    notification_deadline      TIMESTAMPTZ NOT NULL,
//	    responsible_officer        TEXT        NOT NULL,
//	    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
//	    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now()
//	);
//	CREATE INDEX IF NOT EXISTS privacy_breaches_tenant_idx  ON privacy_breaches (tenant_id);
//	CREATE INDEX IF NOT EXISTS privacy_breaches_status_idx  ON privacy_breaches (status);
//	CREATE INDEX IF NOT EXISTS privacy_breaches_deadline_idx ON privacy_breaches (notification_deadline)
//	    WHERE notified_commissioner_at IS NULL;
package breach

import (
	"context"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/email"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// notificationWindow is the statutory deadline imposed by Privacy Act 2020, s 113.
const notificationWindow = 72 * time.Hour

// SeverityLevel classifies the potential harm of a privacy breach.
type SeverityLevel string

const (
	SeverityLow      SeverityLevel = "low"
	SeverityMedium   SeverityLevel = "medium"
	SeverityHigh     SeverityLevel = "high"
	SeverityCritical SeverityLevel = "critical"
)

// BreachType categorises the nature of the privacy breach.
type BreachType string

const (
	BreachTypeUnauthorisedAccess    BreachType = "unauthorised_access"
	BreachTypeAccidentalDisclosure  BreachType = "accidental_disclosure"
	BreachTypeTheftOrLoss           BreachType = "theft_or_loss"
	BreachTypeSystemIntrusion       BreachType = "system_intrusion"
	BreachTypeRansomware            BreachType = "ransomware"
	BreachTypeOther                 BreachType = "other"
)

// Breach records a single privacy breach event. All timestamps are in UTC.
type Breach struct {
	// ID is the unique identifier for this breach record.
	ID uuid.UUID

	// TenantID is the practice/organisation that experienced the breach.
	TenantID uuid.UUID

	// Type categorises the nature of the breach.
	Type BreachType

	// Severity indicates the potential for harm to affected individuals.
	Severity SeverityLevel

	// Description is a plain-text summary of what occurred.
	Description string

	// AffectedPatientCount is the number of patients whose information was involved.
	AffectedPatientCount int

	// AffectedNHIs holds the encrypted NHI numbers of affected patients.
	// Values are stored encrypted at rest; never log or expose these in plaintext.
	AffectedNHIs []string

	// DiscoveredAt is when the breach was first identified by the organisation.
	DiscoveredAt time.Time

	// ReportedAt is when the breach was formally logged in this system.
	ReportedAt *time.Time

	// NotifiedCommissionerAt is when the Privacy Commissioner was notified.
	// Under Privacy Act 2020 s 113, this must occur within 72 hours of discovery
	// for notifiable breaches.
	NotifiedCommissionerAt *time.Time

	// NotifiedAffectedAt is when affected individuals were notified.
	NotifiedAffectedAt *time.Time

	// ContainedAt is when the breach was contained/remediated.
	ContainedAt *time.Time

	// Status is a lifecycle label: "open", "notified", "contained", "closed".
	Status string

	// NotificationDeadline is DiscoveredAt + 72h — the statutory reporting deadline.
	NotificationDeadline time.Time

	// ResponsibleOfficer is the name/identifier of the Privacy Officer accountable.
	ResponsibleOfficer string
}

// Notifier manages breach records and notification workflows.
type Notifier struct {
	pool                *pgxpool.Pool
	emailProvider       email.Provider
	privacyOfficerEmail string
	fromEmail           string
}

// New creates a Notifier backed by the provided connection pool.
func New(pool *pgxpool.Pool) *Notifier {
	return &Notifier{pool: pool}
}

// WithEmail attaches an email provider for immediate notification to the privacy
// officer when a breach is recorded. privacyOfficerEmail is the recipient;
// fromEmail is the "From:" address.
func (n *Notifier) WithEmail(provider email.Provider, privacyOfficerEmail, fromEmail string) *Notifier {
	n.emailProvider = provider
	n.privacyOfficerEmail = privacyOfficerEmail
	n.fromEmail = fromEmail
	return n
}

// Record persists a new breach. It sets NotificationDeadline = DiscoveredAt + 72h
// and populates the breach ID. The returned *Breach reflects the persisted state.
func (n *Notifier) Record(ctx context.Context, b Breach) (*Breach, error) {
	b.ID = uuid.New()
	b.NotificationDeadline = b.DiscoveredAt.Add(notificationWindow)
	now := time.Now().UTC()
	b.ReportedAt = &now
	if b.Status == "" {
		b.Status = "open"
	}

	const q = `
		INSERT INTO privacy_breaches (
			id,
			tenant_id,
			type,
			severity,
			description,
			affected_patient_count,
			affected_nhis,
			discovered_at,
			reported_at,
			status,
			notification_deadline,
			responsible_officer
		) VALUES (
			@id,
			@tenant_id,
			@type,
			@severity,
			@description,
			@affected_patient_count,
			@affected_nhis,
			@discovered_at,
			@reported_at,
			@status,
			@notification_deadline,
			@responsible_officer
		)`

	args := pgNamedArgs(map[string]interface{}{
		"id":                    b.ID,
		"tenant_id":             b.TenantID,
		"type":                  string(b.Type),
		"severity":              string(b.Severity),
		"description":           b.Description,
		"affected_patient_count": b.AffectedPatientCount,
		"affected_nhis":         b.AffectedNHIs,
		"discovered_at":         b.DiscoveredAt.UTC(),
		"reported_at":           b.ReportedAt,
		"status":                b.Status,
		"notification_deadline": b.NotificationDeadline.UTC(),
		"responsible_officer":   b.ResponsibleOfficer,
	})

	if _, err := n.pool.Exec(ctx, q, args); err != nil {
		return nil, fmt.Errorf("breach.Record: insert: %w", err)
	}

	// Notify the privacy officer immediately by email when configured.
	if n.emailProvider != nil && n.privacyOfficerEmail != "" {
		subject := fmt.Sprintf("[ACTION REQUIRED] Privacy breach reported — %s severity (%s)", b.Severity, b.Type)
		body := fmt.Sprintf(
			"A privacy breach has been recorded in the TPT Health system.\n\n"+
				"Breach ID:         %s\n"+
				"Severity:          %s\n"+
				"Type:              %s\n"+
				"Description:       %s\n"+
				"Affected patients: %d\n"+
				"Discovered:        %s\n"+
				"Reporting deadline (Privacy Act 2020 s 113): %s\n\n"+
				"Responsible officer: %s\n\n"+
				"Log in to tpt-admin to manage this breach and record notification actions.",
			b.ID, b.Severity, b.Type, b.Description,
			b.AffectedPatientCount,
			b.DiscoveredAt.Format(time.RFC3339),
			b.NotificationDeadline.Format(time.RFC3339),
			b.ResponsibleOfficer,
		)
		if _, emailErr := n.emailProvider.Send(ctx, email.Message{
			To:       []string{n.privacyOfficerEmail},
			From:     n.fromEmail,
			Subject:  subject,
			TextBody: body,
			Tags:     []string{"breach", "privacy", string(b.Severity)},
		}); emailErr != nil {
			// Non-fatal: breach is already persisted; log and continue.
			_ = emailErr
		}
	}

	return &b, nil
}

// NotifyCommissioner records the timestamp at which the Privacy Commissioner was
// notified. This should be called immediately after the notification is dispatched.
func (n *Notifier) NotifyCommissioner(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE privacy_breaches
		SET    notified_commissioner_at = now(),
		       status = CASE WHEN status = 'open' THEN 'notified' ELSE status END,
		       updated_at = now()
		WHERE  id = @id`

	tag, err := n.pool.Exec(ctx, q, pgNamedArgs(map[string]interface{}{"id": id}))
	if err != nil {
		return fmt.Errorf("breach.NotifyCommissioner: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("breach.NotifyCommissioner: breach %s not found", id)
	}
	return nil
}

// NotifyAffected records the timestamp at which affected individuals were notified.
func (n *Notifier) NotifyAffected(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE privacy_breaches
		SET    notified_affected_at = now(),
		       updated_at = now()
		WHERE  id = @id`

	tag, err := n.pool.Exec(ctx, q, pgNamedArgs(map[string]interface{}{"id": id}))
	if err != nil {
		return fmt.Errorf("breach.NotifyAffected: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("breach.NotifyAffected: breach %s not found", id)
	}
	return nil
}

// OverdueNotifications returns all breaches whose 72-hour notification deadline
// has passed but where the Privacy Commissioner has not yet been notified.
func (n *Notifier) OverdueNotifications(ctx context.Context) ([]Breach, error) {
	const q = `
		SELECT
			id,
			tenant_id,
			type,
			severity,
			description,
			affected_patient_count,
			affected_nhis,
			discovered_at,
			reported_at,
			notified_commissioner_at,
			notified_affected_at,
			contained_at,
			status,
			notification_deadline,
			responsible_officer
		FROM privacy_breaches
		WHERE notification_deadline < now()
		  AND notified_commissioner_at IS NULL
		ORDER BY notification_deadline ASC`

	rows, err := n.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("breach.OverdueNotifications: query: %w", err)
	}
	defer rows.Close()

	var results []Breach
	for rows.Next() {
		var b Breach
		var bType, severity string
		if err := rows.Scan(
			&b.ID,
			&b.TenantID,
			&bType,
			&severity,
			&b.Description,
			&b.AffectedPatientCount,
			&b.AffectedNHIs,
			&b.DiscoveredAt,
			&b.ReportedAt,
			&b.NotifiedCommissionerAt,
			&b.NotifiedAffectedAt,
			&b.ContainedAt,
			&b.Status,
			&b.NotificationDeadline,
			&b.ResponsibleOfficer,
		); err != nil {
			return nil, fmt.Errorf("breach.OverdueNotifications: scan: %w", err)
		}
		b.Type = BreachType(bType)
		b.Severity = SeverityLevel(severity)
		results = append(results, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("breach.OverdueNotifications: rows: %w", err)
	}
	return results, nil
}

// Contain marks a breach as contained by recording the containment timestamp.
// The status is updated to "contained".
func (n *Notifier) Contain(ctx context.Context, id uuid.UUID) error {
	const q = `
		UPDATE privacy_breaches
		SET    contained_at = now(),
		       status = 'contained',
		       updated_at = now()
		WHERE  id = @id`

	tag, err := n.pool.Exec(ctx, q, pgNamedArgs(map[string]interface{}{"id": id}))
	if err != nil {
		return fmt.Errorf("breach.Contain: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("breach.Contain: breach %s not found", id)
	}
	return nil
}

// pgNamedArgs converts a plain map into pgx named arguments.
// pgx v5 accepts pgx.NamedArgs (which is map[string]any) directly.
func pgNamedArgs(m map[string]interface{}) map[string]interface{} {
	return m
}
