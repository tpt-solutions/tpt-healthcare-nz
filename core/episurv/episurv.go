// Package episurv provides a client for reporting notifiable diseases to
// EpiSurv, the ESR (Institute of Environmental Science and Research) national
// notifiable disease surveillance system. Reporting is mandatory under the
// Health Act 1956 and the Health (Notifiable Diseases, Conditions, and
// Risks) Regulations 2016 for the ~50 notifiable conditions in New Zealand.
package episurv

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

// NotifiableCondition is a disease or condition notifiable under the Health
// (Notifiable Diseases, Conditions, and Risks) Regulations 2016.
type NotifiableCondition string

// The most commonly encountered notifiable conditions. The full list of ~50
// is maintained at https://www.health.govt.nz/your-health/conditions-and-treatments/diseases-and-illnesses/notifiable-diseases
const (
	ConditionMeasles            NotifiableCondition = "measles"
	ConditionMumps              NotifiableCondition = "mumps"
	ConditionRubella            NotifiableCondition = "rubella"
	ConditionPertussis          NotifiableCondition = "pertussis"          // whooping cough
	ConditionTuberculosis       NotifiableCondition = "tuberculosis"
	ConditionSalmonella         NotifiableCondition = "salmonella"
	ConditionCampylobacteriosis NotifiableCondition = "campylobacteriosis"
	ConditionListeriosis        NotifiableCondition = "listeriosis"
	ConditionLegionellosis      NotifiableCondition = "legionellosis"
	ConditionHepatitisA         NotifiableCondition = "hepatitis-a"
	ConditionHepatitisB         NotifiableCondition = "hepatitis-b"
	ConditionHepatitisC         NotifiableCondition = "hepatitis-c"
	ConditionHIV                NotifiableCondition = "hiv"
	ConditionGonorrhoea         NotifiableCondition = "gonorrhoea"
	ConditionSyphilis           NotifiableCondition = "syphilis"
	ConditionCOVID19            NotifiableCondition = "covid-19"
	ConditionInfluenza          NotifiableCondition = "influenza"          // laboratory-confirmed
	ConditionMeningococcal      NotifiableCondition = "meningococcal-disease"
	ConditionTetanus            NotifiableCondition = "tetanus"
	ConditionTyphi              NotifiableCondition = "typhoid-fever"
	ConditionCryptosporidiosis  NotifiableCondition = "cryptosporidiosis"
	ConditionGiardiasis         NotifiableCondition = "giardiasis"
	ConditionRheumaticFever     NotifiableCondition = "acute-rheumatic-fever"
)

// CaseClassification reflects the confidence level of the diagnosis.
type CaseClassification string

const (
	ClassConfirmed  CaseClassification = "confirmed"
	ClassProbable   CaseClassification = "probable"
	ClassSuspect    CaseClassification = "suspect"
)

// NotificationStatus tracks the submission lifecycle.
type NotificationStatus string

const (
	NotificationDraft     NotificationStatus = "draft"
	NotificationSubmitted NotificationStatus = "submitted"
	NotificationAccepted  NotificationStatus = "accepted"
	NotificationRejected  NotificationStatus = "rejected"
)

