// Package backup orchestrates WAL archiving, point-in-time recovery snapshots,
// nightly restore verification, and retention policy enforcement for the
// tpt-healthcare PostgreSQL database.
//
// Scheduling:
//   - pg_cron triggers a nightly base backup notification via a DB event.
//   - River workers handle snapshot upload, restore verification, and pruning.
//   - Retention enforcement runs nightly via pg_cron stored procedures.
package backup

import "time"

// Strategy describes how expired data is handled when its retention period ends.
type Strategy string

const (
	// StrategyArchive moves rows to a cold-storage partition rather than deleting.
	StrategyArchive Strategy = "archive"
	// StrategyDelete hard-deletes rows after the retention period.
	StrategyDelete Strategy = "delete"
)

// RetentionPolicy defines how long data in a table or schema must be kept.
type RetentionPolicy struct {
	// TableName is the fully qualified table (e.g. "public.audit_events").
	TableName string `db:"table_name" json:"table_name"`

	// RetentionYears is the minimum number of years data must be retained.
	RetentionYears int `db:"retention_years" json:"retention_years"`

	// Strategy controls what happens when data exceeds the retention period.
	Strategy Strategy `db:"strategy" json:"strategy"`

	// TimestampColumn is the column used to determine record age.
	// Defaults to "created_at".
	TimestampColumn string `db:"timestamp_column" json:"timestamp_column"`
}

// DefaultPolicies returns the baseline retention policies required by
// NZ health information law and HIPC 2020.
func DefaultPolicies() []RetentionPolicy {
	return []RetentionPolicy{
		{
			TableName:       "public.audit_events",
			RetentionYears:  10,
			Strategy:        StrategyArchive, // append-only; move to cold partition
			TimestampColumn: "created_at",
		},
		{
			TableName:       "public.fhir_resources",
			RetentionYears:  10,
			Strategy:        StrategyArchive,
			TimestampColumn: "last_updated",
		},
		{
			TableName:       "public.outbox_messages",
			RetentionYears:  1,
			Strategy:        StrategyDelete,
			TimestampColumn: "created_at",
		},
		{
			TableName:       "public.patient_invoices",
			RetentionYears:  7, // NZ GST/IRD requirement
			Strategy:        StrategyArchive,
			TimestampColumn: "created_at",
		},
		{
			TableName:       "public.backup_runs",
			RetentionYears:  10,
			Strategy:        StrategyDelete,
			TimestampColumn: "started_at",
		},
	}
}

// Run is a record of a backup execution.
type Run struct {
	ID          string     `db:"id"           json:"id"`
	StartedAt   time.Time  `db:"started_at"   json:"started_at"`
	CompletedAt *time.Time `db:"completed_at" json:"completed_at,omitempty"`
	Status      RunStatus  `db:"status"       json:"status"`
	SizeBytes   int64      `db:"size_bytes"   json:"size_bytes,omitempty"`
	StorageKey  string     `db:"storage_key"  json:"storage_key,omitempty"`
	ErrorText   string     `db:"error_text"   json:"error_text,omitempty"`
}

// RunStatus is the lifecycle state of a backup run.
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSuccess   RunStatus = "success"
	RunStatusFailed    RunStatus = "failed"
	RunStatusVerified  RunStatus = "verified"
)
