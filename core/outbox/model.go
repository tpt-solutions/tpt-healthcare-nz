// Package outbox implements the transactional outbox pattern for reliable
// delivery of external integration messages (accounting sync, SMS, email, etc.).
// Messages are written atomically within the business transaction and processed
// asynchronously by a River worker with at-least-once delivery semantics.
package outbox

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of an outbox message.
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusDead       Status = "dead"
)

// Topic identifies the type of integration a message belongs to.
// Consumers subscribe to specific topics.
type Topic string

const (
	// Business integrations
	TopicAccountingInvoice Topic = "accounting.invoice.sync"
	TopicAccountingPayment Topic = "accounting.payment.record"
	TopicAccountingContact Topic = "accounting.contact.sync"
	TopicAccountingJournal Topic = "accounting.journal.post"
	TopicPayrollEmployee   Topic = "payroll.employee.sync"
	TopicPayrollTimesheets Topic = "payroll.timesheets.push"
	TopicPayrollLeave      Topic = "payroll.leave.submit"
	TopicSMSSend           Topic = "sms.send"
	TopicEmailSend         Topic = "email.send"
	TopicFaxSend           Topic = "fax.send"
	TopicVideoRoomCreate   Topic = "video.room.create"
	TopicStorageUpload     Topic = "storage.upload"
	TopicBackupTrigger     Topic = "backup.trigger"

	// NZ health system integrations — queued when live API calls fail so
	// that critical submissions are never silently dropped.
	TopicACCClaimLodge        Topic = "acc.claim.lodge"
	TopicACCPORequest         Topic = "acc.purchase-order.request"
	TopicACCPOConsume         Topic = "acc.purchase-order.consume"
	TopicWorkSafeClaimLodge   Topic = "worksafe.claim.lodge"
	TopicNHILookup            Topic = "nhi.patient.lookup"
	TopicNESEnrol             Topic = "nes.enrolment.submit"
	TopicPRIMHDReferral       Topic = "primhd.referral.open"
	TopicPRIMHDActivity       Topic = "primhd.activity.submit"
	TopicPRIMHDOutcome        Topic = "primhd.outcome.submit"
	TopicPRIMHDDischarge      Topic = "primhd.referral.close"
	TopicMedsafeADE           Topic = "medsafe.ade.report"
	TopicEpiSurvNotify        Topic = "episurv.notification.submit"
	TopicPharmacyDispatch     Topic = "pharmacy.prescription.dispatch"
	TopicERMSReferral         Topic = "erms.referral.submit"
)

// Message is a durable record of a pending external integration call.
// It is written within the same database transaction as the triggering
// business operation, guaranteeing that the message is never lost even
// if the application crashes before the external call completes.
type Message struct {
	ID           uuid.UUID `db:"id"`
	TenantID     uuid.UUID `db:"tenant_id"`
	Topic        Topic     `db:"topic"`
	Payload      []byte    `db:"payload"` // JSON-encoded, topic-specific
	Status       Status    `db:"status"`
	Attempts     int       `db:"attempts"`
	NextAttemptAt time.Time `db:"next_attempt_at"`
	DeadAt       *time.Time `db:"dead_at"`
	CreatedAt    time.Time `db:"created_at"`
	ProcessedAt  *time.Time `db:"processed_at"`
	LastError    string    `db:"last_error"`
}
