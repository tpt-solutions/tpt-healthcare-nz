// Package acc — purchase_order.go implements the ACC purchase order (PO)
// lifecycle: requesting approval, tracking session consumption, and generating
// reconciliation reports for allied-health treatment providers.
package acc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// POStatus represents the lifecycle state of an ACC purchase order.
type POStatus string

const (
	POPending  POStatus = "pending"
	POApproved POStatus = "approved"
	PODeclined POStatus = "declined"
	POExhausted POStatus = "exhausted" // all funded sessions consumed
	POExpired  POStatus = "expired"
)

// PurchaseOrder represents an ACC purchase order authorising a fixed number
// of treatment sessions for a specific patient, claim, and discipline.
type PurchaseOrder struct {
	// ID is the internal UUID for this purchase order record.
	ID uuid.UUID `json:"id"`
	// PONumber is the ACC-assigned purchase order reference.
	PONumber string `json:"poNumber,omitempty"`
	// ClaimNumber is the associated ACC claim number.
	ClaimNumber string `json:"claimNumber"`
	// PatientNHI is the patient's NHI number.
	PatientNHI string `json:"patientNhi"`
	// ProviderHPI is the HPI CPN of the treating provider.
	ProviderHPI string `json:"providerHpi"`
	// Discipline is the treatment type this PO covers.
	Discipline Discipline `json:"discipline"`
	// SessionsApproved is the total number of sessions authorised.
	SessionsApproved int `json:"sessionsApproved"`
	// SessionsUsed is the number of sessions that have been delivered and claimed.
	SessionsUsed int `json:"sessionsUsed"`
	// Status is the current PO lifecycle state.
	Status POStatus `json:"status"`
	// RequestedAt is when the PO request was submitted to ACC.
	RequestedAt time.Time `json:"requestedAt"`
	// ApprovedAt is when ACC approved the PO (zero if not yet approved).
	ApprovedAt *time.Time `json:"approvedAt,omitempty"`
	// ExpiresAt is the date after which unused sessions lapse.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// UpdatedAt tracks the last state change.
	UpdatedAt time.Time `json:"updatedAt"`
}

// SessionsRemaining returns the number of sessions still available on the PO.
func (po *PurchaseOrder) SessionsRemaining() int {
	r := po.SessionsApproved - po.SessionsUsed
	if r < 0 {
		return 0
	}
	return r
}

// PORequest is the payload sent to ACC when requesting a new purchase order.
type PORequest struct {
	ClaimNumber     string     `json:"claimNumber"`
	PatientNHI      string     `json:"patientNhi"`
	ProviderHPI     string     `json:"providerHpi"`
	Discipline      Discipline `json:"discipline"`
	SessionsRequested int      `json:"sessionsRequested"`
	ClinicalJustification string `json:"clinicalJustification,omitempty"`
}

// ReconciliationLine is a single line in a PO reconciliation report.
type ReconciliationLine struct {
	PONumber        string        `json:"poNumber"`
	ClaimNumber     string        `json:"claimNumber"`
	PatientNHI      string        `json:"patientNhi"`
	Discipline      Discipline    `json:"discipline"`
	SessionsApproved int          `json:"sessionsApproved"`
	SessionsUsed    int           `json:"sessionsUsed"`
	SessionsRemaining int         `json:"sessionsRemaining"`
	Status          POStatus      `json:"status"`
	ExpiresAt       *time.Time    `json:"expiresAt,omitempty"`
}

