package mock

import (
	"context"
	"io"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/storage"
)

// StorageProvider is a no-op fake implementing storage.Provider for unit tests.
type StorageProvider struct {
	UploadFunc   func(ctx context.Context, key string, r io.Reader, opts storage.UploadOptions) (*storage.UploadResult, error)
	DownloadFunc func(ctx context.Context, key string) (io.ReadCloser, error)
	DeleteFunc   func(ctx context.Context, key string) error
	SignedURLFunc func(ctx context.Context, key string, expiry time.Duration) (string, error)
	HealthCheckFunc func(ctx context.Context) (*storage.HealthStatus, error)
}

func (s *StorageProvider) Upload(ctx context.Context, key string, r io.Reader, opts storage.UploadOptions) (*storage.UploadResult, error) {
	if s.UploadFunc != nil {
		return s.UploadFunc(ctx, key, r, opts)
	}
	return &storage.UploadResult{Key: key, SizeBytes: 0, UploadedAt: time.Now()}, nil
}

func (s *StorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.DownloadFunc != nil {
		return s.DownloadFunc(ctx, key)
	}
	return io.NopCloser(nil), nil
}

func (s *StorageProvider) Delete(ctx context.Context, key string) error {
	if s.DeleteFunc != nil {
		return s.DeleteFunc(ctx, key)
	}
	return nil
}

func (s *StorageProvider) SignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if s.SignedURLFunc != nil {
		return s.SignedURLFunc(ctx, key, expiry)
	}
	return "https://example.com/signed/" + key, nil
}

func (s *StorageProvider) HealthCheck(ctx context.Context) (*storage.HealthStatus, error) {
	if s.HealthCheckFunc != nil {
		return s.HealthCheckFunc(ctx)
	}
	return &storage.HealthStatus{OK: true, Provider: "mock"}, nil
}
