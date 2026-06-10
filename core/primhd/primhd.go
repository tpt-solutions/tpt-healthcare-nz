// Package primhd provides a client for the PRIMHD (Programme for the
// Integration of Mental Health Data) outcomes reporting system, which is
// mandatory for all DHB-funded mental health and addiction services in
// New Zealand. Reporting is required under the Mental Health (Compulsory
// Assessment and Treatment) Act 1992 and associated DHB service agreements.
package primhd

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

// TeamType identifies the type of DHB mental health or addiction service team.
type TeamType string

const (
	TeamCommunityMentalHealth TeamType = "community-mental-health"
	TeamInpatient             TeamType = "inpatient"
	TeamForensic              TeamType = "forensic"
	TeamAddiction             TeamType = "addiction"
	TeamChildAdolescent       TeamType = "child-adolescent"
	TeamEarlyIntervention     TeamType = "early-intervention"
	TeamLiaison               TeamType = "liaison"
)

// LegalStatus reflects the patient's legal status under the Mental Health Act.
type LegalStatus string

const (
	LegalStatusVoluntary LegalStatus = "voluntary"
	LegalStatusSection2  LegalStatus = "section2"  // Assessment order
	LegalStatusSection3  LegalStatus = "section3"  // Compulsory treatment order
	LegalStatusSection29 LegalStatus = "section29" // Community treatment order
)

// ReferralStatus is the current state of the PRIMHD referral record.
type ReferralStatus string

const (
	ReferralOpen        ReferralStatus = "open"
	ReferralClosed      ReferralStatus = "closed"
	ReferralTransferred ReferralStatus = "transferred"
)

// ReferralRecord is the top-level PRIMHD reporting unit. It spans the entire
// episode of care from first contact to discharge.
type ReferralRecord struct {
	ID               uuid.UUID      `json:"id"`
	PRIMHDReferralID string         `json:"primhdReferralId,omitempty"`
	PatientNHI       string         `json:"patientNhi"`
	TeamType         TeamType       `json:"teamType"`
	TeamCode         string         `json:"teamCode"`
	ReferralDate     time.Time      `json:"referralDate"`
	DischargeDate    *time.Time     `json:"dischargeDate,omitempty"`
	LegalStatus      LegalStatus    `json:"legalStatus"`
	Status           ReferralStatus `json:"status"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// ActivityRecord represents a single contact or activity within a PRIMHD referral.
// Providers must submit an activity record for each clinical contact.
type ActivityRecord struct {
	ID           uuid.UUID `json:"id"`
	ReferralID   string    `json:"referralId"`
	ActivityType string    `json:"activityType"` // face-to-face, phone, group, etc.
	Duration     int       `json:"duration"`     // minutes
	ContactDate  time.Time `json:"contactDate"`
	ClinicianHPI string    `json:"clinicianHpi"`
	Setting      string    `json:"setting"` // outpatient, home, inpatient, etc.
}

// OutcomeRecord holds a HoNOS (Health of the Nation Outcome Scales) assessment,
// which is the mandated PRIMHD outcome measure.
type OutcomeRecord struct {
	ID                  uuid.UUID      `json:"id"`
	ReferralID          string         `json:"referralId"`
	Scale               string         `json:"scale"` // honos, honosca, honosolds
	TotalScore          int            `json:"totalScore"`
	ItemScores          map[string]int `json:"itemScores,omitempty"`
	AssessedAt          time.Time      `json:"assessedAt"`
	ReasonForAssessment string         `json:"reasonForAssessment"` // admission, review, discharge
	ClinicianHPI        string         `json:"clinicianHpi"`
}

// PRIMHDError is returned when the PRIMHD reporting endpoint responds with a
// structured error.
type PRIMHDError struct {
	Code    string
	Message string
}

func (e *PRIMHDError) Error() string {
	return fmt.Sprintf("primhd: error %s: %s", e.Code, e.Message)
}

// Client is the PRIMHD outcomes reporting client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a Client targeting baseURL. tokenFunc returns a bearer token
// for the PRIMHD reporting API (OAuth2 client credentials flow).
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// OpenReferral opens a new PRIMHD referral record when a patient is accepted
// into a DHB-funded mental health or addiction service.
func (c *Client) OpenReferral(ctx context.Context, rec ReferralRecord) (*ReferralRecord, error) {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	rec.Status = ReferralOpen
	rec.UpdatedAt = time.Now().UTC()

	var result ReferralRecord
	if err := c.post(ctx, "ReferralRecord", rec, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CloseReferral marks a PRIMHD referral as discharged. dischargeDate must not
// precede the referral date.
func (c *Client) CloseReferral(ctx context.Context, referralID string, dischargeDate time.Time) (*ReferralRecord, error) {
	payload := map[string]any{
		"primhdReferralId": referralID,
		"dischargeDate":    dischargeDate.Format("2006-01-02"),
		"status":           string(ReferralClosed),
	}
	var result ReferralRecord
	if err := c.post(ctx, fmt.Sprintf("ReferralRecord/%s/$close", referralID), payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SubmitActivity records a clinical contact against an open PRIMHD referral.
func (c *Client) SubmitActivity(ctx context.Context, act ActivityRecord) (*ActivityRecord, error) {
	if act.ID == uuid.Nil {
		act.ID = uuid.New()
	}
	var result ActivityRecord
	if err := c.post(ctx, "ActivityRecord", act, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SubmitOutcome submits a HoNOS outcome assessment for a PRIMHD referral.
func (c *Client) SubmitOutcome(ctx context.Context, outcome OutcomeRecord) (*OutcomeRecord, error) {
	if outcome.ID == uuid.Nil {
		outcome.ID = uuid.New()
	}
	var result OutcomeRecord
	if err := c.post(ctx, "OutcomeRecord", outcome, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// HealthCheck verifies connectivity to the PRIMHD reporting service.
func (c *Client) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/health", c.baseURL), nil)
	if err != nil {
		return false, fmt.Errorf("primhd: health check request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("primhd: health check: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// post is the shared internal helper: marshal payload → POST to path →
// decode 200/201 response into result, or return PRIMHDError.
func (c *Client) post(ctx context.Context, path string, payload any, result any) error {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return fmt.Errorf("primhd: obtaining token: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("primhd: marshaling %s: %w", path, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/%s", c.baseURL, path), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("primhd: building request for %s: %w", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("primhd: POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errBody struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return &PRIMHDError{Code: errBody.Code, Message: errBody.Message}
	}

	return json.NewDecoder(resp.Body).Decode(result)
}
