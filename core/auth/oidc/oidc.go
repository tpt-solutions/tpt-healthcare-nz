// Package oidc provides an OIDC client that integrates with tpt-identity and
// maps verified ID token claims to auth.Principal values.
package oidc

import (
	"context"
	"errors"
	"fmt"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/PhillipC05/tpt-healthcare/core/auth"
)

// Config holds the OIDC client configuration for a tpt-identity deployment.
type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Provider wraps an OIDC verifier and OAuth2 configuration, implementing
// auth.Provider by validating ID tokens issued by the configured issuer.
type Provider struct {
	oidcProvider *gooidc.Provider
	verifier     *gooidc.IDTokenVerifier
	oauth2Config oauth2.Config
}

// New constructs a Provider by discovering OIDC metadata from cfg.IssuerURL.
// The context is used for the discovery HTTP request.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	oidcProv, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: discover provider %s: %w", cfg.IssuerURL, err)
	}

	verifier := oidcProv.Verifier(&gooidc.Config{ClientID: cfg.ClientID})

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     oidcProv.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "profile", "email"},
	}

	return &Provider{
		oidcProvider: oidcProv,
		verifier:     verifier,
		oauth2Config: oauth2Cfg,
	}, nil
}

// oidcClaims mirrors the standard + tpt-identity custom claims in an ID token.
type oidcClaims struct {
	Sub            string   `json:"sub"`
	Email          string   `json:"email"`
	TenantID       string   `json:"tenant_id"`
	Roles          []string `json:"roles"`
	PractitionerID string   `json:"practitioner_id"`
}

// Validate verifies the raw ID token string and returns the corresponding
// Principal. It implements auth.Provider.
func (p *Provider) Validate(ctx context.Context, token string) (*auth.Principal, error) {
	idToken, err := p.verifier.Verify(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("oidc: verify id token: %w", err)
	}

	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc: extract claims: %w", err)
	}

	if claims.Sub == "" {
		return nil, errors.New("oidc: missing sub claim")
	}

	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil {
		// Tolerate missing/invalid tenant_id for federation scenarios.
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

// AuthCodeURL returns the URL to redirect the user to for the authorization
// code flow. state should be a random, unguessable value stored in the session.
func (p *Provider) AuthCodeURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange completes the authorization code flow: it exchanges code for tokens,
// verifies the embedded ID token, and returns the resolved Principal plus the
// raw ID token string for use as a Bearer credential downstream.
func (p *Provider) Exchange(ctx context.Context, code string) (*auth.Principal, string, error) {
	oauth2Token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, "", fmt.Errorf("oidc: exchange code: %w", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, "", errors.New("oidc: no id_token in token response")
	}

	principal, err := p.Validate(ctx, rawIDToken)
	if err != nil {
		return nil, "", err
	}

	return principal, rawIDToken, nil
}
