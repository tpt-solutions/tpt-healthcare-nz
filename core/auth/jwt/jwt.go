// Package jwt provides Ed25519 JWT issuance and validation for tpt-healthcare,
// along with TOTP secret generation and verification via pquerna/otp.
package jwt

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/auth"

	// TOTP support requires github.com/pquerna/otp/totp — add to go.mod:
	//   go get github.com/pquerna/otp/totp
	totplib "github.com/pquerna/otp/totp"
)

// Claims extends jwt.RegisteredClaims with tpt-healthcare-specific fields.
type Claims struct {
	gojwt.RegisteredClaims

	TenantID       string   `json:"tenant_id,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	PractitionerID string   `json:"practitioner_id,omitempty"`
}

// Provider issues and validates Ed25519-signed JWTs.
type Provider struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// New creates a Provider from an Ed25519 private key. The public key is derived
// automatically.
func New(privateKey ed25519.PrivateKey) *Provider {
	return &Provider{
		privateKey: privateKey,
		publicKey:  privateKey.Public().(ed25519.PublicKey),
	}
}

// NewFromFiles reads Ed25519 private and public keys from PEM-encoded files and
// returns a Provider. Both files must contain a single PEM block each.
func NewFromFiles(privateKeyPath, publicKeyPath string) (*Provider, error) {
	privPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("jwt: read private key file: %w", err)
	}

	block, _ := pem.Decode(privPEM)
	if block == nil {
		return nil, errors.New("jwt: failed to decode private key PEM block")
	}

	privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse private key: %w", err)
	}

	edPriv, ok := privKey.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("jwt: private key is not Ed25519")
	}

	pubPEM, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("jwt: read public key file: %w", err)
	}

	pubBlock, _ := pem.Decode(pubPEM)
	if pubBlock == nil {
		return nil, errors.New("jwt: failed to decode public key PEM block")
	}

	pubKeyRaw, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse public key: %w", err)
	}

	edPub, ok := pubKeyRaw.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("jwt: public key is not Ed25519")
	}

	return &Provider{
		privateKey: edPriv,
		publicKey:  edPub,
	}, nil
}

// Issue creates a signed JWT for the given Principal with the specified TTL.
func (p *Provider) Issue(_ context.Context, principal *auth.Principal, ttl time.Duration) (string, error) {
	now := time.Now()

	claims := Claims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   principal.ID,
			IssuedAt:  gojwt.NewNumericDate(now),
			ExpiresAt: gojwt.NewNumericDate(now.Add(ttl)),
		},
		TenantID:       principal.TenantID.String(),
		Roles:          principal.Roles,
		PractitionerID: principal.PractitionerID,
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodEdDSA, claims)

	signed, err := token.SignedString(p.privateKey)
	if err != nil {
		return "", fmt.Errorf("jwt: sign token: %w", err)
	}

	return signed, nil
}

// Validate parses and verifies a JWT, returning the encoded Principal on success.
// It implements auth.Provider.
func (p *Provider) Validate(_ context.Context, tokenStr string) (*auth.Principal, error) {
	token, err := gojwt.ParseWithClaims(tokenStr, &Claims{}, func(t *gojwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method: %v", t.Header["alg"])
		}
		return p.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt: parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("jwt: invalid token claims")
	}

	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		return nil, fmt.Errorf("jwt: parse tenant_id: %w", err)
	}

	principal := &auth.Principal{
		ID:             claims.Subject,
		TenantID:       tenantID,
		Roles:          claims.Roles,
		PractitionerID: claims.PractitionerID,
		Practitioner:   claims.PractitionerID != "",
	}

	return principal, nil
}

// GenerateTOTPSecret generates a new TOTP secret key for a user identified by
// accountName, scoped to the issuer label "tpt-healthcare". The returned string
// is the base32-encoded secret that should be stored (encrypted) for the user.
func GenerateTOTPSecret(accountName string) (string, error) {
	key, err := totplib.Generate(totplib.GenerateOpts{
		Issuer:      "tpt-healthcare",
		AccountName: accountName,
	})
	if err != nil {
		return "", fmt.Errorf("jwt: generate TOTP secret: %w", err)
	}

	return key.Secret(), nil
}

// ValidateTOTP verifies a 6-digit TOTP code against the provided base32-encoded
// secret. Returns true only when the code is valid for the current time window.
func ValidateTOTP(secret, code string) bool {
	return totplib.Validate(code, secret)
}
