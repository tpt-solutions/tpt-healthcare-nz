// Package worksafe provides a client for lodging and tracking workplace injury
// claims with WorkSafe New Zealand (api.worksafe.govt.nz). The API shape
// intentionally mirrors core/acc so claim-handling modules can dispatch to
// either destination based on whether the injury is work-related.
package worksafe

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

// ClaimStatus represents the lifecycle state of a WorkSafe claim.
type ClaimStatus string

const (
	ClaimPending  ClaimStatus = "pending"
	ClaimActive   ClaimStatus = "active"
	ClaimDeclined ClaimStatus = "declined"
	ClaimComplete ClaimStatus = "complete"
)

var terminalStatuses = map[ClaimStatus]bool{
	ClaimActive:   true,
	ClaimDeclined: true,
	ClaimComplete: true,
}

// InjuryMechanism classifies the type of workplace injury for WorkSafe reporting.
type InjuryMechanism string

const (
	MechanismFall          InjuryMechanism = "fall"
	MechanismLiftingStrain InjuryMechanism = "lifting-strain"
	MechanismChemicalExposure InjuryMechanism = "chemical-exposure"
	MechanismMachineryContact InjuryMechanism = "machinery-contact"
	MechanismRepetitiveStrain InjuryMechanism = "repetitive-strain"
	MechanismOther         InjuryMechanism = "other"
)

// WorkplaceClaim represents a WorkSafe NZ workplace injury claim.
type WorkplaceClaim struct {
	// ID is the internal UUID for this claim.
	ID uuid.UUID `json:"id"`
	// ReferenceNumber is the WorkSafe-assigned claim reference.
	ReferenceNumber string `json:"referenceNumber,omitempty"`
	// PatientNHI is the injured worker's NHI.
	PatientNHI string `json:"patientNhi"`
	// ProviderHPI is the HPI CPN of the treating practitioner.
	ProviderHPI string `json:"providerHpi"`
	// EmployerNZBN is the NZBN of the employer at the time of injury (if known).
	EmployerNZBN string `json:"employerNzbn,omitempty"`
	// DateOfInjury is when the workplace injury occurred.
	DateOfInjury time.Time `json:"dateOfInjury"`
	// InjuryDescription is a free-text account of the injury.
	InjuryDescription string `json:"injuryDescription"`
	// InjuryMechanism classifies how the injury occurred.
	InjuryMechanism InjuryMechanism `json:"injuryMechanism,omitempty"`
	// DiagnosisCodes contains ICD-10 codes for the diagnosed condition(s).
	DiagnosisCodes []string `json:"diagnosisCodes,omitempty"`
	// WorkplaceAddress is a plain-text description of the workplace location.
	WorkplaceAddress string `json:"workplaceAddress,omitempty"`
	// Status is the current lifecycle state.
	Status ClaimStatus `json:"status"`
	// LodgedAt is when the claim was submitted.
	LodgedAt time.Time `json:"lodgedAt"`
	// UpdatedAt is the last state-change timestamp.
	UpdatedAt time.Time `json:"updatedAt"`
}

// WorkSafeError is returned when the WorkSafe API responds with a structured error.
type WorkSafeError struct {
	Code            string
	Message         string
	ReferenceNumber string
}

func (e *WorkSafeError) Error() string {
	if e.ReferenceNumber != "" {
		return fmt.Sprintf("worksafe: error %s for claim %s: %s", e.Code, e.ReferenceNumber, e.Message)
	}
	return fmt.Sprintf("worksafe: error %s: %s", e.Code, e.Message)
}

// Client is a WorkSafe NZ FHIR API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a Client targeting baseURL. tokenFunc is called per-request to
// supply a current bearer token obtained from WorkSafe's OAuth2 endpoint.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// Lodge submits a new workplace injury claim to WorkSafe.
func (c *Client) Lodge(ctx context.Context, claim WorkplaceClaim) (*WorkplaceClaim, error) {
	if claim.ID == uuid.Nil {
		claim.ID = uuid.New()
	}
	claim.LodgedAt = time.Now().UTC()
	claim.UpdatedAt = claim.LodgedAt
	claim.Status = ClaimPending

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("worksafe: obtaining access token: %w", err)
	}

	payload, err := json.Marshal(buildFHIRClaim(claim))
	if err != nil {
		return nil, fmt.Errorf("worksafe: marshaling claim: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/Claim", c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("worksafe: building Lodge request: %w", err)
	}
	setHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worksafe: POST Claim: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseError(resp, "")
	}

	return decodeClaimResponse(resp.Body, claim)
}

// GetStatus retrieves the current status of a WorkSafe claim.
func (c *Client) GetStatus(ctx context.Context, referenceNumber string) (*WorkplaceClaim, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("worksafe: obtaining access token: %w", err)
	}

	url := fmt.Sprintf("%s/ClaimResponse?identifier=%s", c.baseURL, referenceNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("worksafe: building GetStatus request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worksafe: GET ClaimResponse: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &WorkSafeError{Code: "NOT_FOUND", Message: "claim not found", ReferenceNumber: referenceNumber}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp, referenceNumber)
	}

	return decodeBundle(resp.Body, referenceNumber)
}

