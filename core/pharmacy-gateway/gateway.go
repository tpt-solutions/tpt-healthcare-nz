// Package pharmacygateway provides a FHIR MedicationRequest dispatch layer for
// routing electronic prescriptions to community pharmacy PMS systems. Supported
// connectors: Fred Dispense (FHIR REST), Toniq (FHIR REST), and an HL7 v2
// RDE^O11 fallback for legacy systems without a FHIR endpoint.
package pharmacygateway

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DispenseStatus reflects the state of a dispatched prescription at the pharmacy.
type DispenseStatus string

const (
	DispensePending    DispenseStatus = "pending"
	DispenseReceived   DispenseStatus = "received"
	DispensedPartial   DispenseStatus = "partial"
	DispensedComplete  DispenseStatus = "complete"
	DispenseCancelled  DispenseStatus = "cancelled"
	DispensedError     DispenseStatus = "error"
)

// ConnectorType identifies the backend PMS connector used for dispatch.
type ConnectorType string

const (
	ConnectorFred   ConnectorType = "fred"
	ConnectorToniq  ConnectorType = "toniq"
	ConnectorHL7v2  ConnectorType = "hl7v2"
)

// DispatchRequest is the normalised e-prescription dispatch payload built
// from a FHIR R5 MedicationRequest resource.
type DispatchRequest struct {
	// ID is the internal UUID for this dispatch event.
	ID uuid.UUID `json:"id"`
	// MedicationRequestID is the FHIR MedicationRequest resource ID.
	MedicationRequestID string `json:"medicationRequestId"`
	// PatientNHI is the patient's NHI.
	PatientNHI string `json:"patientNhi"`
	// PrescriberHPI is the HPI CPN of the prescribing practitioner.
	PrescriberHPI string `json:"prescriberHpi"`
	// PharmacyHPI is the HPI facility number of the destination pharmacy.
	PharmacyHPI string `json:"pharmacyHpi"`
	// NZULM is the New Zealand Universal List of Medicines identifier.
	NZULM string `json:"nzulm"`
	// BrandName is the optional preferred brand (nil = generic substitution allowed).
	BrandName string `json:"brandName,omitempty"`
	// Dose is the prescribed dose and unit (e.g. "500 mg").
	Dose string `json:"dose"`
	// Route is the administration route (e.g. "oral").
	Route string `json:"route"`
	// Frequency is the dosing frequency (e.g. "twice daily").
	Frequency string `json:"frequency"`
	// Quantity is the total quantity to be dispensed (number of units or packs).
	Quantity int `json:"quantity"`
	// Repeats is the number of authorised repeats (0 = single dispense only).
	Repeats int `json:"repeats"`
	// Instructions contains any additional dispensing instructions.
	Instructions string `json:"instructions,omitempty"`
	// IsUrgent flags the prescription as urgent, prompting same-day dispensing.
	IsUrgent bool `json:"isUrgent,omitempty"`
}

// DispatchResult is returned after successfully forwarding a prescription.
type DispatchResult struct {
	// ID matches the DispatchRequest.ID.
	ID uuid.UUID `json:"id"`
	// Connector is the backend used for this dispatch.
	Connector ConnectorType `json:"connector"`
	// ExternalID is the pharmacy PMS reference number (if returned synchronously).
	ExternalID string `json:"externalId,omitempty"`
	// Status is the initial dispense status as reported by the pharmacy PMS.
	Status DispenseStatus `json:"status"`
	// DispatchedAt is the timestamp of the successful dispatch.
	DispatchedAt time.Time `json:"dispatchedAt"`
	// Message is an informational message from the PMS (e.g. "queued for dispense").
	Message string `json:"message,omitempty"`
}

// Connector is the interface that all pharmacy PMS backends must implement.
type Connector interface {
	// Dispatch sends a prescription to the pharmacy PMS.
	Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error)
	// GetStatus retrieves the current dispense status for an external PMS reference.
	GetStatus(ctx context.Context, externalID string) (DispenseStatus, error)
	// Cancel requests cancellation of a previously dispatched prescription.
	Cancel(ctx context.Context, externalID string) error
	// Type returns the connector type identifier.
	Type() ConnectorType
}

// Gateway routes DispatchRequests to registered pharmacy PMS connectors.
// The routing decision is made by PharmacyHPI: if a pharmacy is registered
// with a specific connector, that connector is used; otherwise the gateway
// falls back to the HL7 v2 connector.
type Gateway struct {
	connectors map[string]Connector // keyed by pharmacy HPI
	fallback   Connector            // HL7 v2 fallback for unregistered pharmacies
}

// New constructs a Gateway with a mandatory HL7 v2 fallback connector.
func New(fallback Connector) *Gateway {
	return &Gateway{
		connectors: make(map[string]Connector),
		fallback:   fallback,
	}
}

// Register maps a pharmacy HPI to a specific connector. Pharmacies in the
// Fred Dispense or Toniq network should be pre-registered here.
func (g *Gateway) Register(pharmacyHPI string, connector Connector) {
	g.connectors[pharmacyHPI] = connector
}

// Dispatch routes a prescription to the appropriate connector. It selects the
// registered connector for the pharmacy, or falls back to HL7 v2.
func (g *Gateway) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}

	connector := g.connectors[req.PharmacyHPI]
	if connector == nil {
		connector = g.fallback
	}
	if connector == nil {
		return nil, fmt.Errorf("pharmacy-gateway: no connector registered for pharmacy %s and no fallback configured", req.PharmacyHPI)
	}

	result, err := connector.Dispatch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("pharmacy-gateway: dispatch via %s: %w", connector.Type(), err)
	}
	return result, nil
}

// GetStatus returns the current dispense status from whichever connector handled
// the original dispatch, identified by connectorType + externalID.
func (g *Gateway) GetStatus(ctx context.Context, pharmacyHPI, externalID string) (DispenseStatus, error) {
	connector := g.connectors[pharmacyHPI]
	if connector == nil {
		connector = g.fallback
	}
	if connector == nil {
		return "", fmt.Errorf("pharmacy-gateway: no connector for pharmacy %s", pharmacyHPI)
	}
	return connector.GetStatus(ctx, externalID)
}
