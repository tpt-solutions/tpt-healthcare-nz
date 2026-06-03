// Package pharmac provides a client for the PHARMAC pharmaceutical schedule,
// enabling formulary lookups, subsidy queries, and drug interaction checks
// for medicines funded under New Zealand's national drug subsidy scheme.
package pharmac

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Subsidy describes the level of PHARMAC funding applicable to a medicine.
type Subsidy string

const (
	// FullSubsidy indicates the medicine is fully subsidised (patient pays
	// only the standard co-payment).
	FullSubsidy Subsidy = "full"
	// PartSubsidy indicates a partial subsidy applies; the patient pays the
	// remainder above the subsidy.
	PartSubsidy Subsidy = "partial"
	// Restricted indicates the medicine is subsidised only under specific
	// Special Authority or access criteria.
	Restricted Subsidy = "restricted"
	// HospitalSupply indicates the medicine is supplied via hospital pharmacy
	// only and is not dispensed in the community.
	HospitalSupply Subsidy = "hospital"
	// Unsubsidised indicates no PHARMAC subsidy applies.
	Unsubsidised Subsidy = "unsubsidised"
)

// Medicine represents a medicine entry in the PHARMAC pharmaceutical schedule.
type Medicine struct {
	// NZULM is the New Zealand Universal List of Medicines identifier.
	NZULM string `json:"nzulm"`
	// BrandName is the proprietary name of the medicine (e.g. "Losec").
	BrandName string `json:"brandName,omitempty"`
	// GenericName is the INN / active ingredient name (e.g. "omeprazole").
	GenericName string `json:"genericName"`
	// Form is the dosage form (e.g. "tablet", "capsule", "injection").
	Form string `json:"form,omitempty"`
	// Strength is the dose strength (e.g. "20 mg").
	Strength string `json:"strength,omitempty"`
	// SubsidyType indicates the level of PHARMAC funding.
	SubsidyType Subsidy `json:"subsidyType"`
	// SubsidyAmount is the maximum subsidised price in NZD cents (inclusive of
	// GST where applicable).
	SubsidyAmount int64 `json:"subsidyAmount"`
	// TherapeuticGroup is the PHARMAC therapeutic group classification.
	TherapeuticGroup string `json:"therapeuticGroup,omitempty"`
	// ScheduleSection is the section of the Pharmaceutical Schedule (e.g. "A",
	// "B", "H") under which the medicine is listed.
	ScheduleSection string `json:"scheduleSection,omitempty"`
	// RestrictedIndicator flags whether Special Authority criteria apply.
	RestrictedIndicator bool `json:"restrictedIndicator"`
	// RestrictedReason describes the access restriction criteria if applicable.
	RestrictedReason string `json:"restrictedReason,omitempty"`
}

// Interaction represents a clinically significant interaction between two
// medicines, as returned by the interaction check service.
type Interaction struct {
	// Drug1 is the NZULM of the first medicine involved.
	Drug1 string `json:"drug1"`
	// Drug2 is the NZULM of the second medicine involved.
	Drug2 string `json:"drug2"`
	// Severity classifies the interaction risk: "contraindicated", "major",
	// "moderate", or "minor".
	Severity string `json:"severity"`
	// Description is a plain-language summary of the interaction and any
	// recommended management action.
	Description string `json:"description"`
}

// Client is a PHARMAC schedule and interaction API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a Client targeting baseURL. tokenFunc is called per request
// to obtain a current bearer token; it may return an empty string if the
// PHARMAC endpoint is publicly accessible.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// Search returns medicines matching the supplied query string, which may be a
// brand name, generic name, or NZULM identifier prefix.
func (c *Client) Search(ctx context.Context, query string) ([]Medicine, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("pharmac: search query must not be empty")
	}

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("pharmac: obtaining access token: %w", err)
	}

	u := fmt.Sprintf("%s/medicines?q=%s", c.baseURL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("pharmac: building Search request: %w", err)
	}
	setHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pharmac: GET medicines search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pharmac: Search returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Medicines []Medicine `json:"medicines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("pharmac: decoding Search response: %w", err)
	}
	return result.Medicines, nil
}

// GetByNZULM fetches a single medicine by its exact NZULM identifier.
func (c *Client) GetByNZULM(ctx context.Context, nzulm string) (*Medicine, error) {
	if strings.TrimSpace(nzulm) == "" {
		return nil, fmt.Errorf("pharmac: NZULM identifier must not be empty")
	}

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("pharmac: obtaining access token: %w", err)
	}

	u := fmt.Sprintf("%s/medicines/%s", c.baseURL, url.PathEscape(nzulm))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("pharmac: building GetByNZULM request: %w", err)
	}
	setHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pharmac: GET medicine/%s: %w", nzulm, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pharmac: medicine with NZULM %s not found", nzulm)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pharmac: GetByNZULM returned HTTP %d", resp.StatusCode)
	}

	var m Medicine
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("pharmac: decoding medicine response: %w", err)
	}
	return &m, nil
}

// CheckInteractions queries the interaction service for all pairwise
// interactions among the supplied NZULM identifiers. A minimum of two NZULMs
// must be provided.
func (c *Client) CheckInteractions(ctx context.Context, nzulms []string) ([]Interaction, error) {
	if len(nzulms) < 2 {
		return nil, fmt.Errorf("pharmac: at least 2 NZULMs are required for interaction check")
	}

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("pharmac: obtaining access token: %w", err)
	}

	payload, err := json.Marshal(map[string]any{"nzulms": nzulms})
	if err != nil {
		return nil, fmt.Errorf("pharmac: marshaling interaction request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/interactions", c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("pharmac: building CheckInteractions request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pharmac: POST interactions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pharmac: CheckInteractions returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Interactions []Interaction `json:"interactions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("pharmac: decoding interactions response: %w", err)
	}
	return result.Interactions, nil
}

// GetSubsidySchedule returns the full current PHARMAC Pharmaceutical Schedule.
// This may be a large response; callers should consider caching the result.
func (c *Client) GetSubsidySchedule(ctx context.Context) ([]Medicine, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("pharmac: obtaining access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/schedule", c.baseURL), nil)
	if err != nil {
		return nil, fmt.Errorf("pharmac: building GetSubsidySchedule request: %w", err)
	}
	setHeaders(req, token)

	// The schedule endpoint may be slow; use a longer timeout for this call.
	scheduleClient := *c.httpClient
	scheduleClient.Timeout = 120 * time.Second

	resp, err := scheduleClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pharmac: GET schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pharmac: GetSubsidySchedule returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Medicines []Medicine `json:"medicines"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("pharmac: decoding schedule response: %w", err)
	}
	return result.Medicines, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func setHeaders(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/json")
}
