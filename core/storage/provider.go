// Package storage defines the ObjectStorageProvider interface for storing
// large binary objects: consent evidence, medical certificate PDFs, pathology
// attachments, radiology reports, and encrypted WAL backup files.
// All data is encrypted with AES-256-GCM via core/encryption before upload.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// UploadOptions controls how an object is stored.
type UploadOptions struct {
	ContentType string            `json:"content_type"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	// Encrypted signals that the caller has already applied AES-256-GCM
	// via core/encryption. The storage provider must NOT re-encrypt.
	Encrypted bool `json:"encrypted"`
}

// UploadResult is returned by Upload.
type UploadResult struct {
	Key       string    `json:"key"`
	ETag      string    `json:"etag,omitempty"`
	SizeBytes int64     `json:"size_bytes"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// HealthStatus reports connectivity to the storage provider.
type HealthStatus struct {
	OK       bool          `json:"ok"`
	Provider string        `json:"provider"`
	Latency  time.Duration `json:"latency_ms"`
	Err      string        `json:"error,omitempty"`
}

// Error is a storage provider application-level error.
type Error struct {
	Provider  string
	Code      string
	Message   string
	Retryable bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("storage(%s): %s — %s", e.Provider, e.Code, e.Message)
}

// Provider is the interface all object storage backends must implement.
// Keys are slash-delimited paths (e.g. "backups/nightly/2026-06-05.tar.gz.enc").
type Provider interface {
	Upload(ctx context.Context, key string, r io.Reader, opts UploadOptions) (*UploadResult, error)
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	// SignedURL returns a time-limited pre-signed URL for direct client access.
	SignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
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
		panic(fmt.Sprintf("storage: provider %q already registered", name))
	}
	registry[name] = factory
}

// New instantiates the provider named by viper key "storage.provider".
func New(ctx context.Context, v *viper.Viper) (Provider, error) {
	name := v.GetString("storage.provider")
	if name == "" {
		return nil, errors.New("storage: storage.provider config key is not set")
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
var ErrUnknownProvider = errors.New("storage: unknown provider")
