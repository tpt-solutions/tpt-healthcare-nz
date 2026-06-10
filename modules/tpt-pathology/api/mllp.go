package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
	"github.com/PhillipC05/tpt-healthcare/core/hl7"
	"github.com/PhillipC05/tpt-healthcare/core/subscription"
	"github.com/google/uuid"
)

// mllpConverter receives ORU^R01 HL7 v2 messages from the MLLP listener and
// converts them to FHIR R5 DiagnosticReport + Observation resources, persists
// them, and publishes a FHIR subscription notification.
type mllpConverter struct {
	pool       db.Pool
	enc        *encryption.Cipher
	subEngine  *subscription.Engine
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// handleMessage is the hl7.MLLPServer handler. It returns nil to accept the
// auto-generated AA ACK, or an error to trigger an AE ACK.
func (c *mllpConverter) handleMessage(msg *hl7.Message) (*hl7.Message, error) {
	if msg.Type != "ORU^R01" {
		// Accept but ignore message types we do not process.
		c.logger.Info("mllp: ignoring non-ORU message", slog.String("type", msg.Type))
		return nil, nil
	}

	ctx := context.Background()

	report, observations, err := c.convertORU(msg)
	if err != nil {
		c.logger.Error("mllp: convert ORU^R01", slog.Any("error", err))
		return nil, fmt.Errorf("convert ORU^R01: %w", err)
	}

	// We need a tenantID to scope the records. For MLLP messages arriving
	// without an HTTP tenant header we derive the tenant from the ZNZL
	// LabSite field (mapped at onboarding). Unknown senders are written to a
	// shared "default" tenant; production deployments should configure the
	// sending lab → tenant mapping.
	tenantID := c.resolveTenant(msg)

	specimenID, err := c.insertSpecimen(ctx, msg, report, tenantID)
	if err != nil {
		c.logger.Error("mllp: insert specimen", slog.Any("error", err))
		return nil, fmt.Errorf("insert specimen: %w", err)
	}

	reportID, err := c.insertReport(ctx, report, observations, specimenID, tenantID)
	if err != nil {
		c.logger.Error("mllp: insert diagnostic report", slog.Any("error", err))
		return nil, fmt.Errorf("insert diagnostic report: %w", err)
	}

	// Publish FHIR subscription event so tpt-doctor and patient portals are
	// notified. Failure here does not prevent the ACK — the notification can
	// be retried from the notification_sent flag.
	if pubErr := c.publishNotification(ctx, report, reportID, tenantID); pubErr != nil {
		c.logger.Error("mllp: publish subscription", slog.Any("error", pubErr))
	}

	c.logger.Info("mllp: ORU^R01 processed",
		slog.String("reportID", reportID),
		slog.String("accession", report.accessionNumber),
		slog.Int("observations", len(observations)),
	)

	return nil, nil // nil → auto AA ACK
}

// convertedReport is an intermediate representation built from the ORU^R01
// segments before database persistence.
type convertedReport struct {
	patientNHI      string
	patientID       string // FHIR Patient resource ID (if known)
	accessionNumber string
	orderingHPI     string
	performingLab   string
	status          string // FHIR DiagnosticReport status
	loincCode       string
	loincDisplay    string
	effectiveAt     *time.Time
	issuedAt        *time.Time
	znzl            map[string]string
}

// convertORU parses an ORU^R01 message and produces a FHIR DiagnosticReport
// and a slice of Observations. The FHIR resources hold all result details;
// the convertedReport carries metadata needed for the database columns.
func (c *mllpConverter) convertORU(msg *hl7.Message) (convertedReport, []r5.Observation, error) {
	var rep convertedReport

	// --- MSH ----------------------------------------------------------------
	rep.performingLab = msg.GetField("MSH", "4") // MSH-4: Sending Facility

	// --- PID ----------------------------------------------------------------
	// PID-3: Patient Identifier List — first component is the ID, system may
	// indicate NHI. Try common NHI positions: PID-3 component 1 or ZNZL-12.
	rep.patientNHI = extractNHI(msg)

	// --- ZNZL (NZ Lab Z-segment) --------------------------------------------
	rep.znzl = extractZNZL(msg)
	if nhi, ok := rep.znzl["NHINumber"]; ok && nhi != "" {
		rep.patientNHI = nhi
	}
	if acc, ok := rep.znzl["LabOrderNumber"]; ok && acc != "" {
		rep.accessionNumber = acc
	}

	// --- OBR ----------------------------------------------------------------
	obr, ok := msg.GetSegment("OBR")
	if !ok {
		return rep, nil, fmt.Errorf("OBR segment missing")
	}

	if rep.accessionNumber == "" {
		rep.accessionNumber = firstComponent(obr, "3") // OBR-3: Filler Order Number
	}

	// OBR-4: Universal Service Identifier (LOINC code^display)
	rep.loincCode = firstComponent(obr, "4")
	rep.loincDisplay = secondComponent(obr, "4")

	// OBR-7: Observation Date/Time
	if t := parseHL7DateTime(firstComponent(obr, "7")); t != nil {
		rep.effectiveAt = t
	}

	// OBR-16: Ordering Provider (HPI CPN)
	rep.orderingHPI = firstComponent(obr, "16")

	// OBR-25: Result Status (F=Final, P=Preliminary, C=Corrected)
	rep.status = mapOBRStatus(firstComponent(obr, "25"))
	if rep.status == "" {
		rep.status = "registered"
	}

	now := time.Now().UTC()
	rep.issuedAt = &now

	// --- OBX segments → Observations ----------------------------------------
	obxSegs := msg.GetAllSegments("OBX")
	observations := make([]r5.Observation, 0, len(obxSegs))

	for _, obx := range obxSegs {
		obs := c.buildObservation(obx, rep)
		observations = append(observations, obs)
	}

	return rep, observations, nil
}

// buildObservation converts a single OBX segment into a FHIR R5 Observation.
func (c *mllpConverter) buildObservation(obx map[string][]string, rep convertedReport) r5.Observation {
	obsID := uuid.New().String()
	field := func(key string) string { return segField(obx, key) }
	field2 := func(key string) string { return segField2(obx, key) }

	// OBX-3: Observation Identifier (LOINC code^display)
	loincCode := field("3")
	loincDisplay := field2("3")

	// OBX-5: Observation Value
	valueType := field("2") // NM, ST, CE, TX, ...
	rawValue := field("5")

	// OBX-6: Units
	unitCode := field("6")
	unitDisplay := field2("6")

	// OBX-7: Reference Range (e.g. "3.5-5.5" or "< 10")
	refRangeText := field("7")

	// OBX-8: Abnormal Flags (H, L, A, N, ...)
	abnFlag := field("8")

	// OBX-11: Result Status
	obsStatus := mapOBXStatus(field("11"))
	if obsStatus == "" {
		obsStatus = rep.status
	}

	// OBX-14: Date/Time of Observation
	var obsEffective *time.Time
	if t := parseHL7DateTime(field("14")); t != nil {
		obsEffective = t
	} else {
		obsEffective = rep.effectiveAt
	}

	obs := r5.Observation{
		ResourceType: "Observation",
		ID:           obsID,
		Status:       obsStatus,
		Category: []r5.CodeableConcept{
			{
				Coding: []r5.Coding{{
					System:  "http://terminology.hl7.org/CodeSystem/observation-category",
					Code:    "laboratory",
					Display: "Laboratory",
				}},
			},
		},
		Code: r5.CodeableConcept{
			Coding: []r5.Coding{{
				System:  "http://loinc.org",
				Code:    loincCode,
				Display: loincDisplay,
			}},
			Text: loincDisplay,
		},
		EffectiveDateTime: obsEffective,
	}

	if rep.patientID != "" {
		obs.Subject = &r5.Reference{Reference: "Patient/" + rep.patientID}
	}

	// Value
	switch valueType {
	case "NM":
		if v, err := strconv.ParseFloat(rawValue, 64); err == nil {
			obs.ValueQuantity = &r5.Quantity{
				Value: v,
				Unit:  unitDisplay,
				Code:  unitCode,
			}
		} else {
			obs.ValueString = rawValue
		}
	case "CE", "CWE":
		// Coded value: code^display^system
		parts := strings.Split(rawValue, "^")
		cc := r5.CodeableConcept{}
		if len(parts) >= 1 {
			coding := r5.Coding{Code: parts[0]}
			if len(parts) >= 2 {
				coding.Display = parts[1]
			}
			if len(parts) >= 3 {
				coding.System = parts[2]
			}
			cc.Coding = []r5.Coding{coding}
			if len(parts) >= 2 {
				cc.Text = parts[1]
			}
		}
		obs.ValueCodeableConcept = &cc
	default:
		obs.ValueString = rawValue
	}

	// Reference range
	if refRangeText != "" {
		rr := r5.ObservationReferenceRange{Text: refRangeText}
		if lo, hi, ok := parseRefRange(refRangeText); ok {
			rr.Low = &r5.Quantity{Value: lo, Unit: unitDisplay, Code: unitCode}
			rr.High = &r5.Quantity{Value: hi, Unit: unitDisplay, Code: unitCode}
		}
		obs.ReferenceRange = []r5.ObservationReferenceRange{rr}
	}

	// Interpretation
	if abnFlag != "" && abnFlag != "N" {
		obs.Interpretation = []r5.CodeableConcept{
			{
				Coding: []r5.Coding{{
					System:  "http://terminology.hl7.org/CodeSystem/v3-ObservationInterpretation",
					Code:    abnFlag,
					Display: interpretationDisplay(abnFlag),
				}},
			},
		}
	}

	return obs
}

// insertSpecimen creates a pathology_specimens row from ZNZL / OBR metadata.
// Returns the new specimen UUID.
func (c *mllpConverter) insertSpecimen(ctx context.Context, msg *hl7.Message, rep convertedReport, tenantID string) (string, error) {
	obr, _ := msg.GetSegment("OBR")
	collectedAt := parseHL7DateTime(segField(obr, "7"))
	specimenType := segField(obr, "15") // OBR-15: Specimen Source

	urgency := rep.znzl["UrgencyIndicator"]
	fundingCode := rep.znzl["FundingCode"]
	collectionSite := rep.znzl["CollectionSite"]
	labOrderNumber := rep.znzl["LabOrderNumber"]

	var id string
	err := c.pool.QueryRow(ctx,
		`INSERT INTO pathology_specimens
		   (tenant_id, patient_nhi, accession_number, collection_site,
		    collected_at, status, specimen_type, ordering_hpi,
		    nzl_lab_order, nzl_funding_code, nzl_urgency)
		 VALUES
		   (@tenant_id, @patient_nhi, @accession_number, @collection_site,
		    @collected_at, 'received', @specimen_type, @ordering_hpi,
		    @nzl_lab_order, @nzl_funding_code, @nzl_urgency)
		 RETURNING id`,
		db.NamedArgs{
			"tenant_id":       tenantID,
			"patient_nhi":     rep.patientNHI,
			"accession_number": rep.accessionNumber,
			"collection_site": collectionSite,
			"collected_at":    collectedAt,
			"specimen_type":   specimenType,
			"ordering_hpi":    rep.orderingHPI,
			"nzl_lab_order":   labOrderNumber,
			"nzl_funding_code": fundingCode,
			"nzl_urgency":     urgency,
		},
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert specimen: %w", err)
	}
	return id, nil
}

// insertReport creates a diagnostic_reports row and persists the encrypted
// FHIR DiagnosticReport JSON. Returns the new report UUID.
func (c *mllpConverter) insertReport(ctx context.Context, rep convertedReport, observations []r5.Observation, specimenID, tenantID string) (string, error) {
	reportID := uuid.New().String()

	// Build FHIR Observation references.
	obsRefs := make([]r5.Reference, len(observations))
	for i, obs := range observations {
		obsRefs[i] = r5.Reference{Reference: "Observation/" + obs.ID}
	}

	fhirReport := r5.DiagnosticReport{
		ResourceType: "DiagnosticReport",
		ID:           reportID,
		Identifier: []r5.Identifier{{
			System: "https://standards.digital.health.nz/ns/lab-accession",
			Value:  rep.accessionNumber,
		}},
		Status: rep.status,
		Category: []r5.CodeableConcept{{
			Coding: []r5.Coding{{
				System:  "http://terminology.hl7.org/CodeSystem/v2-0074",
				Code:    "LAB",
				Display: "Laboratory",
			}},
		}},
		Code: r5.CodeableConcept{
			Coding: []r5.Coding{{
				System:  "http://loinc.org",
				Code:    rep.loincCode,
				Display: rep.loincDisplay,
			}},
			Text: rep.loincDisplay,
		},
		EffectiveDateTime: rep.effectiveAt,
		Issued:            rep.issuedAt,
		Performer: []r5.Reference{{
			Display: rep.performingLab,
		}},
		Result: obsRefs,
	}

	if rep.patientID != "" {
		fhirReport.Subject = &r5.Reference{Reference: "Patient/" + rep.patientID}
	}

	reportJSON, err := json.Marshal(fhirReport)
	if err != nil {
		return "", fmt.Errorf("marshal DiagnosticReport: %w", err)
	}

	var encReport []byte
	if c.enc != nil {
		encReport, err = c.enc.Encrypt(reportJSON)
		if err != nil {
			return "", fmt.Errorf("encrypt DiagnosticReport: %w", err)
		}
	} else {
		encReport = reportJSON
	}

	_, err = c.pool.Exec(ctx,
		`INSERT INTO diagnostic_reports
		   (id, tenant_id, patient_nhi, specimen_id, accession_number,
		    ordering_hpi, performing_lab, status, loinc_code, loinc_display,
		    fhir_report, issued_at, effective_at)
		 VALUES
		   (@id, @tenant_id, @patient_nhi, @specimen_id, @accession_number,
		    @ordering_hpi, @performing_lab, @status, @loinc_code, @loinc_display,
		    @fhir_report, @issued_at, @effective_at)`,
		db.NamedArgs{
			"id":               reportID,
			"tenant_id":        tenantID,
			"patient_nhi":      rep.patientNHI,
			"specimen_id":      specimenID,
			"accession_number": rep.accessionNumber,
			"ordering_hpi":     rep.orderingHPI,
			"performing_lab":   rep.performingLab,
			"status":           rep.status,
			"loinc_code":       rep.loincCode,
			"loinc_display":    rep.loincDisplay,
			"fhir_report":      encReport,
			"issued_at":        rep.issuedAt,
			"effective_at":     rep.effectiveAt,
		},
	)
	if err != nil {
		return "", fmt.Errorf("insert diagnostic report: %w", err)
	}

	// Write audit record (system actor for MLLP-originated writes).
	{
		var tid uuid.UUID
		if u, err := uuid.Parse(tenantID); err == nil {
			tid = u
		}
		_ = c.auditTrail.Record(ctx, audit.Event{
			PrincipalID:  "tpt-pathology-mllp",
			Action:       "create",
			ResourceType: "DiagnosticReport",
			ResourceID:   reportID,
			TenantID:     tid,
			Details: map[string]any{
				"accession":   rep.accessionNumber,
				"loinc_code":  rep.loincCode,
				"patient_nhi": rep.patientNHI,
			},
			OccurredAt: time.Now().UTC(),
		})
	}

	return reportID, nil
}

// publishNotification publishes a FHIR subscription event for the new
// DiagnosticReport. The ordering HPI is included as a filter criterion so
// tpt-doctor subscriptions scoped to that practitioner receive the event.
func (c *mllpConverter) publishNotification(ctx context.Context, rep convertedReport, reportID, tenantID string) error {
	payload := map[string]string{
		"resourceType":    "DiagnosticReport",
		"id":              reportID,
		"accessionNumber": rep.accessionNumber,
		"orderingHPI":     rep.orderingHPI,
		"tenantID":        tenantID,
		"status":          rep.status,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	if err := c.subEngine.Publish(ctx, DiagnosticReportTopic, "DiagnosticReport", reportID, data); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	// Mark notification_sent in the database.
	_, _ = c.pool.Exec(ctx,
		`UPDATE diagnostic_reports SET notification_sent = true WHERE id = @id`,
		db.NamedArgs{"id": reportID},
	)
	return nil
}

// resolveTenant returns a tenantID for MLLP messages. In production, the
// ZNZL LabSite field (or MSH-4) should be mapped to a tenant UUID at
// onboarding. This stub returns the zero UUID as a safe default.
func (c *mllpConverter) resolveTenant(msg *hl7.Message) string {
	// TODO: look up tenant by lab site code in a configured mapping table.
	return uuid.Nil.String()
}

// ---------------------------------------------------------------------------
// HL7 field extraction helpers
// ---------------------------------------------------------------------------

// extractNHI returns the patient NHI from PID-3.
func extractNHI(msg *hl7.Message) string {
	pid, ok := msg.GetSegment("PID")
	if !ok {
		return ""
	}
	return segField(pid, "3")
}

// extractZNZL returns the parsed ZNZL segment values, or an empty map.
func extractZNZL(msg *hl7.Message) map[string]string {
	znzlSeg, ok := msg.GetSegment("ZNZL")
	if !ok {
		return map[string]string{}
	}
	fields := make([]string, 15)
	fields[0] = "ZNZL"
	for i := 1; i <= 14; i++ {
		key := fmt.Sprintf("%d", i)
		if v, ok := znzlSeg[key]; ok && len(v) > 0 {
			fields[i] = v[0]
		}
	}
	return hl7.ParseZNZL(fields)
}

// segField returns the first component of field `key` from a segment map.
func segField(seg map[string][]string, key string) string {
	if seg == nil {
		return ""
	}
	v, ok := seg[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}

// segField2 returns the second component of field `key` from a segment map.
func segField2(seg map[string][]string, key string) string {
	if seg == nil {
		return ""
	}
	v, ok := seg[key]
	if !ok || len(v) < 2 {
		return ""
	}
	return v[1]
}

// firstComponent returns the first component of a segment map field.
func firstComponent(seg map[string][]string, key string) string {
	return segField(seg, key)
}

// secondComponent returns the second component of a segment map field.
func secondComponent(seg map[string][]string, key string) string {
	return segField2(seg, key)
}

// parseHL7DateTime parses an HL7 v2 date/time string (YYYYMMDDHHMMSS or YYYYMMDD).
func parseHL7DateTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	formats := []string{
		"20060102150405",
		"200601021504",
		"2006010215",
		"20060102",
	}
	for _, f := range formats {
		if len(s) == len(f) {
			t, err := time.ParseInLocation(f, s, time.UTC)
			if err == nil {
				return &t
			}
		}
	}
	// Try with timezone offset suffix: YYYYMMDDHHMMSS+HHMM
	if len(s) > 14 {
		t, err := time.ParseInLocation("20060102150405", s[:14], time.UTC)
		if err == nil {
			return &t
		}
	}
	return nil
}

// mapOBRStatus converts an OBR-25 HL7 result status code to a FHIR status.
func mapOBRStatus(code string) string {
	switch code {
	case "F":
		return "final"
	case "P":
		return "preliminary"
	case "C":
		return "corrected"
	case "X":
		return "cancelled"
	case "R":
		return "registered"
	default:
		return "registered"
	}
}

// mapOBXStatus converts an OBX-11 HL7 observation result status to a FHIR status.
func mapOBXStatus(code string) string {
	switch code {
	case "F":
		return "final"
	case "P":
		return "preliminary"
	case "C":
		return "corrected"
	case "X":
		return "cancelled"
	case "D":
		return "entered-in-error"
	default:
		return "final"
	}
}

// parseRefRange attempts to parse a numeric reference range string like
// "3.5-5.5" or "3.50 - 5.50" into low/high float64 values.
func parseRefRange(s string) (low, high float64, ok bool) {
	// Normalise separators.
	s = strings.TrimSpace(s)
	for _, sep := range []string{" - ", "–", "-"} {
		if idx := strings.Index(s, sep); idx > 0 {
			loStr := strings.TrimSpace(s[:idx])
			hiStr := strings.TrimSpace(s[idx+len(sep):])
			lo, err1 := strconv.ParseFloat(loStr, 64)
			hi, err2 := strconv.ParseFloat(hiStr, 64)
			if err1 == nil && err2 == nil {
				return lo, hi, true
			}
		}
	}
	return 0, 0, false
}

// interpretationDisplay maps an HL7 abnormal flag to a human-readable display.
func interpretationDisplay(code string) string {
	switch code {
	case "H":
		return "High"
	case "HH":
		return "Critical High"
	case "L":
		return "Low"
	case "LL":
		return "Critical Low"
	case "A":
		return "Abnormal"
	case "AA":
		return "Critical Abnormal"
	case "N":
		return "Normal"
	case "R":
		return "Resistant"
	case "S":
		return "Susceptible"
	default:
		return code
	}
}
