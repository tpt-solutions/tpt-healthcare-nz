// Package msd provides clients for two Ministry of Social Development (MSD)
// APIs used at reception:
//
//  1. Community Services Card (CSC) eligibility check — determines whether a
//     patient is entitled to a reduced consultation fee under the CSC scheme.
//  2. NZBN lookup — queries the New Zealand Business Number register to verify
//     practice/entity details during onboarding.
package msd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Community Services Card
// ---------------------------------------------------------------------------

// CSCEligibility describes a patient's current CSC status.
type CSCEligibility struct {
	// PatientNHI is the NHI of the queried patient.
	PatientNHI string `json:"patientNhi"`
	// Eligible indicates whether the patient holds a valid CSC.
	Eligible bool `json:"eligible"`
	// CardNumber is the CSC card number (populated when Eligible=true).
	CardNumber string `json:"cardNumber,omitempty"`
	// ExpiryDate is when the card expires (zero if not eligible or unknown).
	ExpiryDate *time.Time `json:"expiryDate,omitempty"`
	// ReducedFeeScheme indicates which fee scheme applies:
	// "csc" = Community Services Card, "huhc" = High Use Health Card, "" = none.
	ReducedFeeScheme string `json:"reducedFeeScheme,omitempty"`
	// CheckedAt is the timestamp of this eligibility check.
	CheckedAt time.Time `json:"checkedAt"`
}

// CSCClient performs CSC eligibility lookups against the MSD API.
type CSCClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewCSCClient constructs a CSC eligibility client.
func NewCSCClient(baseURL, apiKey string) *CSCClient {
	return &CSCClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
	}
}

// CheckEligibility looks up the CSC status for a patient by NHI.
// The result should be cached at the session level; re-checking within the
// same appointment is unnecessary.
func (c *CSCClient) CheckEligibility(ctx context.Context, patientNHI string) (*CSCEligibility, error) {
	url := fmt.Sprintf("%s/csc/eligibility?nhi=%s", c.baseURL, strings.ToUpper(patientNHI))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("msd/csc: build request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("msd/csc: GET eligibility: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Patient not in MSD system — treat as not eligible.
		return &CSCEligibility{
			PatientNHI: patientNHI,
			Eligible:   false,
			CheckedAt:  time.Now().UTC(),
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseCSCError(resp, patientNHI)
	}

	var result CSCEligibility
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("msd/csc: decode response: %w", err)
	}
	result.PatientNHI = patientNHI
	result.CheckedAt = time.Now().UTC()
	return &result, nil
}

