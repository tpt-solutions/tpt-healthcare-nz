// Package erms provides a client for the ERMS (Electronic Referral Management
// System) used by Health New Zealand districts for routing specialist referrals
// between primary and secondary care. ERMS supplements the Healthlink EDI
// pathway with region-specific workflow rules (triage, waitlist, status updates).
package erms

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

// ReferralPriority maps to the ERMS clinical urgency classification.
type ReferralPriority string

const (
	PriorityUrgent   ReferralPriority = "urgent"   // clinic within 4 weeks
	PrioritySemiUrgent ReferralPriority = "semi-urgent" // clinic within 3 months
	PriorityRoutine  ReferralPriority = "routine"  // routine waitlist
)

// ReferralStatus is the current state in the ERMS workflow.
type ReferralStatus string

const (
	StatusDraft      ReferralStatus = "draft"
	StatusSubmitted  ReferralStatus = "submitted"
	StatusAccepted   ReferralStatus = "accepted"
	StatusDeclined   ReferralStatus = "declined"
	StatusWaitlisted ReferralStatus = "waitlisted"
	StatusBooked     ReferralStatus = "booked"
	StatusCompleted  ReferralStatus = "completed"
	StatusCancelled  ReferralStatus = "cancelled"
)

// Referral is the ERMS referral document sent from a GP or specialist to a
// Health NZ service.
type Referral struct {
	// ID is the internal UUID for this referral.
	ID uuid.UUID `json:"id"`
	// ERMSReferralID is the ERMS-assigned referral reference number.
	ERMSReferralID string `json:"ermsReferralId,omitempty"`
	// PatientNHI is the patient's NHI.
	PatientNHI string `json:"patientNhi"`
	// ReferrerHPI is the HPI CPN of the referring practitioner.
	ReferrerHPI string `json:"referrerHpi"`
	// ReferrerFacilityHPI is the HPI facility ID of the referring practice.
	ReferrerFacilityHPI string `json:"referrerFacilityHpi,omitempty"`
	// RecipientServiceCode is the ERMS service code for the destination
	// specialist service (e.g. "AUCK-ORTHO-01").
	RecipientServiceCode string `json:"recipientServiceCode"`
	// RecipientDHB is the Health NZ district (e.g. "te-whatu-ora-auckland").
	RecipientDHB string `json:"recipientDhb"`
	// Priority is the clinical urgency classification.
	Priority ReferralPriority `json:"priority"`
	// Specialty is the clinical specialty (e.g. "orthopaedics", "cardiology").
	Specialty string `json:"specialty"`
	// ClinicalSummary is the referring clinician's summary of the patient's
	// presentation and reason for referral.
	ClinicalSummary string `json:"clinicalSummary"`
	// AttachmentIDs are IDs of supporting documents attached to the referral
	// (e.g. lab results, imaging reports) stored in the document repository.
	AttachmentIDs []string `json:"attachmentIds,omitempty"`
	// Status is the current ERMS workflow state.
	Status ReferralStatus `json:"status"`
	// SubmittedAt is when the referral was sent to ERMS.
	SubmittedAt *time.Time `json:"submittedAt,omitempty"`
	// TriageDate is the target date for the initial triage decision.
	TriageDate *time.Time `json:"triageDate,omitempty"`
	// AppointmentDate is the booked appointment date (populated by ERMS on booking).
	AppointmentDate *time.Time `json:"appointmentDate,omitempty"`
	// UpdatedAt is the last state-change timestamp.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ERMSError is returned when the ERMS API responds with a structured error.
type ERMSError struct {
	Code    string
	Message string
}

func (e *ERMSError) Error() string {
	return fmt.Sprintf("erms: error %s: %s", e.Code, e.Message)
}

// Client is the ERMS electronic referral client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a Client targeting the ERMS API baseURL. tokenFunc returns
// a bearer token via the Health NZ IdP OAuth2 client credentials flow.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// Submit sends a new referral to ERMS. The referral is validated and routed
// to the destination service.
func (c *Client) Submit(ctx context.Context, ref Referral) (*Referral, error) {
	if ref.ID == uuid.Nil {
		ref.ID = uuid.New()
	}
	ref.Status = StatusSubmitted
	now := time.Now().UTC()
	ref.SubmittedAt = &now
	ref.UpdatedAt = now

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("erms: obtaining token: %w", err)
	}

	body, err := json.Marshal(ref)
	if err != nil {
		return nil, fmt.Errorf("erms: marshaling referral: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/referrals", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("erms: building Submit request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erms: POST referral: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result Referral
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erms: decoding Submit response: %w", err)
	}
	return &result, nil
}

// GetStatus retrieves the current status of a referral by ERMS referral ID.
func (c *Client) GetStatus(ctx context.Context, ermsReferralID string) (*Referral, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("erms: obtaining token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/referrals/%s", c.baseURL, ermsReferralID), nil)
	if err != nil {
		return nil, fmt.Errorf("erms: building GetStatus request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erms: GET referral: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &ERMSError{Code: "NOT_FOUND", Message: fmt.Sprintf("referral %s not found", ermsReferralID)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result Referral
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("erms: decoding GetStatus response: %w", err)
	}
	return &result, nil
}

// Cancel withdraws a previously submitted referral. Only referrals in
// StatusSubmitted, StatusAccepted, or StatusWaitlisted may be cancelled.
func (c *Client) Cancel(ctx context.Context, ermsReferralID, reason string) error {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return fmt.Errorf("erms: obtaining token: %w", err)
	}

	body, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return fmt.Errorf("erms: marshaling cancel: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/referrals/%s/cancel", c.baseURL, ermsReferralID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("erms: building Cancel request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erms: POST cancel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return parseError(resp)
	}
	return nil
}

// HealthCheck verifies connectivity to the ERMS API.
func (c *Client) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/health", c.baseURL), nil)
	if err != nil {
		return false, fmt.Errorf("erms: health check request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("erms: health check: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

func parseError(resp *http.Response) error {
	var body struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Code == "" {
		body.Code = fmt.Sprintf("HTTP_%d", resp.StatusCode)
	}
	if body.Message == "" {
		body.Message = fmt.Sprintf("unexpected status %d", resp.StatusCode)
	}
	return &ERMSError{Code: body.Code, Message: body.Message}
}
