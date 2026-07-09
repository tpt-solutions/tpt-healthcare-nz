package jwt

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
)

func newTestProvider(t *testing.T) *Provider {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return New(priv)
}

func newTestPrincipal() *auth.Principal {
	return &auth.Principal{
		ID:             "user-123",
		TenantID:       uuid.New(),
		Roles:          []string{"clinician", "nurse"},
		PractitionerID: "CPN123456",
		Practitioner:   true,
	}
}

func TestJWT_IssueValidate_RoundTrip(t *testing.T) {
	p := newTestProvider(t)
	principal := newTestPrincipal()

	token, err := p.Issue(context.Background(), principal, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	got, err := p.Validate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, principal.ID, got.ID)
	assert.Equal(t, principal.TenantID, got.TenantID)
	assert.Equal(t, principal.Roles, got.Roles)
	assert.Equal(t, principal.PractitionerID, got.PractitionerID)
	assert.True(t, got.Practitioner)
}

func TestJWT_Validate_Expired(t *testing.T) {
	p := newTestProvider(t)
	principal := newTestPrincipal()

	token, err := p.Issue(context.Background(), principal, -time.Hour)
	require.NoError(t, err)

	_, err = p.Validate(context.Background(), token)
	assert.Error(t, err)
}

func TestJWT_Validate_WrongKey(t *testing.T) {
	_, priv1, _ := ed25519.GenerateKey(rand.Reader)
	_, priv2, _ := ed25519.GenerateKey(rand.Reader)

	p1 := New(priv1)
	p2 := New(priv2)

	principal := newTestPrincipal()
	token, err := p1.Issue(context.Background(), principal, time.Hour)
	require.NoError(t, err)

	_, err = p2.Validate(context.Background(), token)
	assert.Error(t, err)
}

func TestJWT_Validate_Malformed(t *testing.T) {
	p := newTestProvider(t)
	_, err := p.Validate(context.Background(), "not-a-jwt")
	assert.Error(t, err)
}

func TestJWT_NewFromFiles(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	pub := priv.Public().(ed25519.PublicKey)

	dir := t.TempDir()
	privFile := filepath.Join(dir, "priv.pem")
	pubFile := filepath.Join(dir, "pub.pem")

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(privFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}), 0600))

	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(pubFile, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), 0644))

	t.Run("valid files", func(t *testing.T) {
		p, err := NewFromFiles(privFile, pubFile)
		require.NoError(t, err)
		assert.NotNil(t, p)

		token, err := p.Issue(context.Background(), newTestPrincipal(), time.Hour)
		require.NoError(t, err)

		got, err := p.Validate(context.Background(), token)
		require.NoError(t, err)
		assert.Equal(t, "user-123", got.ID)
	})

	t.Run("missing private key", func(t *testing.T) {
		_, err := NewFromFiles(filepath.Join(dir, "missing.pem"), pubFile)
		assert.Error(t, err)
	})

	t.Run("missing public key", func(t *testing.T) {
		_, err := NewFromFiles(privFile, filepath.Join(dir, "missing.pem"))
		assert.Error(t, err)
	})

	t.Run("corrupt PEM", func(t *testing.T) {
		badFile := filepath.Join(dir, "bad.pem")
		os.WriteFile(badFile, []byte("not-pem-data"), 0644)
		_, err := NewFromFiles(badFile, pubFile)
		assert.Error(t, err)
	})

	t.Run("wrong key type", func(t *testing.T) {
		badPriv := filepath.Join(dir, "rsa-priv.pem")
		os.WriteFile(badPriv, pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: []byte("not-valid-key"),
		}), 0600)
		_, err := NewFromFiles(badPriv, pubFile)
		assert.Error(t, err)
	})
}

func TestGenerateTOTPSecret(t *testing.T) {
	secret, err := GenerateTOTPSecret("user@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
}

func TestValidateTOTP(t *testing.T) {
	secret, err := GenerateTOTPSecret("user@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)

	t.Run("invalid code", func(t *testing.T) {
		assert.False(t, ValidateTOTP(secret, "000000"))
	})

	t.Run("empty code", func(t *testing.T) {
		assert.False(t, ValidateTOTP(secret, ""))
	})
}
