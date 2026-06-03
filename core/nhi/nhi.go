// Package nhi provides an NHI (National Health Index) FHIR API client for
// Te Whatu Ora (Health New Zealand). It covers NHI validation, patient lookup
// by NHI, and patient matching via the FHIR $match operation.
package nhi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	// TODO: replace with proper FHIR R4 types once
	// github.com/PhillipC05/tpt-healthcare/core/fhir/r4 is implemented.
	fhirr4 "github.com/PhillipC05/tpt-healthcare/core/fhir/r4"
)

// nhiOldPattern matches the legacy NHI format: 3 letters + 4 digits.
// Example: ABC1234
var nhiOldPattern = regexp.MustCompile(`(?i)^[A-Z]{3}\d{4}$`)

// nhiNewPattern matches the new NHI format introduced by Te Whatu Ora:
// starts with Z, followed by 2 letters, 2 digits, then 2 letters.
// Example: ZAB1234 (the new format is ZAA1234 through ZZZ9999 with alpha check).
var nhiNewPattern = regexp.MustCompile(`(?i)^Z[A-Z]{2}\d{2}[A-Z]{2}$`)

// letterValues maps uppercase letters to their NHI checksum alphabet values
// (A=1, B=2, … Z=26, skipping I and O per the NHI spec).
var letterValues = func() map[rune]int {
	m := make(map[rune]int, 26)
	val := 1
	for c := 'A'; c <= 'Z'; c++ {
		if c == 'I' || c == 'O' {
			continue
		}
		m[c] = val
		val++
	}
	return m
}()

// ValidateNHI reports whether nhi is a valid NHI number.
//
// Old format (e.g. ABC1234): 3 letters + 4 digits where the last digit is a
// Luhn-like check digit. The algorithm multiplies each of the first 6
// characters' values by descending weights 7 down to 2, sums the products,
// computes (sum mod 11), and derives the check digit as (11 - result). A
// computed check digit of 0 is valid; 10 is invalid.
//
// New format (e.g. ZAB1234): starts with Z, basic format check only — the
// checksum scheme differs and is verified by the HPI/NHI service.
func ValidateNHI(nhi string) bool {
	upper := strings.ToUpper(strings.TrimSpace(nhi))

	if nhiNewPattern.MatchString(upper) {
		// New-format NHIs starting with Z use a different check mechanism
		// enforced server-side; we validate structure only.
		return true
	}

	if !nhiOldPattern.MatchString(upper) {
		return false
	}

	// Old-format checksum: positions 0-5 (3 letters + first 3 digits) weighted
	// by 7,6,5,4,3,2; position 6 is the check digit.
	//
	// Character value:
	//   - Letters: use letterValues map (A=1…Z=24, skipping I and O)
	//   - Digits:  face value (0-9)
	weights := [6]int{7, 6, 5, 4, 3, 2}
	sum := 0
	for i := 0; i < 6; i++ {
		ch := rune(upper[i])
		var val int
		if ch >= 'A' && ch <= 'Z' {
			v, ok := letterValues[ch]
			if !ok {
				// Letter I or O — not permitted in old-format NHI.
				return false
			}
			val = v
		} else {
			val = int(ch - '0')
		}
		sum += val * weights[i]
	}

	remainder := sum % 11
	if remainder == 0 {
		// check digit would need to be 11, which is not a single digit — invalid.
		return false
	}
	computed := 11 - remainder
	if computed == 10 {
		return false
	}

	actual := int(upper[6] - '0')
	return computed%10 == actual
}

// MatchParams holds the demographic parameters used for a FHIR $match search.
type MatchParams struct {
	GivenName  string
	FamilyName string
	BirthDate  time.Time
	Gender     string // "male" | "female" | "other" | "unknown"
	Address    string
}

// Client is an NHI FHIR API client. It obtains short-lived bearer tokens via
// the supplied tokenFunc (SMART on FHIR client-credentials flow) before every
// request.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	accessToken func(ctx context.Context) (string, error)
}

// New constructs a Client targeting the given baseURL. tokenFunc is called
// per request to supply a current SMART on FHIR access token.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		baseURL:     strings.TrimRight(baseURL, "/"),
		accessToken: tokenFunc,
	}
}

// GetPatient retrieves a FHIR Patient resource by NHI number.
// The NHI is validated locally before the remote call is made.
func (c *Client) GetPatient(ctx context.Context, nhi string) (*fhirr4.Patient, error) {
	if !ValidateNHI(nhi) {
		return nil, fmt.Errorf("nhi: invalid NHI format or checksum: %q", nhi)
	}

	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("nhi: failed to obtain access token: %w", err)
	}

	url := fmt.Sprintf("%s/Patient/%s", c.baseURL, strings.ToUpper(strings.TrimSpace(nhi)))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("nhi: building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nhi: GET Patient/%s: %w", nhi, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nhi: GET Patient/%s returned HTTP %d", nhi, resp.StatusCode)
	}

	var patient fhirr4.Patient
	if err := json.NewDecoder(resp.Body).Decode(&patient); err != nil {
		return nil, fmt.Errorf("nhi: decoding Patient response: %w", err)
	}
	return &patient, nil
}

// MatchPatient performs a FHIR $match operation, returning all candidate
// Patient resources that match the supplied demographic parameters.
func (c *Client) MatchPatient(ctx context.Context, params MatchParams) ([]*fhirr4.Patient, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("nhi: failed to obtain access token: %w", err)
	}

	// Build a FHIR Parameters resource for $match.
	body := buildMatchParameters(params)
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("nhi: marshaling $match parameters: %w", err)
	}

	url := fmt.Sprintf("%s/Patient/$match", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("nhi: building $match request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/fhir+json")
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nhi: POST Patient/$match: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nhi: POST Patient/$match returned HTTP %d", resp.StatusCode)
	}

	// The response is a FHIR Bundle of type "searchset".
	var bundle struct {
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			Resource fhirr4.Patient `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("nhi: decoding $match bundle response: %w", err)
	}

	patients := make([]*fhirr4.Patient, 0, len(bundle.Entry))
	for i := range bundle.Entry {
		p := bundle.Entry[i].Resource
		patients = append(patients, &p)
	}
	return patients, nil
}

// buildMatchParameters constructs a FHIR Parameters resource for a $match call.
func buildMatchParameters(p MatchParams) map[string]any {
	humanName := map[string]any{
		"family": p.FamilyName,
		"given":  []string{p.GivenName},
	}
	patient := map[string]any{
		"resourceType": "Patient",
		"name":         []any{humanName},
		"birthDate":    p.BirthDate.Format("2006-01-02"),
		"gender":       p.Gender,
	}
	if p.Address != "" {
		patient["address"] = []any{map[string]any{"text": p.Address}}
	}

	return map[string]any{
		"resourceType": "Parameters",
		"parameter": []any{
			map[string]any{
				"name":     "resource",
				"resource": patient,
			},
			map[string]any{
				"name":       "onlyCertainMatches",
				"valueBoolean": false,
			},
		},
	}
}