// HealthCheck verifies connectivity to the MSD CSC API.
func (c *CSCClient) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/health", c.baseURL), nil)
	if err != nil {
		return false, fmt.Errorf("msd/csc: health check request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("msd/csc: health check: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// ---------------------------------------------------------------------------
// NZBN lookup
// ---------------------------------------------------------------------------

// NZBNEntity holds the registration details for a New Zealand business entity.
type NZBNEntity struct {
	// NZBN is the 13-digit New Zealand Business Number.
	NZBN string `json:"nzbn"`
	// EntityName is the registered name of the entity.
	EntityName string `json:"entityName"`
	// EntityType is the legal entity type (e.g. "Limited Company", "Sole Trader").
	EntityType string `json:"entityType"`
	// NZBNStatus is the registration status: "active", "removed", "pending".
	NZBNStatus string `json:"nzbnStatus"`
	// RegisteredAddress is the entity's registered address.
	RegisteredAddress string `json:"registeredAddress,omitempty"`
	// GST is the entity's GST number (if registered for GST).
	GST string `json:"gst,omitempty"`
	// TradingNames is a list of trading names registered under the NZBN.
	TradingNames []string `json:"tradingNames,omitempty"`
	// LookedUpAt is the timestamp of this lookup.
	LookedUpAt time.Time `json:"lookedUpAt"`
}

// NZBNClient performs NZBN lookups against the business.govt.nz API.
type NZBNClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewNZBNClient constructs an NZBN lookup client.
// baseURL is typically "https://api.business.govt.nz/gateway/nzbn/v5".
func NewNZBNClient(baseURL, apiKey string) *NZBNClient {
	return &NZBNClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
	}
}

// LookupNZBN retrieves entity details for a 13-digit NZBN.
// Returns an error if the NZBN does not exist or is in an invalid format.
func (c *NZBNClient) LookupNZBN(ctx context.Context, nzbn string) (*NZBNEntity, error) {
	nzbn = strings.TrimSpace(nzbn)
	if len(nzbn) != 13 {
		return nil, fmt.Errorf("nzbn: invalid format — expected 13 digits, got %d", len(nzbn))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/entities/%s", c.baseURL, nzbn), nil)
	if err != nil {
		return nil, fmt.Errorf("nzbn: build request: %w", err)
	}
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nzbn: GET entity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("nzbn: entity %s not found", nzbn)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseNZBNError(resp, nzbn)
	}

	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("nzbn: decode response: %w", err)
	}

	entity := &NZBNEntity{
		NZBN:       nzbn,
		LookedUpAt: time.Now().UTC(),
	}
	entity.EntityName, _ = raw["entityName"].(string)
	entity.EntityType, _ = raw["entityTypeDescription"].(string)
	entity.NZBNStatus, _ = raw["entityStatusDescription"].(string)

	// Extract GST from identifiers if present.
	if ids, ok := raw["additionalIdentifiers"].([]any); ok {
		for _, idRaw := range ids {
			idMap, ok := idRaw.(map[string]any)
			if !ok {
				continue
			}
			if idType, _ := idMap["additionalIdentifierType"].(string); idType == "GST_NUMBER" {
				entity.GST, _ = idMap["identifier"].(string)
			}
		}
	}

	// Extract trading names.
	if trades, ok := raw["tradingNames"].([]any); ok {
		for _, t := range trades {
			if tm, ok := t.(map[string]any); ok {
				if name, ok := tm["name"].(string); ok && name != "" {
					entity.TradingNames = append(entity.TradingNames, name)
				}
			}
		}
	}

	return entity, nil
}

// SearchByName searches the NZBN register for entities matching name.
// Returns up to 10 results. Useful for autocomplete in the practice
// onboarding wizard.
func (c *NZBNClient) SearchByName(ctx context.Context, name string) ([]NZBNEntity, error) {
	url := fmt.Sprintf("%s/entities?search-term=%s&entity-status=50&page-size=10",
		c.baseURL, strings.ReplaceAll(name, " ", "+"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("nzbn: build search request: %w", err)
	}
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nzbn: search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseNZBNError(resp, "")
	}

	var bundle struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("nzbn: decode search response: %w", err)
	}

	entities := make([]NZBNEntity, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		e := NZBNEntity{LookedUpAt: time.Now().UTC()}
		e.NZBN, _ = item["nzbn"].(string)
		e.EntityName, _ = item["entityName"].(string)
		e.EntityType, _ = item["entityTypeDescription"].(string)
		e.NZBNStatus, _ = item["entityStatusDescription"].(string)
		entities = append(entities, e)
	}
	return entities, nil
}

// HealthCheck verifies connectivity to the NZBN API.
func (c *NZBNClient) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/health", c.baseURL), nil)
	if err != nil {
		return false, fmt.Errorf("nzbn: health check request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("nzbn: health check: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// ---------------------------------------------------------------------------
// Error helpers
// ---------------------------------------------------------------------------

// MSDError is a generic API error from either the CSC or NZBN clients.
type MSDError struct {
	Code    string
	Message string
	Context string
}

func (e *MSDError) Error() string {
	if e.Context != "" {
		return fmt.Sprintf("msd: %s — error %s: %s", e.Context, e.Code, e.Message)
	}
	return fmt.Sprintf("msd: error %s: %s", e.Code, e.Message)
}

func parseCSCError(resp *http.Response, nhi string) error {
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Code == "" {
		body.Code = fmt.Sprintf("HTTP_%d", resp.StatusCode)
	}
	return &MSDError{Code: body.Code, Message: body.Message, Context: "CSC/" + nhi}
}

func parseNZBNError(resp *http.Response, nzbn string) error {
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Code == "" {
		body.Code = fmt.Sprintf("HTTP_%d", resp.StatusCode)
	}
	ctx := "NZBN"
	if nzbn != "" {
		ctx = "NZBN/" + nzbn
	}
	return &MSDError{Code: body.Code, Message: body.Message, Context: ctx}
}
