// Package medsafe provides a client for reporting adverse drug events (ADEs)
// to Medsafe (New Zealand Medicines and Medical Devices Safety Authority) as
// required under section 45 of the Medicines Act 1981. Pharmacists and
// prescribers have a professional obligation to report suspected ADEs using
// the Centre for Adverse Reactions Monitoring (CARM) system.
package medsafe

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

// Seriousness classifies the severity of an adverse drug event.
type Seriousness string

const (
	// SeriousDeath indicates the ADE resulted in patient death.
	SeriousDeath Seriousness = "death"
	// SeriousLifeThreatening indicates the ADE was immediately life-threatening.
	SeriousLifeThreatening Seriousness = "life-threatening"
	// SeriousHospitalisation indicates the ADE required or prolonged hospitalisation.
	SeriousHospitalisation Seriousness = "hospitalisation"
	// SeriousDisability indicates the ADE caused persistent significant disability.
	SeriousDisability Seriousness = "disability"
	// SeriousCongenital indicates a congenital anomaly or birth defect.
	SeriousCongenital Seriousness = "congenital"
	// NonSerious indicates the ADE was not serious by the above criteria.
	NonSerious Seriousness = "non-serious"
)

// CausalityAssessment reflects the WHO causality classification.
type CausalityAssessment string

const (
	CausalityCertain    CausalityAssessment = "certain"
	CausalityProbable   CausalityAssessment = "probable"
	CausalityPossible   CausalityAssessment = "possible"
	CausalityUnlikely   CausalityAssessment = "unlikely"
	CausalityConditional CausalityAssessment = "conditional"
	CausalityUnclassifiable CausalityAssessment = "unclassifiable"
)

// SuspectDrug describes a medicine suspected of causing the adverse event.
type SuspectDrug struct {
	// NZULM is the New Zealand Universal List of Medicines identifier.
	NZULM string `json:"nzulm"`
	// BrandName is the brand name dispensed (if known).
	BrandName string `json:"brandName,omitempty"`
	// GenericName is the INN / active ingredient.
	GenericName string `json:"genericName"`
	// Dose is the dose and unit (e.g. "500 mg").
	Dose string `json:"dose,omitempty"`
	// Route is the administration route (e.g. "oral", "IV").
	Route string `json:"route,omitempty"`
	// StartDate is when the patient first took the medicine.
	StartDate *time.Time `json:"startDate,omitempty"`
	// StopDate is when the medicine was stopped (nil if still ongoing).
	StopDate *time.Time `json:"stopDate,omitempty"`
	// Indication is the reason the medicine was prescribed.
	Indication string `json:"indication,omitempty"`
	// Causality is the reporter's assessment of causality.
	Causality CausalityAssessment `json:"causality,omitempty"`
	// ActionTaken describes what was done: "drug withdrawn", "dose reduced", "none".
	ActionTaken string `json:"actionTaken,omitempty"`
}

// ADEReport is a CARM adverse drug event report to be submitted to Medsafe.
type ADEReport struct {
	// ID is the internal UUID for this report.
	ID uuid.UUID `json:"id"`
	// CARMReportID is the CARM-assigned report number (populated after submission).
	CARMReportID string `json:"carmReportId,omitempty"`
	// PatientNHI is the NHI of the patient who experienced the adverse event.
	PatientNHI string `json:"patientNhi"`
	// PatientAge is the patient's age at the time of the event.
	PatientAge int `json:"patientAge,omitempty"`
	// PatientSex is "male", "female", or "unknown".
	PatientSex string `json:"patientSex,omitempty"`
	// ReporterHPI is the HPI CPN of the reporting clinician.
	ReporterHPI string `json:"reporterHpi"`
	// ReporterType is "prescriber", "pharmacist", "nurse", "consumer", or "other".
	ReporterType string `json:"reporterType"`
	// EventDate is when the adverse event first occurred.
	EventDate time.Time `json:"eventDate"`
	// EventDescription is a free-text description of the adverse reaction.
	EventDescription string `json:"eventDescription"`
	// Seriousness classifies the event by the ICH E2A criteria.
	Seriousness Seriousness `json:"seriousness"`
	// Outcome is the patient's outcome: "recovered", "recovering", "not-recovered",
	// "recovered-with-sequelae", "fatal", "unknown".
	Outcome string `json:"outcome,omitempty"`
	// SuspectDrugs lists all medicines suspected of causing the event.
	SuspectDrugs []SuspectDrug `json:"suspectDrugs"`
	// ConcomitantDrugs lists other medicines the patient was taking.
	ConcomitantDrugs []SuspectDrug `json:"concomitantDrugs,omitempty"`
	// RelevantHistory contains relevant past medical history.
	RelevantHistory string `json:"relevantHistory,omitempty"`
	// ReportedAt is when the report was submitted.
	ReportedAt time.Time `json:"reportedAt"`
}

// MedsafeError is returned when the Medsafe / CARM API responds with a
// structured error.
type MedsafeError struct {
	Code    string
	Message string
}

func (e *MedsafeError) Error() string {
	return fmt.Sprintf("medsafe: error %s: %s", e.Code, e.Message)
}

// Client is the Medsafe CARM ADE reporting client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	tokenFunc  func(ctx context.Context) (string, error)
}

// New constructs a Client targeting the CARM API baseURL.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		tokenFunc:  tokenFunc,
	}
}

// SubmitADE submits an adverse drug event report to the CARM system.
// The returned report will have CARMReportID populated if the submission was
// processed synchronously; otherwise it should be treated as pending.
func (c *Client) SubmitADE(ctx context.Context, report ADEReport) (*ADEReport, error) {
	if report.ID == uuid.Nil {
		report.ID = uuid.New()
	}
	report.ReportedAt = time.Now().UTC()

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("medsafe: obtaining token: %w", err)
	}

	body, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("medsafe: marshaling ADE report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/reports", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("medsafe: building SubmitADE request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("medsafe: POST report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result ADEReport
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("medsafe: decoding ADE response: %w", err)
	}
	return &result, nil
}

// GetReport retrieves a previously submitted ADE report by CARM report ID.
func (c *Client) GetReport(ctx context.Context, carmReportID string) (*ADEReport, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("medsafe: obtaining token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/reports/%s", c.baseURL, carmReportID), nil)
	if err != nil {
		return nil, fmt.Errorf("medsafe: building GetReport request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("medsafe: GET report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &MedsafeError{Code: "NOT_FOUND", Message: fmt.Sprintf("report %s not found", carmReportID)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(resp)
	}

	var result ADEReport
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("medsafe: decoding GetReport response: %w", err)
	}
	return &result, nil
}

// HealthCheck verifies connectivity to the Medsafe CARM API.
func (c *Client) HealthCheck(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/health", c.baseURL), nil)
	if err != nil {
		return false, fmt.Errorf("medsafe: health check request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("medsafe: health check: %w", err)
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
	return &MedsafeError{Code: body.Code, Message: body.Message}
}
