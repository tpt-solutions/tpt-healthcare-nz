// Package acc provides a FHIR-based client for lodging and tracking claims
// with ACC (Accident Compensation Corporation), New Zealand's national
// accidental injury insurance scheme.
package acc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ClaimStatus represents the lifecycle state of an ACC claim.
type ClaimStatus string

const (
	// ClaimPending indicates the claim has been submitted but not yet assessed.
	ClaimPending ClaimStatus = "pending"
	// ClaimActive indicates an accepted, ongoing claim.
	ClaimActive ClaimStatus = "active"
	// ClaimDeclined indicates the claim was assessed and declined.
	ClaimDeclined ClaimStatus = "declined"
	// ClaimComplete indicates the claim has been fully resolved.
	ClaimComplete ClaimStatus = "complete"
	// ClaimDisputed indicates the decision on the claim is under review or dispute.
	ClaimDisputed ClaimStatus = "disputed"
)

// terminalStatuses is the set of ClaimStatus values after which no further
// state changes are expected, used by Poll to stop waiting.
var terminalStatuses = map[ClaimStatus]bool{
	ClaimActive:   true,
	ClaimDeclined: true,
	ClaimComplete: true,
	ClaimDisputed: true,
}

// Claim represents an ACC injury claim.
type Claim struct {
	// ID is the internal UUID assigned at lodgement time.
	ID uuid.UUID `json:"id"`
	// ClaimNumber is the ACC-assigned claim reference number.
	ClaimNumber string `json:"claimNumber,omitempty"`
	// PurchaseOrderNumber is an optional purchase order reference.
	PurchaseOrderNumber string `json:"purchaseOrderNumber,omitempty"`
	// PatientNHI is the NHI of the injured patient.
	PatientNHI string `json:"patientNhi"`
	// ProviderHPI is the HPI CPN of the treating provider.
	ProviderHPI string `json:"providerHpi"`
	// DateOfAccident is the date and time the injury occurred.
	DateOfAccident time.Time `json:"dateOfAccident"`
	// InjuryDescription is a free-text summary of the injury.
	InjuryDescription string `json:"injuryDescription"`
	// DiagnosisCodes contains ICD-10 or READ codes for the diagnosed condition(s).
	DiagnosisCodes []string `json:"diagnosisCodes,omitempty"`
	// Status is the current lifecycle status of the claim.
	Status ClaimStatus `json:"status"`
	// LodgedAt is when the claim was first submitted.
	LodgedAt time.Time `json:"lodgedAt"`
	// UpdatedAt is when the claim record was last modified.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ACCError is returned when the ACC FHIR endpoint responds with a structured
// error, preserving the ACC error code alongside a human-readable message.
type ACCError struct {
	// Code is the ACC-specific error code (e.g. "CLAIM_DUPLICATE").
	Code string
	// Message is the human-readable error description.
	Message string
	// ClaimNumber is populated when the error relates to a specific claim.
	ClaimNumber string
}

func (e *ACCError) Error() string {
	if e.ClaimNumber != "" {
		return fmt.Sprintf("acc: error %s for claim %s: %s", e.Code, e.ClaimNumber, e.Message)
	}
	return fmt.Sprintf("acc: error %s: %s", e.Code, e.Message)
}

// Client is an ACC FHIR API client authenticated via SMART on FHIR bearer tokens.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a Client targeting baseURL. tokenFunc is called per request
// to supply a current bearer token.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// Lodge submits a new ACC claim via the FHIR Claim resource endpoint.
// The returned Claim will have ClaimNumber populated if ACC accepted the
// submission synchronously; otherwise it will be empty and must be polled.
func (c *Client) Lodge(ctx context.Context, claim Claim) (*Claim, error) {
	if claim.ID == uuid.Nil {
		claim.ID = uuid.New()
	}
	claim.LodgedAt = time.Now().UTC()
	claim.UpdatedAt = claim.LodgedAt
	claim.Status = ClaimPending

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("acc: obtaining access token: %w", err)
	}

	fhirClaim := buildFHIRClaim(claim)
	payload, err := json.Marshal(fhirClaim)
	if err != nil {
		return nil, fmt.Errorf("acc: marshaling FHIR Claim: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/Claim", c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("acc: building Lodge request: %w", err)
	}
	setHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("acc: POST Claim: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp, "")
	}

	return decodeClaimResponse(resp.Body, claim)
}

// GetStatus retrieves the current status of an ACC claim by its claim number.
// It looks up the corresponding FHIR ClaimResponse resource.
func (c *Client) GetStatus(ctx context.Context, claimNumber string) (*Claim, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("acc: obtaining access token: %w", err)
	}

	url := fmt.Sprintf("%s/ClaimResponse?identifier=%s", c.baseURL, claimNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("acc: building GetStatus request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("acc: GET ClaimResponse: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &ACCError{Code: "NOT_FOUND", Message: "claim not found", ClaimNumber: claimNumber}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp, claimNumber)
	}

	return decodeClaimResponseBundle(resp.Body, claimNumber)
}

