// Package nes provides a client for the National Enrolment Service (NES),
// Te Whatu Ora's FHIR-based system for managing patient enrolments with
// general practices across New Zealand.
package nes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// EnrolmentStatus represents the lifecycle state of a patient enrolment.
type EnrolmentStatus string

const (
	// Active indicates the patient is currently enrolled at the practice.
	Active EnrolmentStatus = "active"
	// Pending indicates an enrolment request that has not yet been confirmed.
	Pending EnrolmentStatus = "pending"
	// Transferred indicates the patient has transferred to another practice.
	Transferred EnrolmentStatus = "transferred"
	// Deceased indicates the patient is deceased and the enrolment is closed.
	Deceased EnrolmentStatus = "deceased"
)

// Enrolment represents a patient's enrolment with a general practice.
type Enrolment struct {
	// NHI is the patient's National Health Index number.
	NHI string `json:"nhi"`
	// PracticeID is the HPI (Health Provider Index) facility identifier of the
	// enrolling general practice.
	PracticeID string `json:"practiceId"`
	// GPName is the display name of the enrolled general practice.
	GPName string `json:"gpName,omitempty"`
	// EnrolledAt is the date the enrolment became (or is expected to become)
	// effective.
	EnrolledAt time.Time `json:"enrolledAt"`
	// Status is the current lifecycle status of the enrolment.
	Status EnrolmentStatus `json:"status"`
}

// Client is a NES API client. It authenticates using SMART on FHIR
// client-credentials tokens supplied by tokenFunc.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a NES Client. baseURL should be the root of the NES FHIR
// endpoint (e.g. "https://api.integration.nes.health.govt.nz/fhir/r4").
// tokenFunc is called per request and must return a valid bearer token.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// Enrol creates a new enrolment for the patient (identified by nhi) at the
// practice identified by practiceID (HPI facility code).
func (c *Client) Enrol(ctx context.Context, nhi, practiceID string) (*Enrolment, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("nes: obtaining access token: %w", err)
	}

	payload, err := json.Marshal(buildEpisodeOfCare(nhi, practiceID, string(Pending), time.Now()))
	if err != nil {
		return nil, fmt.Errorf("nes: marshaling enrolment payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/EpisodeOfCare", c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("nes: building Enrol request: %w", err)
	}
	setFHIRHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nes: POST EpisodeOfCare: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nes: Enrol returned HTTP %d", resp.StatusCode)
	}

	return decodeEnrolment(resp)
}

// Update modifies an existing enrolment record. The enrolment's NHI and
// PracticeID must be set; all other fields will be updated to the supplied
// values.
func (c *Client) Update(ctx context.Context, e Enrolment) (*Enrolment, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("nes: obtaining access token: %w", err)
	}

	payload, err := json.Marshal(buildEpisodeOfCare(e.NHI, e.PracticeID, string(e.Status), e.EnrolledAt))
	if err != nil {
		return nil, fmt.Errorf("nes: marshaling update payload: %w", err)
	}

	// Use a conditional PUT against the patient's EpisodeOfCare.
	url := fmt.Sprintf("%s/EpisodeOfCare?patient.identifier=https://standards.digital.health.nz/ns/nhi-id|%s&managingOrganization.identifier=https://standards.digital.health.nz/ns/hpi-facility-id|%s",
		c.baseURL, e.NHI, e.PracticeID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("nes: building Update request: %w", err)
	}
	setFHIRHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nes: PUT EpisodeOfCare: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("nes: Update returned HTTP %d", resp.StatusCode)
	}

	return decodeEnrolment(resp)
}

