// Package auth0 provides Auth0 OIDC token validation, mapping verified JWT
// claims to auth.Principal values for use within tpt-healthcare services.
package auth0

import (
	"context"
	"errors"
	"fmt"
	"os"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
)

// Config holds the parameters required to validate Auth0-issued tokens.
type Config struct {
	// Domain is the Auth0 tenant domain, e.g. "example.au.auth0.com".
	Domain string
	// Audience is the API identifier registered in Auth0.
	Audience string
	// ClientID is the Auth0 application client ID.
	ClientID string
}

// Provider validates Auth0 JWTs using OIDC discovery and maps their claims to
// auth.Principal values.
type Provider struct {
	cfg      Config
	verifier *gooidc.IDTokenVerifier
}

// New constructs a Provider by discovering OIDC metadata from the Auth0 tenant
// identified by cfg.Domain. The ctx is used for the discovery HTTP request.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	if cfg.Domain == "" {
		return nil, errors.New("auth0: Domain must not be empty")
	}

	issuerURL := "https://" + cfg.Domain + "/"

	oidcProv, err := gooidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth0: discover provider for domain %s: %w", cfg.Domain, err)
	}

	// Auth0 tokens carry the audience in the `aud` claim; the verifier checks it.
	verifier := oidcProv.Verifier(&gooidc.Config{
		ClientID:          cfg.Audience,
		SkipClientIDCheck: cfg.Audience == "",
	})

	return &Provider{
		cfg:      cfg,
		verifier: verifier,
	}, nil
}

// NewProvider is a compatibility constructor that accepts domain and audience strings.
// Preserved for module compatibility; prefer New with a full Config.
func NewProvider(domain, audience string) (*Provider, error) {
	return New(context.Background(), Config{Domain: domain, Audience: audience})
}

// NewFromEnv reads AUTH0_DOMAIN, AUTH0_AUDIENCE, and AUTH0_CLIENT_ID from the
// environment and delegates to New.
func NewFromEnv(ctx context.Context) (*Provider, error) {
	cfg := Config{
		Domain:   os.Getenv("AUTH0_DOMAIN"),
		Audience: os.Getenv("AUTH0_AUDIENCE"),
		ClientID: os.Getenv("AUTH0_CLIENT_ID"),
	}

	if cfg.Domain == "" {
		return nil, errors.New("auth0: AUTH0_DOMAIN environment variable is not set")
	}

	return New(ctx, cfg)
}

// auth0Claims mirrors the standard + Auth0-specific claims in a JWT.
// Auth0 exposes custom claims under a namespace; tpt-healthcare uses the
// https://tpt-healthcare.co.nz/ namespace by convention.
type auth0Claims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`

	// Custom claims injected via Auth0 Actions / Rules using the
	// https://tpt-healthcare.co.nz/ namespace.
	TenantID       string   `json:"https://tpt-healthcare.co.nz/tenant_id"`
	Roles          []string `json:"https://tpt-healthcare.co.nz/roles"`
	PractitionerID string   `json:"https://tpt-healthcare.co.nz/practitioner_id"`
}

// Validate verifies an Auth0-issued JWT and returns the corresponding Principal.
// It implements auth.Provider.
func (p *Provider) Validate(ctx context.Context, token string) (*auth.Principal, error) {
	idToken, err := p.verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("auth0: verify token: %w", err)
	}

	var claims auth0Claims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("auth0: extract claims: %w", err)
	}

	if claims.Sub == "" {
		return nil, errors.New("auth0: missing sub claim")
	}

	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		// Tolerate missing/invalid tenant_id gracefully.
		tenantID = uuid.Nil
	}

	return &auth.Principal{
		ID:             claims.Sub,
		TenantID:       tenantID,
		Email:          claims.Email,
		Roles:          claims.Roles,
		Practitioner:   claims.PractitionerID != "",
		PractitionerID: claims.PractitionerID,
	}, nil
}
