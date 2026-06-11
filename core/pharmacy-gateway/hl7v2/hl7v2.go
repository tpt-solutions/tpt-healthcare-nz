// Package hl7v2 provides an HL7 v2 RDE^O11 (Pharmacy/Treatment Encoded Order)
// fallback connector for legacy pharmacy PMS systems that do not expose a
// FHIR endpoint. Messages are delivered via MLLP over TCP to the pharmacy's
// HL7 listener, using the existing core/hl7 MLLP transport.
package hl7v2

import (
	"context"
	"fmt"
	"strings"
	"time"

	gateway "github.com/PhillipC05/tpt-healthcare/core/pharmacy-gateway"
	"github.com/PhillipC05/tpt-healthcare/core/hl7"
)

// Connector implements gateway.Connector using HL7 v2 RDE^O11 over MLLP.
type Connector struct {
	mllpClient *hl7.MLLPClient
	sendingApp string // MSH-3: sending application ID
	facilityID string // MSH-4: sending facility HPI
}

// New constructs an HL7 v2 connector.
// mllpAddr is the "host:port" of the pharmacy's HL7 MLLP listener.
// sendingApp is the sending application ID (MSH-3); facilityID is the practice
// HPI facility identifier (MSH-4).
func New(mllpAddr, sendingApp, facilityID string) (*Connector, error) {
	client, err := hl7.NewMLLPClient(mllpAddr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("hl7v2: connect to %s: %w", mllpAddr, err)
	}
	return &Connector{
		mllpClient: client,
		sendingApp: sendingApp,
		facilityID: facilityID,
	}, nil
}

// Type returns the connector identifier.
func (c *Connector) Type() gateway.ConnectorType { return gateway.ConnectorHL7v2 }

// Dispatch encodes the DispatchRequest as an HL7 v2 RDE^O11 message and sends
// it via MLLP. Returns a DispatchResult with Status=DispensePending; the
// pharmacy system will send an ACK but does not return a synchronous status.
func (c *Connector) Dispatch(ctx context.Context, req gateway.DispatchRequest) (*gateway.DispatchResult, error) {
	msg := c.buildRDEO11(req)

	if err := c.mllpClient.Send(ctx, msg); err != nil {
		return nil, fmt.Errorf("hl7v2: send RDE^O11: %w", err)
	}

	return &gateway.DispatchResult{
		ID:           req.ID,
		Connector:    gateway.ConnectorHL7v2,
		ExternalID:   req.MedicationRequestID,
		Status:       gateway.DispensePending,
		DispatchedAt: time.Now().UTC(),
		Message:      "sent via HL7 v2 MLLP",
	}, nil
}

// GetStatus is not available for HL7 v2 connectors — legacy PMS systems do not
// provide a query interface. Returns DispensePending and an informational error.
func (c *Connector) GetStatus(_ context.Context, externalID string) (gateway.DispenseStatus, error) {
	return gateway.DispensePending, fmt.Errorf("hl7v2: status queries not supported for HL7 v2 connector (id=%s)", externalID)
}

// Cancel sends an HL7 v2 ORC-1=CA (Cancel Order) RDE^O11 for the prescription.
func (c *Connector) Cancel(ctx context.Context, externalID string) error {
	msg := c.buildCancelRDE(externalID)
	if err := c.mllpClient.Send(ctx, msg); err != nil {
		return fmt.Errorf("hl7v2: send cancel RDE^O11: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// HL7 v2 message builders
// ---------------------------------------------------------------------------

// buildRDEO11 constructs a minimal HL7 v2 RDE^O11 (Pharmacy Encoded Order) message.
// The MSH, PID, ORC, RXE, and RXR segments are required by the NZ HL7 v2 pharmacy profile.
func (c *Connector) buildRDEO11(req gateway.DispatchRequest) string {
	now := time.Now().UTC().Format("20060102150405")
	msgID := req.ID.String()[:8] // HL7 msg control ID limited to 20 chars; 8 chars of UUID is sufficient

	seg := func(fields ...string) string {
		return strings.Join(fields, "|") + "\r"
	}

	var sb strings.Builder

	// MSH — Message Header
	sb.WriteString(seg("MSH", "^~\\&",
		c.sendingApp, c.facilityID,
		req.PharmacyHPI, req.PharmacyHPI,
		now, "",
		"RDE^O11^RDE_O11",
		msgID,
		"P", "2.5.1",
	))

	// PID — Patient Identification (NHI as patient identifier)
	sb.WriteString(seg("PID", "1", "",
		fmt.Sprintf("^^^NHI^NI~%s^^^&https://standards.digital.health.nz/ns/nhi-id&ISO^NI", strings.ToUpper(req.PatientNHI)),
	))

	// ORC — Common Order (NW = New Order)
	sb.WriteString(seg("ORC", "NW",
		req.MedicationRequestID, "", "", "", "", "",
		now, "",
		fmt.Sprintf("%s^^^&https://standards.digital.health.nz/ns/hpi-person-id&ISO^L", req.PrescriberHPI),
	))

	// RXE — Pharmacy/Treatment Encoded Order
	sb.WriteString(seg("RXE", "",
		fmt.Sprintf("%s^%s^^^NZMT", req.NZULM, req.NZULM),
		"", "", "",
		fmt.Sprintf("%s^^NZMT", req.NZULM),
		req.Dose, "", "", "",
		fmt.Sprintf("%d", req.Quantity), "",
		fmt.Sprintf("%d", req.Repeats),
	))

	// RXR — Pharmacy/Treatment Route
	sb.WriteString(seg("RXR",
		fmt.Sprintf("^%s", req.Route),
	))

	return sb.String()
}

// buildCancelRDE constructs an RDE^O11 with ORC-1=CA to cancel an existing order.
func (c *Connector) buildCancelRDE(externalID string) string {
	now := time.Now().UTC().Format("20060102150405")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("MSH|^~\\&|%s|%s|PHARMACY|PHARMACY|%s||RDE^O11^RDE_O11|%s|P|2.5.1\r",
		c.sendingApp, c.facilityID, now, now[:8]+"CA"))
	sb.WriteString(fmt.Sprintf("ORC|CA|%s\r", externalID))
	return sb.String()
}