// Transfer moves a patient's enrolment from fromPracticeID to toPracticeID.
// It closes the existing enrolment and creates a new one in a single
// $process-message operation.
func (c *Client) Transfer(ctx context.Context, nhi, fromPracticeID, toPracticeID string) error {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return fmt.Errorf("nes: obtaining access token: %w", err)
	}

	msg := buildTransferMessage(nhi, fromPracticeID, toPracticeID)
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("nes: marshaling transfer message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/$process-message", c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("nes: building Transfer request: %w", err)
	}
	setFHIRHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nes: $process-message Transfer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("nes: Transfer returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// GetStatus returns the current enrolment record for the given NHI number.
func (c *Client) GetStatus(ctx context.Context, nhi string) (*Enrolment, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("nes: obtaining access token: %w", err)
	}

	url := fmt.Sprintf("%s/EpisodeOfCare?patient.identifier=https://standards.digital.health.nz/ns/nhi-id|%s&status=active,pending", c.baseURL, nhi)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("nes: building GetStatus request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nes: GET EpisodeOfCare: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("nes: no enrolment found for NHI %s", nhi)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nes: GetStatus returned HTTP %d", resp.StatusCode)
	}

	// Response is a Bundle; decode the first entry.
	var bundle struct {
		Entry []struct {
			Resource json.RawMessage `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("nes: decoding GetStatus bundle: %w", err)
	}
	if len(bundle.Entry) == 0 {
		return nil, fmt.Errorf("nes: no enrolment found for NHI %s", nhi)
	}
	return decodeEpisodeOfCare(bundle.Entry[0].Resource)
}

// Withdraw terminates a patient's enrolment at the specified practice.
func (c *Client) Withdraw(ctx context.Context, nhi, practiceID string) error {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return fmt.Errorf("nes: obtaining access token: %w", err)
	}

	payload, err := json.Marshal(buildEpisodeOfCare(nhi, practiceID, "finished", time.Now()))
	if err != nil {
		return fmt.Errorf("nes: marshaling Withdraw payload: %w", err)
	}

	url := fmt.Sprintf("%s/EpisodeOfCare?patient.identifier=https://standards.digital.health.nz/ns/nhi-id|%s&managingOrganization.identifier=https://standards.digital.health.nz/ns/hpi-facility-id|%s",
		c.baseURL, nhi, practiceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("nes: building Withdraw request: %w", err)
	}
	setFHIRHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nes: PUT EpisodeOfCare (Withdraw): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("nes: Withdraw returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func setFHIRHeaders(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")
}

// buildEpisodeOfCare constructs a minimal FHIR EpisodeOfCare resource for NES.
func buildEpisodeOfCare(nhi, practiceID, status string, start time.Time) map[string]any {
	return map[string]any{
		"resourceType": "EpisodeOfCare",
		"status":       status,
		"patient": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  strings.ToUpper(nhi),
			},
		},
		"managingOrganization": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/hpi-facility-id",
				"value":  practiceID,
			},
		},
		"period": map[string]any{
			"start": start.Format(time.RFC3339),
		},
	}
}

// buildTransferMessage constructs a FHIR Bundle of type "message" for a NES
// enrolment transfer operation.
func buildTransferMessage(nhi, fromPracticeID, toPracticeID string) map[string]any {
	now := time.Now().UTC().Format(time.RFC3339)
	return map[string]any{
		"resourceType": "Bundle",
		"type":         "message",
		"timestamp":    now,
		"entry": []any{
			map[string]any{
				"resource": map[string]any{
					"resourceType": "MessageHeader",
					"eventCoding": map[string]any{
						"system": "https://standards.digital.health.nz/ns/nes-message-event",
						"code":   "enrolment-transfer",
					},
					"source": map[string]any{
						"endpoint": "urn:tpt-healthcare:nes-client",
					},
					"focus": []any{
						map[string]any{"reference": "#transfer-params"},
					},
				},
			},
			map[string]any{
				"fullUrl": "#transfer-params",
				"resource": map[string]any{
					"resourceType": "Parameters",
					"id":           "transfer-params",
					"parameter": []any{
						map[string]any{
							"name":        "nhi",
							"valueString": strings.ToUpper(nhi),
						},
						map[string]any{
							"name":        "fromPractice",
							"valueString": fromPracticeID,
						},
						map[string]any{
							"name":        "toPractice",
							"valueString": toPracticeID,
						},
					},
				},
			},
		},
	}
}

// decodeEnrolment decodes an HTTP response body containing a FHIR
// EpisodeOfCare resource into an Enrolment.
func decodeEnrolment(resp *http.Response) (*Enrolment, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("nes: decoding enrolment response: %w", err)
	}
	return decodeEpisodeOfCare(raw)
}

// decodeEpisodeOfCare maps a raw FHIR EpisodeOfCare JSON blob to an Enrolment.
func decodeEpisodeOfCare(raw json.RawMessage) (*Enrolment, error) {
	var eoc struct {
		Status  string `json:"status"`
		Patient struct {
			Identifier struct {
				Value string `json:"value"`
			} `json:"identifier"`
		} `json:"patient"`
		ManagingOrganization struct {
			Identifier struct {
				Value string `json:"value"`
			} `json:"identifier"`
			Display string `json:"display"`
		} `json:"managingOrganization"`
		Period struct {
			Start string `json:"start"`
		} `json:"period"`
	}
	if err := json.Unmarshal(raw, &eoc); err != nil {
		return nil, fmt.Errorf("nes: parsing EpisodeOfCare: %w", err)
	}

	var enrolledAt time.Time
	if eoc.Period.Start != "" {
		t, err := time.Parse(time.RFC3339, eoc.Period.Start)
		if err != nil {
			// Attempt date-only fallback used by some NES environments.
			t, err = time.Parse("2006-01-02", eoc.Period.Start)
			if err != nil {
				return nil, fmt.Errorf("nes: parsing period.start %q: %w", eoc.Period.Start, err)
			}
		}
		enrolledAt = t
	}

	status := EnrolmentStatus(eoc.Status)

	return &Enrolment{
		NHI:        eoc.Patient.Identifier.Value,
		PracticeID: eoc.ManagingOrganization.Identifier.Value,
		GPName:     eoc.ManagingOrganization.Display,
		EnrolledAt: enrolledAt,
		Status:     status,
	}, nil
}
