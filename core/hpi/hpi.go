// Package hpi provides a client for the Health Practitioner Index (HPI),
// Te Whatu Ora's authoritative register of health practitioners and facilities
// in New Zealand. Practitioner lookups are cached in Redis for 24 hours to
// reduce latency and load on the upstream FHIR endpoint.
package hpi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// practitionerCacheTTL is the Redis TTL for cached Practitioner records.
	practitionerCacheTTL = 24 * time.Hour
	// practitionerCacheKeyPrefix is prepended to each CPN to form the Redis key.
	practitionerCacheKeyPrefix = "hpi:prac:"
)

// Practitioner represents a registered health practitioner in the HPI.
type Practitioner struct {
	// CPNID is the HPI Common Person Number — the unique identifier for the
	// practitioner across all Te Whatu Ora systems.
	CPNID string `json:"cpnId"`
	// Name is the practitioner's full registered name.
	Name string `json:"name"`
	// RegisteredBody is the regulatory authority (e.g. "Medical Council of NZ").
	RegisteredBody string `json:"registeredBody"`
	// Scope is the practitioner's scope of practice (e.g. "General Practice").
	Scope string `json:"scope"`
	// APC indicates whether the practitioner currently holds an Annual
	// Practising Certificate.
	APC bool `json:"apc"`
	// APCExpiry is the expiry date of the current APC, if available.
	APCExpiry *time.Time `json:"apcExpiry,omitempty"`
	// Facilities lists the HPI facility IDs the practitioner is associated with.
	Facilities []string `json:"facilities,omitempty"`
}

// Client is a HPI FHIR API client with Redis-backed result caching.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	tokenFunc   func(ctx context.Context) (string, error)
	redisClient *redis.Client
}

// New constructs a Client targeting baseURL, using tokenFunc for SMART on FHIR
// authentication and rdb for caching. rdb may be nil to disable caching.
func New(baseURL string, tokenFunc func(ctx context.Context) (string, error), rdb *redis.Client) *Client {
	return &Client{
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		baseURL:     strings.TrimRight(baseURL, "/"),
		tokenFunc:   tokenFunc,
		redisClient: rdb,
	}
}

// NewClient is a compatibility constructor preserved for module compatibility.
// It returns a Client with no base URL and no caching; wire the real dependencies at deployment.
func NewClient(_ string, _ any) *Client {
	return New("", nil, nil)
}

// GetPractitioner returns the Practitioner record for the given HPI Common
// Person Number (CPN). Results are served from Redis when available (TTL 24h).
// A cache miss fetches from the HPI FHIR API and repopulates the cache.
func (c *Client) GetPractitioner(ctx context.Context, cpn string) (*Practitioner, error) {
	cpn = strings.ToUpper(strings.TrimSpace(cpn))

	// 1. Attempt cache read.
	if c.redisClient != nil {
		cacheKey := practitionerCacheKeyPrefix + cpn
		cached, err := c.redisClient.Get(ctx, cacheKey).Bytes()
		if err == nil {
			var p Practitioner
			if jsonErr := json.Unmarshal(cached, &p); jsonErr == nil {
				return &p, nil
			}
		}
		// redis.Nil means key not found — proceed to live fetch.
		// Any other Redis error is non-fatal; we fall through to the API.
	}

	// 2. Fetch from HPI FHIR API.
	p, err := c.fetchPractitioner(ctx, cpn)
	if err != nil {
		return nil, err
	}

	// 3. Populate cache.
	if c.redisClient != nil {
		if data, jsonErr := json.Marshal(p); jsonErr == nil {
			cacheKey := practitionerCacheKeyPrefix + cpn
			// Best-effort cache write; ignore errors to remain non-blocking.
			_ = c.redisClient.Set(ctx, cacheKey, data, practitionerCacheTTL).Err()
		}
	}

	return p, nil
}

// APCStatus is the result of an APC validation check.
type APCStatus struct {
	Valid  bool       `json:"valid"`
	CPN    string     `json:"cpn,omitempty"`
	Expiry *time.Time `json:"expiry,omitempty"`
	Scope  string     `json:"scope,omitempty"`
}

// ValidateAPC reports whether the practitioner identified by cpn holds a
// current (non-expired) Annual Practising Certificate.
func (c *Client) ValidateAPC(ctx context.Context, cpn string) (APCStatus, error) {
	p, err := c.GetPractitioner(ctx, cpn)
	if err != nil {
		return APCStatus{}, fmt.Errorf("hpi: ValidateAPC for CPN %s: %w", cpn, err)
	}
	if !p.APC {
		return APCStatus{CPN: cpn, Scope: p.Scope}, nil
	}
	if p.APCExpiry != nil && time.Now().After(*p.APCExpiry) {
		return APCStatus{CPN: cpn, Expiry: p.APCExpiry, Scope: p.Scope}, nil
	}
	return APCStatus{Valid: true, CPN: cpn, Expiry: p.APCExpiry, Scope: p.Scope}, nil
}

