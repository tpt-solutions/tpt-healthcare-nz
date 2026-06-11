// Package toniq provides a Toniq PMS connector for the pharmacy dispensing
// gateway. Toniq is used by a significant number of New Zealand independent
// pharmacies. Like Fred Dispense, it exposes a FHIR R4 endpoint but uses
// different authentication and resource identifiers.
package toniq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	gateway "github.com/PhillipC05/tpt-healthcare/core/pharmacy-gateway"
)

// Connector implements gateway.Connector for Toniq.
type Connector struct {
	httpClient *http.Client
	baseURL    string
	clientID   string
	clientSecret string
}

// New constructs a Toniq connector. clientID and clientSecret are the OAuth2
// credentials issued by the Toniq integration portal.
func New(baseURL, clientID, clientSecret string) *Connector {
	return &Connector{
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		baseURL:      strings.TrimRight(baseURL, "/"),
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// Type returns the connector identifier.
func (c *Connector) Type() gateway.ConnectorType { return gateway.ConnectorToniq }

// Dispatch converts the DispatchRequest to a Toniq FHIR MedicationRequest and
// POSTs it to the Toniq FHIR endpoint.
func (c *Connector) Dispatch(ctx context.Context, req gateway.DispatchRequest) (*gateway.DispatchResult, error) {
	token, err := c.fetchToken(ctx)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(buildFHIRRequest(req))
	if err != nil {
		return nil, fmt.Errorf("toniq: marshal MedicationRequest: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/fhir/MedicationRequest", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("toniq: build dispatch request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/fhir+json")
	httpReq.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("toniq: POST MedicationRequest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var raw map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&raw)

	result := &gateway.DispatchResult{
		ID:           req.ID,
		Connector:    gateway.ConnectorToniq,
		Status:       gateway.DispensePending,
		DispatchedAt: time.Now().UTC(),
	}
	if id, ok := raw["id"].(string); ok {
		result.ExternalID = id
	}
	return result, nil
}

// GetStatus queries Toniq for the dispense status of a prescription.
func (c *Connector) GetStatus(ctx context.Context, externalID string) (gateway.DispenseStatus, error) {
	token, err := c.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/fhir/MedicationDispense?request=%s", c.baseURL, externalID), nil)
	if err != nil {
		return "", fmt.Errorf("toniq: build GetStatus request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("toniq: GET MedicationDispense: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", parseError(resp)
	}

	var bundle struct {
		Entry []struct {
			Resource struct {
				Status string `json:"status"`
			} `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return "", fmt.Errorf("toniq: decode status bundle: %w", err)
	}
	if len(bundle.Entry) == 0 {
		return gateway.DispensePending, nil
	}
	return fhirStatusToDispense(bundle.Entry[0].Resource.Status), nil
}

// Cancel requests cancellation of a prescription at Toniq.
func (c *Connector) Cancel(ctx context.Context, externalID string) error {
	token, err := c.fetchToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/fhir/MedicationRequest/%s", c.baseURL, externalID), nil)
	if err != nil {
		return fmt.Errorf("toniq: build Cancel request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("toniq: DELETE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return parseError(resp)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (c *Connector) fetchToken(ctx context.Context) (string, error) {
	payload := strings.NewReader(
		fmt.Sprintf("grant_type=client_credentials&client_id=%s&client_secret=%s",
			c.clientID, c.clientSecret))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/oauth/token", c.baseURL), payload)
	if err != nil {
		return "", fmt.Errorf("toniq: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("toniq: obtain token: %w", err)
	}
	defer resp.Body.Close()

	var tok struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("toniq: decode token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("toniq: empty access token")
	}
	return tok.AccessToken, nil
}

func buildFHIRRequest(req gateway.DispatchRequest) map[string]any {
	return map[string]any{
		"resourceType": "MedicationRequest",
		"id":           req.MedicationRequestID,
		"status":       "active",
		"intent":       "order",
		"subject": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  strings.ToUpper(req.PatientNHI),
			},
		},
		"requester": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/hpi-person-id",
				"value":  req.PrescriberHPI,
			},
		},
		"dispenseRequest": map[string]any{
			"performer": map[string]any{
				"identifier": map[string]any{
					"system": "https://standards.digital.health.nz/ns/hpi-facility-id",
					"value":  req.PharmacyHPI,
				},
			},
			"quantity":               map[string]any{"value": req.Quantity},
			"numberOfRepeatsAllowed": req.Repeats,
		},
		"medicationCodeableConcept": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "https://standards.digital.health.nz/ns/nzmt-id",
					"code":   req.NZULM,
				},
			},
		},
		"dosageInstruction": []any{
			map[string]any{
				"text": fmt.Sprintf("%s %s %s", req.Dose, req.Route, req.Frequency),
			},
		},
	}
}

func fhirStatusToDispense(status string) gateway.DispenseStatus {
	switch strings.ToLower(status) {
	case "completed":
		return gateway.DispensedComplete
	case "in-progress":
		return gateway.DispensePending
	case "stopped":
		return gateway.DispenseCancelled
	case "entered-in-error":
		return gateway.DispensedError
	default:
		return gateway.DispensePending
	}
}

func parseError(resp *http.Response) error {
	var oo struct {
		Issue []struct {
			Diagnostics string `json:"diagnostics"`
		} `json:"issue"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&oo)
	msg := fmt.Sprintf("unexpected status %d", resp.StatusCode)
	if len(oo.Issue) > 0 && oo.Issue[0].Diagnostics != "" {
		msg = oo.Issue[0].Diagnostics
	}
	return fmt.Errorf("toniq: %s", msg)
}