// Poll repeatedly calls GetStatus until the claim reaches a terminal status
// (Active, Declined, Complete, Disputed) or ctx is cancelled. It uses a
// fixed 10-second polling interval.
func (c *Client) Poll(ctx context.Context, claimNumber string) (*Claim, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Check immediately before waiting for the first tick.
	claim, err := c.GetStatus(ctx, claimNumber)
	if err != nil {
		return nil, err
	}
	if terminalStatuses[claim.Status] {
		return claim, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acc: polling claim %s: %w", claimNumber, ctx.Err())
		case <-ticker.C:
			claim, err = c.GetStatus(ctx, claimNumber)
			if err != nil {
				return nil, err
			}
			if terminalStatuses[claim.Status] {
				return claim, nil
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func setHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")
}

// buildFHIRClaim converts an internal Claim to a FHIR Claim resource map.
func buildFHIRClaim(c Claim) map[string]any {
	diagnoses := make([]any, 0, len(c.DiagnosisCodes))
	for i, code := range c.DiagnosisCodes {
		diagnoses = append(diagnoses, map[string]any{
			"sequence": i + 1,
			"diagnosisCodeableConcept": map[string]any{
				"coding": []any{
					map[string]any{
						"system": "http://hl7.org/fhir/sid/icd-10",
						"code":   code,
					},
				},
			},
		})
	}

	claim := map[string]any{
		"resourceType": "Claim",
		"id":           c.ID.String(),
		"status":       "active",
		"type": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "http://terminology.hl7.org/CodeSystem/claim-type",
					"code":   "professional",
				},
			},
		},
		"use":         "claim",
		"created":     c.LodgedAt.Format(time.RFC3339),
		"patient": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  strings.ToUpper(c.PatientNHI),
			},
		},
		"provider": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/hpi-person-id",
				"value":  c.ProviderHPI,
			},
		},
		"priority": map[string]any{
			"coding": []any{
				map[string]any{
					"code": "normal",
				},
			},
		},
		"accident": map[string]any{
			"date": c.DateOfAccident.Format("2006-01-02"),
		},
		"diagnosis": diagnoses,
	}

	if c.InjuryDescription != "" {
		claim["text"] = map[string]any{
			"status": "generated",
			"div":    fmt.Sprintf("<div>%s</div>", c.InjuryDescription),
		}
	}
	if c.PurchaseOrderNumber != "" {
		claim["identifier"] = []any{
			map[string]any{
				"system": "https://acc.govt.nz/ns/purchase-order",
				"value":  c.PurchaseOrderNumber,
			},
		}
	}

	return claim
}

// decodeClaimResponse parses a FHIR Claim or ClaimResponse body, merging
// server-assigned fields into the provided base Claim.
func decodeClaimResponse(body interface{ Read([]byte) (int, error) }, base Claim) (*Claim, error) {
	var raw map[string]any
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("acc: decoding Lodge response: %w", err)
	}

	result := base
	if id, ok := raw["id"].(string); ok && id != "" {
		if parsed, err := uuid.Parse(id); err == nil {
			result.ID = parsed
		}
	}

	// Extract ACC claim number from identifiers if present.
	if ids, ok := raw["identifier"].([]any); ok {
		for _, idRaw := range ids {
			idMap, ok := idRaw.(map[string]any)
			if !ok {
				continue
			}
			sys, _ := idMap["system"].(string)
			val, _ := idMap["value"].(string)
			if strings.Contains(sys, "acc.govt.nz") && val != "" {
				result.ClaimNumber = val
			}
		}
	}

	if outcome, ok := raw["outcome"].(string); ok {
		result.Status = outcomeToStatus(outcome)
	}
	result.UpdatedAt = time.Now().UTC()

	return &result, nil
}

// decodeClaimResponseBundle parses a FHIR Bundle of ClaimResponse entries.
func decodeClaimResponseBundle(body interface{ Read([]byte) (int, error) }, claimNumber string) (*Claim, error) {
	var bundle struct {
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			Resource map[string]any `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(body).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("acc: decoding ClaimResponse bundle: %w", err)
	}
	if len(bundle.Entry) == 0 {
		return nil, &ACCError{Code: "NOT_FOUND", Message: "no ClaimResponse found", ClaimNumber: claimNumber}
	}

	raw := bundle.Entry[0].Resource
	claim := &Claim{ClaimNumber: claimNumber, UpdatedAt: time.Now().UTC()}

	if outcome, ok := raw["outcome"].(string); ok {
		claim.Status = outcomeToStatus(outcome)
	}
	if created, ok := raw["created"].(string); ok {
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			claim.LodgedAt = t
		}
	}

	// Extract patient NHI from the request reference if present.
	if patRef, ok := raw["patient"].(map[string]any); ok {
		if ident, ok := patRef["identifier"].(map[string]any); ok {
			if val, ok := ident["value"].(string); ok {
				claim.PatientNHI = val
			}
		}
	}

	return claim, nil
}

// outcomeToStatus maps a FHIR ClaimResponse outcome code to a ClaimStatus.
func outcomeToStatus(outcome string) ClaimStatus {
	switch strings.ToLower(outcome) {
	case "complete":
		return ClaimComplete
	case "error", "partial":
		return ClaimDeclined
	case "queued":
		return ClaimPending
	default:
		return ClaimActive
	}
}

// parseErrorResponse attempts to decode a FHIR OperationOutcome from an error
// response and wraps it in an ACCError.
func parseErrorResponse(resp *http.Response, claimNumber string) error {
	var oo struct {
		Issue []struct {
			Code        string `json:"code"`
			Diagnostics string `json:"diagnostics"`
		} `json:"issue"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&oo)

	code := fmt.Sprintf("HTTP_%d", resp.StatusCode)
	msg := fmt.Sprintf("unexpected status %d", resp.StatusCode)
	if len(oo.Issue) > 0 {
		code = oo.Issue[0].Code
		msg = oo.Issue[0].Diagnostics
	}
	return &ACCError{Code: code, Message: msg, ClaimNumber: claimNumber}
}
