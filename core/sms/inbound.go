package sms

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InboundMessage represents an SMS received from a patient.
type InboundMessage struct {
	// From is the sender's phone number in E.164 format.
	From string
	// To is the receiving virtual number (the practice's SMS number).
	To string
	// Body is the raw message text.
	Body string
	// ExternalID is the provider's message ID, for deduplication.
	ExternalID string
	// ReceivedAt is when the provider received the message.
	ReceivedAt time.Time
}

// ReplyAction is the outcome of parsing an inbound message.
type ReplyAction string

const (
	// ReplyConfirm means the patient confirmed their appointment (digit "1").
	ReplyConfirm ReplyAction = "confirm"
	// ReplyCancel means the patient cancelled their appointment (digit "2").
	ReplyCancel ReplyAction = "cancel"
	// ReplyUnknown means the body was not recognised as a structured reply.
	ReplyUnknown ReplyAction = "unknown"
)

// ParseReply interprets the body of a reply SMS and returns the corresponding action.
// "1" or "yes" (case-insensitive) → confirm; "2" or "no" or "cancel" → cancel.
func ParseReply(body string) ReplyAction {
	b := strings.TrimSpace(strings.ToLower(body))
	switch {
	case b == "1" || b == "yes" || b == "y" || b == "confirm":
		return ReplyConfirm
	case b == "2" || b == "no" || b == "n" || b == "cancel":
		return ReplyCancel
	default:
		return ReplyUnknown
	}
}

// InboundHandler processes incoming SMS webhooks from providers and resolves
// pending appointment confirmations or cancellations.
type InboundHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	// OnUnknown is called when the reply body is not recognised, allowing
	// the caller to route it to the messaging inbox or staff attention queue.
	OnUnknown func(ctx context.Context, msg InboundMessage)
}

// NewInboundHandler creates an InboundHandler.
// onUnknown may be nil; unrecognised replies are then logged and discarded.
func NewInboundHandler(pool *pgxpool.Pool, logger *slog.Logger, onUnknown func(context.Context, InboundMessage)) *InboundHandler {
	return &InboundHandler{pool: pool, logger: logger, OnUnknown: onUnknown}
}

// Process handles a single inbound SMS. It looks up the most recent pending
// appointment reminder sent to msg.From, then confirms or cancels it according
// to the reply body.
func (h *InboundHandler) Process(ctx context.Context, msg InboundMessage) error {
	action := ParseReply(msg.Body)

	h.logger.InfoContext(ctx, "inbound SMS received",
		slog.String("from", msg.From),
		slog.String("action", string(action)),
		slog.String("external_id", msg.ExternalID),
	)

	switch action {
	case ReplyConfirm:
		return h.confirmAppointment(ctx, msg.From)
	case ReplyCancel:
		return h.cancelAppointment(ctx, msg.From)
	case ReplyUnknown:
		if h.OnUnknown != nil {
			h.OnUnknown(ctx, msg)
		} else {
			h.logger.InfoContext(ctx, "unrecognised SMS reply; no handler configured",
				slog.String("from", msg.From),
				slog.String("body", msg.Body),
			)
		}
	}
	return nil
}

func (h *InboundHandler) confirmAppointment(ctx context.Context, from string) error {
	tag, err := h.pool.Exec(ctx,
		`UPDATE appointments
		 SET status = 'confirmed', sms_confirmed_at = NOW()
		 WHERE id = (
			SELECT a.id FROM appointments a
			JOIN patients p ON p.id = a.patient_id
			WHERE p.mobile_e164 = $1
			  AND a.status IN ('booked','pending')
			  AND a.start_time > NOW()
			ORDER BY a.start_time ASC
			LIMIT 1
		 )`,
		from,
	)
	if err != nil {
		return fmt.Errorf("sms inbound: confirming appointment for %s: %w", from, err)
	}
	if tag.RowsAffected() == 0 {
		h.logger.InfoContext(ctx, "SMS confirm: no pending appointment found", slog.String("from", from))
	}
	return nil
}

