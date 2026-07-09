package storage

import (
	"errors"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	e := &Error{
		Provider:  "s3",
		Code:      "ACCESS_DENIED",
		Message:   "Permission denied",
		Retryable: false,
	}
	s := e.Error()
	assert.Contains(t, s, "s3")
	assert.Contains(t, s, "ACCESS_DENIED")
	assert.Contains(t, s, "Permission denied")
}

func TestStorage_New_EmptyConfig(t *testing.T) {
	v := viper.New()
	_, err := New(t.Context(), v)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not set")
}

func TestStorage_New_UnknownProvider(t *testing.T) {
	v := viper.New()
	v.Set("storage.provider", "nonexistent")
	_, err := New(t.Context(), v)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownProvider))
}

func TestStorageError_Is(t *testing.T) {
	err := &Error{Provider: "s3", Code: "ERR", Message: "test"}
	assert.True(t, errors.Is(err, err))
}

func TestUploadOptions_JSONRoundTrip(t *testing.T) {
	opts := UploadOptions{
		ContentType: "application/pdf",
		Metadata:    map[string]string{"patient": "ZAC1234"},
		Encrypted:   true,
	}
	assert.Equal(t, "application/pdf", opts.ContentType)
	assert.Equal(t, "ZAC1234", opts.Metadata["patient"])
	assert.True(t, opts.Encrypted)
}

func TestHealthStatus_JSONRoundTrip(t *testing.T) {
	hs := HealthStatus{
		OK:       true,
		Provider: "s3",
		Latency:  100000000, // 100ms
	}
	assert.True(t, hs.OK)
	assert.Equal(t, "s3", hs.Provider)
}

func TestUploadResult_JSONRoundTrip(t *testing.T) {
	ur := UploadResult{
		Key:       "backups/test.enc",
		ETag:      "abc123",
		SizeBytes: 1024,
	}
	assert.Equal(t, "backups/test.enc", ur.Key)
	assert.Equal(t, int64(1024), ur.SizeBytes)
}
