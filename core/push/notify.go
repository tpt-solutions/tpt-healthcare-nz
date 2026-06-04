package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Notifier sends Web Push notifications to all registered devices for a patient.
type Notifier struct {
	client *Client
	store  *Store
	logger *slog.Logger
}

// NewNotifier creates a Notifier combining the VAPID client and subscription store.
func NewNotifier(client *Client, store *Store, logger *slog.Logger) *Notifier {
	return &Notifier{client: client, store: store, logger: logger}
}

// Send delivers a notification to every push subscription registered for the patient.
// Subscriptions that respond with HTTP 410 (Gone) are automatically removed.
func (n *Notifier) Send(ctx context.Context, patientID uuid.UUID, notif Notification) error {
	subs, err := n.store.ListByPatient(ctx, patientID)
	if err != nil {
		return fmt.Errorf("notify: list subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}

	var lastErr error
	for _, sub := range subs {
		status, err := n.client.Send(ctx, &sub, notif)
		if err != nil {
			n.logger.WarnContext(ctx, "push delivery failed",
				slog.String("endpoint", sub.Endpoint),
				slog.String("error", err.Error()),
			)
			lastErr = err
			continue
		}
		if status == http.StatusGone {
			// Subscription has expired — clean up so we don't keep trying
			n.logger.InfoContext(ctx, "push subscription expired, removing",
				slog.String("endpoint", sub.Endpoint),
			)
			if delErr := n.store.Delete(ctx, patientID, sub.Endpoint); delErr != nil {
				n.logger.WarnContext(ctx, "failed to remove expired subscription", slog.String("error", delErr.Error()))
			}
		}
	}
	return lastErr
}

// marshalJSON is a package-level helper used by both client.go and notify.go.
func marshalJSON(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// pgNamedArgs wraps a map into pgx named-argument syntax.
func pgNamedArgs(m map[string]any) pgx.NamedArgs {
	return pgx.NamedArgs(m)
}