// GetFacility fetches the FHIR Organization resource for the given HPI facility
// ID. The result is returned as a generic map matching the FHIR JSON structure.
func (c *Client) GetFacility(ctx context.Context, hpiFacID string) (map[string]any, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("hpi: obtaining access token: %w", err)
	}

	url := fmt.Sprintf("%s/Organization?identifier=https://standards.digital.health.nz/ns/hpi-facility-id|%s",
		c.baseURL, hpiFacID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("hpi: building GetFacility request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hpi: GET Organization: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("hpi: facility %s not found", hpiFacID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hpi: GetFacility returned HTTP %d", resp.StatusCode)
	}

	// Response is a FHIR Bundle; extract the first Organization entry.
	var bundle struct {
		Entry []struct {
			Resource map[string]any `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("hpi: decoding GetFacility bundle: %w", err)
	}
	if len(bundle.Entry) == 0 {
		return nil, fmt.Errorf("hpi: no Organization found for facility %s", hpiFacID)
	}
	return bundle.Entry[0].Resource, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// fetchPractitioner calls the HPI FHIR API for a Practitioner resource
// identified by cpn and maps the FHIR response to our Practitioner struct.
func (c *Client) fetchPractitioner(ctx context.Context, cpn string) (*Practitioner, error) {
	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("hpi: obtaining access token: %w", err)
	}

	// Query by identifier (HPI CPN system).
	url := fmt.Sprintf("%s/Practitioner?identifier=https://standards.digital.health.nz/ns/hpi-person-id|%s", c.baseURL, cpn)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("hpi: building Practitioner request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/fhir+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("hpi: GET Practitioner: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("hpi: practitioner %s not found", cpn)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hpi: GET Practitioner returned HTTP %d", resp.StatusCode)
	}

	var bundle struct {
		Entry []struct {
			Resource fhirPractitioner `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return nil, fmt.Errorf("hpi: decoding Practitioner bundle: %w", err)
	}
	if len(bundle.Entry) == 0 {
		return nil, fmt.Errorf("hpi: practitioner %s not found", cpn)
	}

	return mapFHIRPractitioner(cpn, bundle.Entry[0].Resource), nil
}

// fhirPractitioner is a minimal FHIR R4 Practitioner projection used only
// for decoding the HPI API response. It covers the fields we care about.
type fhirPractitioner struct {
	Name []struct {
		Text   string   `json:"text"`
		Family string   `json:"family"`
		Given  []string `json:"given"`
	} `json:"name"`
	Qualification []struct {
		Code struct {
			Text   string `json:"text"`
			Coding []struct {
				Display string `json:"display"`
			} `json:"coding"`
		} `json:"code"`
		Issuer struct {
			Display string `json:"display"`
		} `json:"issuer"`
		Period struct {
			End string `json:"end"`
		} `json:"period"`
		Extension []struct {
			URL         string `json:"url"`
			ValueString string `json:"valueString,omitempty"`
		} `json:"extension"`
	} `json:"qualification"`
	Extension []struct {
		URL          string `json:"url"`
		ValueBoolean *bool  `json:"valueBoolean,omitempty"`
		ValueString  string `json:"valueString,omitempty"`
	} `json:"extension"`
}

// mapFHIRPractitioner converts a fhirPractitioner to our domain Practitioner.
func mapFHIRPractitioner(cpn string, fp fhirPractitioner) *Practitioner {
	p := &Practitioner{CPNID: cpn}

	// Extract display name.
	for _, n := range fp.Name {
		if n.Text != "" {
			p.Name = n.Text
			break
		}
		if n.Family != "" {
			parts := append(n.Given, n.Family)
			p.Name = strings.Join(parts, " ")
			break
		}
	}

	// Extract qualifications — look for APC and scope.
	for _, q := range fp.Qualification {
		text := q.Code.Text
		if len(q.Code.Coding) > 0 && text == "" {
			text = q.Code.Coding[0].Display
		}

		// Treat any qualification from a recognised NZ regulatory body as APC.
		issuer := q.Issuer.Display
		if strings.Contains(strings.ToLower(text), "annual practising") ||
			strings.Contains(strings.ToLower(text), "apc") {
			p.APC = true
			p.RegisteredBody = issuer
			if q.Period.End != "" {
				t, err := time.Parse("2006-01-02", q.Period.End)
				if err == nil {
					p.APCExpiry = &t
				}
			}
		} else if p.Scope == "" {
			p.Scope = text
			if p.RegisteredBody == "" {
				p.RegisteredBody = issuer
			}
		}
	}

	// Extract facility associations from extensions.
	for _, ext := range fp.Extension {
		if strings.Contains(ext.URL, "facility") && ext.ValueString != "" {
			p.Facilities = append(p.Facilities, ext.ValueString)
		}
	}

	return p
}