// Poll repeatedly calls GetStatus until the claim reaches a terminal status or
// ctx is cancelled. Uses a 10-second polling interval.
func (c *Client) Poll(ctx context.Context, referenceNumber string) (*WorkplaceClaim, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	claim, err := c.GetStatus(ctx, referenceNumber)
	if err != nil {
		return nil, err
	}
	if terminalStatuses[claim.Status] {
		return claim, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("worksafe: polling claim %s: %w", referenceNumber, ctx.Err())
		case <-ticker.C:
			claim, err = c.GetStatus(ctx, referenceNumber)
			if err != nil {
				return nil, err
			}
			if terminalStatuses[claim.Status] {
				return claim, nil
			}
		}
	}
}

// HealthCheck pings the WorkSafe FHIR metadata endpoint to verify connectivity.
func (c *Client) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/metadata", c.baseURL), nil)
	if err != nil {
		return false, fmt.Errorf("worksafe: health check request: %w", err)
	}
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("worksafe: health check: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func setHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")
}

func buildFHIRClaim(c WorkplaceClaim) map[string]any {
	diagnoses := make([]any, 0, len(c.DiagnosisCodes))
	for i, code := range c.DiagnosisCodes {
		diagnoses = append(diagnoses, map[string]any{
			"sequence": i + 1,
			"diagnosisCodeableConcept": map[string]any{
				"coding": []any{map[string]any{
					"system": "http://hl7.org/fhir/sid/icd-10",
					"code":   code,
				}},
			},
		})
	}

	claim := map[string]any{
		"resourceType": "Claim",
		"id":           c.ID.String(),
		"status":       "active",
		"type": map[string]any{
			"coding": []any{map[string]any{
				"system": "http://terminology.hl7.org/CodeSystem/claim-type",
				"code":   "professional",
			}},
		},
		"use":     "claim",
		"created": c.LodgedAt.Format(time.RFC3339),
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
		"priority": map[string]any{"coding": []any{map[string]any{"code": "normal"}}},
		"accident": map[string]any{
			"date": c.DateOfInjury.Format("2006-01-02"),
			"type": map[string]any{
				"coding": []any{map[string]any{
					"system": "http://terminology.hl7.org/CodeSystem/v3-ActIncidentCode",
					"code":   "WPA", // Workplace Accident
				}},
			},
		},
		"diagnosis": diagnoses,
	}

	if c.EmployerNZBN != "" {
		claim["insurer"] = map[string]any{
			"identifier": map[string]any{
				"system": "https://www.business.govt.nz/nzbn/",
				"value":  c.EmployerNZBN,
			},
		}
	}
	if c.InjuryMechanism != "" {
		claim["supportingInfo"] = []any{
			map[string]any{
				"sequence": 1,
				"category": map[string]any{"text": "injuryMechanism"},
				"valueString": string(c.InjuryMechanism),
			},
		}
	}

	return claim
}

func decodeClaimResponse(body interface{ Read([]byte) (int, error) }, base WorkplaceClaim) (*WorkplaceClaim, error) {
	var raw map[string]any
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("worksafe: decoding Lodge response: %w", err)
	}
	result := base
	if ids, ok := raw["identifier"].([]any); ok {
		for _, idRaw := range ids {
			idMap, ok := idRaw.(map[string]any)
			if !ok {
				continue
			}
			sys, _ := idMap["system"].(string)
			val, _ := idMap["value"].(string)
			if strings.Contains(sys, "worksafe.govt.nz") && val != "" {
				result.ReferenceNumber = val
			}
		}
	}
	if outcome, ok := raw["outcome"].(string); ok {
		result.Status = outcomeToStatus(outcome)
	}
	result.UpdatedAt = time.Now().UTC()
	return &result, nil
}

func decodeBundle(body interface{ Read([]byte) (int, error) }, refNum string) (*WorkplaceClaim, error) {
	var bundle struct {
		Entry []struct {
			Resource map[string]any `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(body).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("worksafe: decoding bundle: %w", err)
	}
	if len(bundle.Entry) == 0 {
		return nil, &WorkSafeError{Code: "NOT_FOUND", Message: "no ClaimResponse found", ReferenceNumber: refNum}
	}

	raw := bundle.Entry[0].Resource
	claim := &WorkplaceClaim{ReferenceNumber: refNum, UpdatedAt: time.Now().UTC()}
	if outcome, ok := raw["outcome"].(string); ok {
		claim.Status = outcomeToStatus(outcome)
	}
	if created, ok := raw["created"].(string); ok {
		if t, err := time.Parse(time.RFC3339, created); err == nil {
			claim.LodgedAt = t
		}
	}
	return claim, nil
}

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

func parseError(resp *http.Response, refNum string) error {
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
	return &WorkSafeError{Code: code, Message: msg, ReferenceNumber: refNum}
}
