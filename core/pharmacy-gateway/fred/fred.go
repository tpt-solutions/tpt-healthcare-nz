// Package fred provides a Fred Dispense PMS connector for the pharmacy
// dispensing gateway. Fred Dispense is a widely used pharmacy PMS in New
// Zealand; it exposes a FHIR R4 REST API for receiving electronic prescriptions.
package fred

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

// Connector implements gateway.Connector for Fred Dispense.
type Connector struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// New constructs a Fred Dispense connector. apiKey is the API key issued by
// the pharmacy's Fred Dispense instance.
func New(baseURL, apiKey string) *Connector {
	return &Connector{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
	}
}

// Type returns the connector identifier.
func (c *Connector) Type() gateway.ConnectorType { return gateway.ConnectorFred }

// Dispatch converts the DispatchRequest into a FHIR R4 MedicationRequest and
// POSTs it to the Fred Dispense FHIR endpoint.
func (c *Connector) Dispatch(ctx context.Context, req gateway.DispatchRequest) (*gateway.DispatchResult, error) {
	fhirReq := buildFHIRMedicationRequest(req)
	body, err := json.Marshal(fhirReq)
	if err != nil {
		return nil, fmt.Errorf("fred: marshal MedicationRequest: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/MedicationRequest", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fred: build request: %w", err)
	}
	httpReq.Header.Set("X-Api-Key", c.apiKey)
	httpReq.Header.Set("Content-Type", "application/fhir+json")
	httpReq.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fred: POST MedicationRequest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseError(resp, "fred")
	}

	var raw map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&raw)

	result := &gateway.DispatchResult{
		ID:           req.ID,
		Connector:    gateway.ConnectorFred,
		Status:       gateway.DispensePending,
		DispatchedAt: time.Now().UTC(),
	}
	if id, ok := raw["id"].(string); ok {
		result.ExternalID = id
	}
	if note, ok := raw["note"].([]any); ok && len(note) > 0 {
		if m, ok := note[0].(map[string]any); ok {
			result.Message, _ = m["text"].(string)
		}
	}
	return result, nil
}

// GetStatus queries Fred Dispense for the current dispense status.
func (c *Connector) GetStatus(ctx context.Context, externalID string) (gateway.DispenseStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/MedicationDispense?request=%s", c.baseURL, externalID), nil)
	if err != nil {
		return "", fmt.Errorf("fred: build GetStatus request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fred: GET MedicationDispense: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", parseError(resp, "fred")
	}

	var bundle struct {
		Entry []struct {
			Resource struct {
				Status string `json:"status"`
			} `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return "", fmt.Errorf("fred: decode status: %w", err)
	}
	if len(bundle.Entry) == 0 {
		return gateway.DispensePending, nil
	}
	return fhirStatusToDispense(bundle.Entry[0].Resource.Status), nil
}

// Cancel sends a DELETE request to Fred Dispense for the prescription.
func (c *Connector) Cancel(ctx context.Context, externalID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/MedicationRequest/%s", c.baseURL, externalID), nil)
	if err != nil {
		return fmt.Errorf("fred: build Cancel request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fred: DELETE MedicationRequest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return parseError(resp, "fred")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func buildFHIRMedicationRequest(req gateway.DispatchRequest) map[string]any {
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
			"quantity": map[string]any{
				"value": req.Quantity,
			},
			"numberOfRepeatsAllowed": req.Repeats,
		},
		"medicationCodeableConcept": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "https://standards.digital.health.nz/ns/nzmt-id",
					"code":   req.NZULM,
					"display": func() string {
						if req.BrandName != "" {
							return req.BrandName
						}
						return req.NZULM
					}(),
				},
			},
		},
		"dosageInstruction": []any{
			map[string]any{
				"text": fmt.Sprintf("%s %s %s", req.Dose, req.Route, req.Frequency),
				"route": map[string]any{
					"text": req.Route,
				},
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

func parseError(resp *http.Response, prefix string) error {
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
	return fmt.Errorf("%s: %s", prefix, msg)
}