func (h *InboundHandler) cancelAppointment(ctx context.Context, from string) error {
	tag, err := h.pool.Exec(ctx,
		`UPDATE appointments
		 SET status = 'cancelled', cancelled_at = NOW(), cancellation_reason = 'patient-sms'
		 WHERE id = (
			SELECT a.id FROM appointments a
			JOIN patients p ON p.id = a.patient_id
			WHERE p.mobile_e164 = $1
			  AND a.status IN ('booked','confirmed','pending')
			  AND a.start_time > NOW()
			ORDER BY a.start_time ASC
			LIMIT 1
		 )`,
		from,
	)
	if err != nil {
		return fmt.Errorf("sms inbound: cancelling appointment for %s: %w", from, err)
	}
	if tag.RowsAffected() == 0 {
		h.logger.InfoContext(ctx, "SMS cancel: no pending appointment found", slog.String("from", from))
	}
	return nil
}

// WebhookParser is a lightweight adapter that translates provider-specific
// webhook payloads into InboundMessages. Each provider implements this
// interface so the InboundHandler stays provider-agnostic.
type WebhookParser interface {
	// ParseWebhook reads r and returns the inbound message, or an error if the
	// payload is malformed or the request is not authentic.
	ParseWebhook(r *http.Request) (*InboundMessage, error)
}

// InboundWebhookHandler returns an http.HandlerFunc that accepts a POST
// webhook from an SMS provider, parses it with parser, and delegates to
// handler.Process.
func InboundWebhookHandler(parser WebhookParser, handler *InboundHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		msg, err := parser.ParseWebhook(r)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := handler.Process(r.Context(), *msg); err != nil {
			handler.logger.ErrorContext(r.Context(), "inbound SMS processing error", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// MessageBirdWebhookParser implements WebhookParser for MessageBird inbound SMS.
type MessageBirdWebhookParser struct{}

func (MessageBirdWebhookParser) ParseWebhook(r *http.Request) (*InboundMessage, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("messagebird: parsing form: %w", err)
	}
	msg := &InboundMessage{
		From:       r.FormValue("originator"),
		To:         r.FormValue("recipient"),
		Body:       r.FormValue("body"),
		ExternalID: r.FormValue("id"),
		ReceivedAt: time.Now().UTC(),
	}
	if msg.From == "" || msg.Body == "" {
		return nil, fmt.Errorf("messagebird: missing originator or body")
	}
	return msg, nil
}

// VonageWebhookParser implements WebhookParser for Vonage (formerly Nexmo) inbound SMS.
type VonageWebhookParser struct{}

func (VonageWebhookParser) ParseWebhook(r *http.Request) (*InboundMessage, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("vonage: parsing form: %w", err)
	}
	msg := &InboundMessage{
		From:       r.FormValue("msisdn"),
		To:         r.FormValue("to"),
		Body:       r.FormValue("text"),
		ExternalID: r.FormValue("messageId"),
		ReceivedAt: time.Now().UTC(),
	}
	if msg.From == "" || msg.Body == "" {
		return nil, fmt.Errorf("vonage: missing msisdn or text")
	}
	return msg, nil
}

// TwilioWebhookParser implements WebhookParser for Twilio inbound SMS.
// Twilio signatures should be verified using the auth token before calling
// ParseWebhook; this parser assumes the middleware layer has already done so.
type TwilioWebhookParser struct{}

func (TwilioWebhookParser) ParseWebhook(r *http.Request) (*InboundMessage, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("twilio: parsing form: %w", err)
	}
	msg := &InboundMessage{
		From:       r.FormValue("From"),
		To:         r.FormValue("To"),
		Body:       r.FormValue("Body"),
		ExternalID: r.FormValue("MessageSid"),
		ReceivedAt: time.Now().UTC(),
	}
	if msg.From == "" || msg.Body == "" {
		return nil, fmt.Errorf("twilio: missing From or Body")
	}
	return msg, nil
}

