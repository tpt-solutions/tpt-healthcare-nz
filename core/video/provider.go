// Package video defines the VideoProvider interface for telehealth video
// consultations. Room management, JWT-authenticated join URLs, and optional
// recording are the core operations.
package video

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// ParticipantRole classifies a meeting participant.
type ParticipantRole string

const (
	RoleHost  ParticipantRole = "host"
	RoleGuest ParticipantRole = "guest"
)

// RoomOptions controls how a video room is created.
type RoomOptions struct {
	AppointmentID string        `json:"appointment_id"`
	HostHPI       string        `json:"host_hpi,omitempty"`
	PatientNHI    string        `json:"patient_nhi,omitempty"`
	MaxDuration   time.Duration `json:"max_duration"`
	Recording     bool          `json:"recording"`
}

// Room is a created video room.
type Room struct {
	ExternalID string    `json:"external_id"`
	HostURL    string    `json:"host_url"`
	PatientURL string    `json:"patient_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// HealthStatus reports connectivity to the video provider.
type HealthStatus struct {
	OK       bool          `json:"ok"`
	Provider string        `json:"provider"`
	Latency  time.Duration `json:"latency_ms"`
	Err      string        `json:"error,omitempty"`
}

// Error is a video provider application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("video(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all video backends must implement.
type Provider interface {
	CreateRoom(ctx context.Context, opts RoomOptions) (*Room, error)
	GetJoinURL(ctx context.Context, roomID, participantName string, role ParticipantRole) (string, error)
	EndRoom(ctx context.Context, roomID string) error
	GetRecordingURL(ctx context.Context, roomID string) (string, error)
	HealthCheck(ctx context.Context) (*HealthStatus, error)
}

// Factory is a constructor registered by each backend.
type Factory func(ctx context.Context, v *viper.Viper) (Provider, error)

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Factory)
)

// Register associates name with factory.
func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("video: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "video.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("video.provider")
	if name == "" {
		return nil, errors.New("video: video.provider config key is not set")
	}
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, name)
	}
	return factory(ctx, v)
}

// ErrUnknownProvider is returned when no registered backend matches.
var ErrUnknownProvider = errors.New("video: unknown provider")