// Notification is a notifiable disease report to be submitted to EpiSurv.
type Notification struct {
	// ID is the internal UUID for this notification.
	ID uuid.UUID `json:"id"`
	// EpiSurvID is the EpiSurv-assigned notification number (set after acceptance).
	EpiSurvID string `json:"episurvId,omitempty"`
	// PatientNHI is the NHI of the patient with the notifiable condition.
	PatientNHI string `json:"patientNhi"`
	// PatientDHB is the DHB of the patient's registered GP practice.
	PatientDHB string `json:"patientDhb,omitempty"`
	// ReporterHPI is the HPI CPN of the notifying practitioner.
	ReporterHPI string `json:"reporterHpi"`
	// Condition is the notifiable disease or condition being reported.
	Condition NotifiableCondition `json:"condition"`
	// Classification is the case classification at time of notification.
	Classification CaseClassification `json:"classification"`
	// OnsetDate is the approximate date symptoms first appeared.
	OnsetDate *time.Time `json:"onsetDate,omitempty"`
	// DiagnosisDate is the date the condition was diagnosed.
	DiagnosisDate time.Time `json:"diagnosisDate"`
	// LabConfirmed indicates whether laboratory confirmation was obtained.
	LabConfirmed bool `json:"labConfirmed"`
	// LabResultDetails contains relevant laboratory findings (free text).
	LabResultDetails string `json:"labResultDetails,omitempty"`
	// Hospitalised indicates whether the patient was hospitalised.
	Hospitalised bool `json:"hospitalised"`
	// HospitalAdmitDate is the date of hospital admission (if hospitalised).
	HospitalAdmitDate *time.Time `json:"hospitalAdmitDate,omitempty"`
	// DeathDate is the date of death if the patient died from the condition.
	DeathDate *time.Time `json:"deathDate,omitempty"`
	// EpidemiologicalNotes are free-text notes for the PHU epidemiologist.
	EpidemiologicalNotes string `json:"epidemiologicalNotes,omitempty"`
	// Status is the current submission lifecycle state.
	Status NotificationStatus `json:"status"`
	// NotifiedAt is when this notification was submitted.
	NotifiedAt time.Time `json:"notifiedAt"`
}

// EpiSurvError is returned when the EpiSurv API responds with a structured error.
type EpiSurvError struct {
	Code    string
	Message string
}

func (e *EpiSurvError) Error() string {
	return fmt.Sprintf("episurv: error %s: %s", e.Code, e.Message)
}

// Client is the EpiSurv notifiable disease reporting client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a Client targeting baseURL. tokenFunc returns a bearer token
// for the EpiSurv API (OAuth2 client credentials flow, issued by ESR).
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// Notify submits a notifiable disease notification to EpiSurv. Urgent
// notifications (meningococcal disease, measles, etc.) should also be
// telephoned to the local Public Health Unit — this client handles the
// electronic reporting component only.
func (c *Client) Notify(ctx context.Context, n Notification) (*Notification, error) {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	n.Status = NotificationSubmitted
	n.NotifiedAt = time.Now().UTC()

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("episurv: obtaining token: %w", err)
	}

	body, err := json.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("episurv: marshaling notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/notifications", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("episurv: building Notify request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("episurv: POST notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result Notification
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("episurv: decoding response: %w", err)
	}
	return &result, nil
}

// GetNotification retrieves a previously submitted notification by EpiSurv ID.
func (c *Client) GetNotification(ctx context.Context, episurvID string) (*Notification, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("episurv: obtaining token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/notifications/%s", c.baseURL, episurvID), nil)
	if err != nil {
		return nil, fmt.Errorf("episurv: building GetNotification request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("episurv: GET notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &EpiSurvError{Code: "NOT_FOUND", Message: fmt.Sprintf("notification %s not found", episurvID)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result Notification
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("episurv: decoding GetNotification response: %w", err)
	}
	return &result, nil
}

// IsNotifiable returns true if condition is on the current notifiable
// conditions list. Practitioners should call this before building a Notification
// to avoid submitting reports for non-notifiable diagnoses.
func IsNotifiable(condition NotifiableCondition) bool {
	switch condition {
	case ConditionMeasles, ConditionMumps, ConditionRubella, ConditionPertussis,
		ConditionTuberculosis, ConditionSalmonella, ConditionCampylobacteriosis,
		ConditionListeriosis, ConditionLegionellosis, ConditionHepatitisA,
		ConditionHepatitisB, ConditionHepatitisC, ConditionHIV,
		ConditionGonorrhoea, ConditionSyphilis, ConditionCOVID19,
		ConditionInfluenza, ConditionMeningococcal, ConditionTetanus,
		ConditionTyphi, ConditionCryptosporidiosis, ConditionGiardiasis,
		ConditionRheumaticFever:
		return true
	default:
		return false
	}
}

// HealthCheck verifies connectivity to the EpiSurv API.
func (c *Client) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/health", c.baseURL), nil)
	if err != nil {
		return false, fmt.Errorf("episurv: health check request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("episurv: health check: %w", err)
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
	return &EpiSurvError{Code: body.Code, Message: body.Message}
}