// RequestPO submits a purchase order request to ACC for a given claim and
// discipline. The returned PurchaseOrder will have Status=POPending until
// ACC processes the request.
func (c *Client) RequestPO(ctx context.Context, req PORequest) (*PurchaseOrder, error) {
	if req.SessionsRequested <= 0 {
		return nil, fmt.Errorf("acc: sessionsRequested must be > 0")
	}

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("acc: obtaining token for PO request: %w", err)
	}

	fhirTask := buildPORequestTask(req)
	payload, err := json.Marshal(fhirTask)
	if err != nil {
		return nil, fmt.Errorf("acc: marshal PO request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/Task", c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("acc: build PO request: %w", err)
	}
	setHeaders(httpReq, token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("acc: POST Task (PO request): %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp, req.ClaimNumber)
	}

	now := time.Now().UTC()
	po := &PurchaseOrder{
		ID:                uuid.New(),
		ClaimNumber:       req.ClaimNumber,
		PatientNHI:        req.PatientNHI,
		ProviderHPI:       req.ProviderHPI,
		Discipline:        req.Discipline,
		SessionsApproved:  0,
		SessionsUsed:      0,
		Status:            POPending,
		RequestedAt:       now,
		UpdatedAt:         now,
	}

	// Parse the ACC response to extract the PO number if synchronously assigned.
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil {
		if ids, ok := raw["identifier"].([]any); ok {
			for _, idRaw := range ids {
				idMap, ok := idRaw.(map[string]any)
				if !ok {
					continue
				}
				sys, _ := idMap["system"].(string)
				val, _ := idMap["value"].(string)
				if sys == "https://acc.govt.nz/ns/purchase-order" && val != "" {
					po.PONumber = val
				}
			}
		}
		if sessApproved, ok := raw["sessionsApproved"].(float64); ok {
			po.SessionsApproved = int(sessApproved)
			if po.SessionsApproved > 0 {
				po.Status = POApproved
				t := time.Now().UTC()
				po.ApprovedAt = &t
			}
		}
	}

	return po, nil
}

// ConsumeSession records one delivered session against a purchase order.
// It decrements the remaining session count and updates the PO status to
// POExhausted when the last session is consumed.
func (c *Client) ConsumeSession(ctx context.Context, po *PurchaseOrder) (*PurchaseOrder, error) {
	if po.Status != POApproved {
		return nil, fmt.Errorf("acc: cannot consume session: PO %s is %s", po.PONumber, po.Status)
	}
	if po.SessionsRemaining() == 0 {
		po.Status = POExhausted
		po.UpdatedAt = time.Now().UTC()
		return po, nil
	}

	token, err := c.tokenFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("acc: obtaining token for session consumption: %w", err)
	}

	payload, err := json.Marshal(map[string]any{
		"resourceType": "Task",
		"status":       "completed",
		"code": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "https://acc.govt.nz/ns/task-type",
					"code":   "consume-session",
				},
			},
		},
		"focus": map[string]any{
			"identifier": map[string]any{
				"system": "https://acc.govt.nz/ns/purchase-order",
				"value":  po.PONumber,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("acc: marshal ConsumeSession task: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/Task", c.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("acc: build ConsumeSession request: %w", err)
	}
	setHeaders(req, token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("acc: POST ConsumeSession: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp, po.ClaimNumber)
	}

	updated := *po
	updated.SessionsUsed++
	updated.UpdatedAt = time.Now().UTC()
	if updated.SessionsRemaining() == 0 {
		updated.Status = POExhausted
	}
	return &updated, nil
}

// ReconcilePOs generates a reconciliation report for the given purchase orders.
// It is typically called at end-of-month to verify claimed sessions match the
// ACC-held record before submitting the provider invoice.
func ReconcilePOs(pos []PurchaseOrder) []ReconciliationLine {
	lines := make([]ReconciliationLine, 0, len(pos))
	for _, po := range pos {
		lines = append(lines, ReconciliationLine{
			PONumber:          po.PONumber,
			ClaimNumber:       po.ClaimNumber,
			PatientNHI:        po.PatientNHI,
			Discipline:        po.Discipline,
			SessionsApproved:  po.SessionsApproved,
			SessionsUsed:      po.SessionsUsed,
			SessionsRemaining: po.SessionsRemaining(),
			Status:            po.Status,
			ExpiresAt:         po.ExpiresAt,
		})
	}
	return lines
}

// buildPORequestTask constructs a FHIR Task resource for a PO request.
func buildPORequestTask(req PORequest) map[string]any {
	codes, _ := CodesFor(req.Discipline)
	return map[string]any{
		"resourceType": "Task",
		"status":       "requested",
		"intent":       "order",
		"code": map[string]any{
			"coding": []any{
				map[string]any{
					"system": "https://acc.govt.nz/ns/task-type",
					"code":   "purchase-order-request",
				},
			},
		},
		"for": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/nhi-id",
				"value":  req.PatientNHI,
			},
		},
		"owner": map[string]any{
			"identifier": map[string]any{
				"system": "https://standards.digital.health.nz/ns/hpi-person-id",
				"value":  req.ProviderHPI,
			},
		},
		"reasonReference": map[string]any{
			"identifier": map[string]any{
				"system": "https://acc.govt.nz/ns/claim-number",
				"value":  req.ClaimNumber,
			},
		},
		"input": []any{
			map[string]any{
				"type": map[string]any{"text": "discipline"},
				"valueString": string(req.Discipline),
			},
			map[string]any{
				"type": map[string]any{"text": "procedureCode"},
				"valueString": string(codes[0]),
			},
			map[string]any{
				"type": map[string]any{"text": "sessionsRequested"},
				"valueInteger": req.SessionsRequested,
			},
			map[string]any{
				"type": map[string]any{"text": "clinicalJustification"},
				"valueString": req.ClinicalJustification,
			},
		},
	}
}
