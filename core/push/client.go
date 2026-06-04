package push

import (
	"context"
	"fmt"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Config holds the VAPID signing credentials loaded from environment.
type Config struct {
	PrivateKey string // VAPID_PRIVATE_KEY — base64url-encoded EC private key
	PublicKey  string // VAPID_PUBLIC_KEY  — base64url-encoded EC public key
	Subject    string // VAPID_SUBJECT     — mailto: or https: contact URI
}

// Notification is the payload sent to a push subscription.
type Notification struct {
	Title            string `json:"title"`
	Body             string `json:"body"`
	Tag              string `json:"tag,omitempty"`
	URL              string `json:"url,omitempty"`
	RequireInteract  bool   `json:"requireInteraction,omitempty"`
}

// Client signs and sends Web Push messages via VAPID.
type Client struct {
	cfg Config
}

// NewClient creates a VAPID push client. cfg.PrivateKey and cfg.PublicKey must be set.
func NewClient(cfg Config) (*Client, error) {
	if cfg.PrivateKey == "" || cfg.PublicKey == "" {
		return nil, fmt.Errorf("push: VAPID_PRIVATE_KEY and VAPID_PUBLIC_KEY are required")
	}
	if cfg.Subject == "" {
		return nil, fmt.Errorf("push: VAPID_SUBJECT is required (e.g. mailto:ops@example.com)")
	}
	return &Client{cfg: cfg}, nil
}

// Send delivers a notification to a single push endpoint. Returns the HTTP status
// code from the push service so callers can detect 410 Gone (subscription expired).
func (c *Client) Send(ctx context.Context, sub *Subscription, n Notification) (int, error) {
	payload, err := marshalJSON(n)
	if err != nil {
		return 0, fmt.Errorf("push: marshal notification: %w", err)
	}

	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		HTTPClient:      &http.Client{Timeout: 10 * time.Second},
		TTL:             60 * 60 * 24, // 24h — push service may hold it if device is offline
		VAPIDPrivateKey: c.cfg.PrivateKey,
		VAPIDPublicKey:  c.cfg.PublicKey,
		Subscriber:      c.cfg.Subject,
		Urgency:         webpush.UrgencyNormal,
	})
	if err != nil {
		return 0, fmt.Errorf("push: send to %s: %w", sub.Endpoint, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
