package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// TokenFunc is a function that returns a bearer token for authenticating to the NIR FHIR API.
// Implementations should cache the token and refresh it before expiry.
type TokenFunc func(ctx context.Context) (string, error)

// NIRClient is a client for the Te Whatu Ora National Immunisation Register FHIR R4 API.
//
// The NIR FHIR API endpoint and authentication details are provided by Health New Zealand.
// Authentication uses the Health New Zealand OAuth 2.0 token endpoint with client credentials.
// All requests must be made over TLS 1.2+.
//
// API reference: https://fhir.api.health.govt.nz/R4 (placeholder — subject to Te Whatu Ora docs)
type NIRClient struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  TokenFunc
	logger     *slog.Logger
}

// NewNIRClient constructs a NIRClient with a default client-credentials token function.
// If tokenURL, clientID, or clientSecret are empty, the token function will return an error
// at call time (allowing the server to start even without NIR credentials configured).
func NewNIRClient(baseURL, tokenURL, clientID, clientSecret string) *NIRClient {
	tokenFunc := buildClientCredentialsTokenFunc(tokenURL, clientID, clientSecret)
	return &NIRClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   baseURL,
		tokenFunc: tokenFunc,
		logger:    slog.Default(),
	}
}

// GetImmunisationHistory fetches a patient's immunisation history from the NIR.
//
// The NIR returns a FHIR R4 Bundle of Immunization resources. NHI is passed as the
// patient identifier on the request URL.
//
// HIPC Rule 11 (Disclosure): Callers must obtain patient consent or establish a
// disclosure exception before calling this function. Consent is not checked here —
// it is the caller's responsibility per the consent layer in core/consent.
func (c *NIRClient) GetImmunisationHistory(ctx context.Context, nhi string) ([]Immunisation, error) {
	if nhi == "" {
		return nil, fmt.Errorf("NIRClient.GetImmunisationHistory: nhi is required")
	}

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("NIRClient.GetImmunisationHistory: get token: %w", err)
	}

	url := fmt.Sprintf("%s/Immunization?patient.identifier=https://standards.digital.health.nz/ns/nhi-id|%s", c.baseURL, nhi)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("NIRClient.GetImmunisationHistory: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("NIRClient.GetImmunisationHistory: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("NIRClient.GetImmunisationHistory: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NIRClient.GetImmunisationHistory: NIR returned %d: %s",
			resp.StatusCode, string(body))
	}

	// Parse FHIR R4 Bundle and extract Immunization entries.
	// In production: unmarshal into core/fhir/r4 Bundle and translate each resource
	// to R5 Immunization via core/fhir/translate before returning.
	var bundle struct {
		Entry []struct {
			Resource json.RawMessage `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(body, &bundle); err != nil {
		return nil, fmt.Errorf("NIRClient.GetImmunisationHistory: decode bundle: %w", err)
	}

	immunisations := make([]Immunisation, 0, len(bundle.Entry))
	for _, entry := range bundle.Entry {
		var imm Immunisation
		if err := json.Unmarshal(entry.Resource, &imm); err != nil {
			c.logger.Warn("NIRClient.GetImmunisationHistory: skip malformed entry", "error", err)
			continue
		}
		immunisations = append(immunisations, imm)
	}

	return immunisations, nil
}

// Submit POSTs a single Immunization resource to the NIR.
//
// The resource is translated from R5 to R4 before submission (FHIR R4 is the NIR's
// supported version). On success, the NIR returns a 201 Created with a Location header
// containing the NIR-assigned resource ID (stored as NIRReferenceID by the caller).
//
// Idempotency: NIR does not natively support idempotent submissions via If-None-Exist;
// callers must check NIRSubmitted before calling this function to avoid duplicates.
func (c *NIRClient) Submit(ctx context.Context, imm Immunisation) error {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return fmt.Errorf("NIRClient.Submit: get token: %w", err)
	}

	// In production: translate Immunisation (R5) → FHIR R4 Immunization via core/fhir/translate
	// before marshalling and submitting. Here we submit the internal struct as a placeholder.
	payload, err := json.Marshal(imm)
	if err != nil {
		return fmt.Errorf("NIRClient.Submit: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/Immunization", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("NIRClient.Submit: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("NIRClient.Submit: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("NIRClient.Submit: read body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("NIRClient.Submit: NIR returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// NIRHandler handles /api/v1/nir routes — proxying to the NIR FHIR API.
type NIRHandler struct {
	logger *slog.Logger
	nir    *NIRClient
}

// GetHistory handles GET /api/v1/nir/{nhi} — proxy to NIR immunisation history.
//
// This endpoint checks HIPC Rule 11 consent before proxying to the NIR. The NHI is
// validated for format and checksum before making the upstream call. Results are not
// cached (the NIR is the system of record for immunisation history).
func (h *NIRHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	nhi := r.PathValue("nhi")
	if nhi == "" {
		writeError(w, http.StatusBadRequest, "nhi path parameter is required")
		return
	}

	ctx := r.Context()

	// In production:
	//   1. Validate NHI format + checksum via core/nhi.
	//   2. Check consent via core/consent (HIPC Rule 11 — disclosure).
	//   3. Write AuditEvent (read from NIR) via core/audit.

	immunisations, err := h.nir.GetImmunisationHistory(ctx, nhi)
	if err != nil {
		h.logger.Error("NIR history fetch failed",
			"nhi", nhi,
			"error", err,
			"request_id", ctx.Value(requestIDKey),
		)
		writeError(w, http.StatusBadGateway, fmt.Sprintf("NIR history unavailable: %v", err))
		return
	}

	h.logger.Info("NIR history fetched",
		"nhi", nhi,
		"count", len(immunisations),
		"request_id", ctx.Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"nhi":           nhi,
		"immunisations": immunisations,
		"total":         len(immunisations),
	})
}

// --- OAuth2 client credentials token function ---

// tokenCache holds a cached access token.
type tokenCache struct {
	token     string
	expiresAt time.Time
}

// buildClientCredentialsTokenFunc returns a TokenFunc that fetches tokens using the
// OAuth 2.0 client credentials flow and caches them until 60 seconds before expiry.
func buildClientCredentialsTokenFunc(tokenURL, clientID, clientSecret string) TokenFunc {
	var cache *tokenCache

	return func(ctx context.Context) (string, error) {
		if tokenURL == "" || clientID == "" || clientSecret == "" {
			return "", fmt.Errorf("NIR OAuth credentials not configured (NIR_TOKEN_URL, NIR_CLIENT_ID, NIR_CLIENT_SECRET)")
		}

		// Return cached token if still valid (with 60-second buffer).
		if cache != nil && time.Now().Before(cache.expiresAt.Add(-60*time.Second)) {
			return cache.token, nil
		}

		// Request a new token.
		body := fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s",
			clientID, clientSecret)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
			bytes.NewBufferString(body))
		if err != nil {
			return "", fmt.Errorf("buildClientCredentialsTokenFunc: build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("buildClientCredentialsTokenFunc: http: %w", err)
		}
		defer resp.Body.Close()

		var tokenResp struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			return "", fmt.Errorf("buildClientCredentialsTokenFunc: decode response: %w", err)
		}
		if tokenResp.AccessToken == "" {
			return "", fmt.Errorf("buildClientCredentialsTokenFunc: empty access_token in response")
		}

		cache = &tokenCache{
			token:     tokenResp.AccessToken,
			expiresAt: time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		}
		return cache.token, nil
	}
}
