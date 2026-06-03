// Package hl7 provides HL7 v2 message parsing for NZ lab and clinical messages,
// including support for NZ Z-segments used by Labtests, Healthscope, and SCL.
package hl7

import (
	"errors"
	"fmt"
	"strings"
)

// Segment is a map keyed by field name (e.g. "PID.3") to the field value.
type Segment map[string]string

// Message represents a parsed HL7 v2 message.
type Message struct {
	// Type is the message type/event, e.g. "ORU^R01".
	Type string
	// Segments holds all parsed segments as ordered slice of field maps.
	// Each entry maps field index strings (e.g. "3", "3.1") to values.
	Segments []map[string][]string
	// segIndex maps segment name to indices in Segments slice.
	segIndex map[string][]int
	// raw is the original unparsed message.
	raw string
}

// supportedTypes lists the HL7 message types handled by this package.
var supportedTypes = map[string]bool{
	"ORU^R01": true, // Lab results (unsolicited observation result)
	"ADT^A01": true, // Admit / Visit notification
	"ADT^A08": true, // Update patient information
	"ORM^O01": true, // General order message
}

// Parse parses a raw HL7 v2 pipe-delimited message string.
// Segments are separated by \r, \n, or \r\n.
// Returns an error if the message does not begin with a valid MSH segment.
func Parse(raw string) (*Message, error) {
	if raw == "" {
		return nil, errors.New("hl7: empty message")
	}

	// Normalise line endings.
	normalised := strings.ReplaceAll(raw, "\r\n", "\r")
	normalised = strings.ReplaceAll(normalised, "\n", "\r")
	lines := strings.Split(normalised, "\r")

	// Filter blank lines.
	var segLines []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			segLines = append(segLines, l)
		}
	}
	if len(segLines) == 0 {
		return nil, errors.New("hl7: message contains no segments")
	}

	// First segment must be MSH.
	if !strings.HasPrefix(segLines[0], "MSH") {
		return nil, fmt.Errorf("hl7: expected MSH as first segment, got %q", segLines[0][:min(len(segLines[0]), 3)])
	}

	msg := &Message{
		raw:      raw,
		segIndex: make(map[string][]int),
	}

	for _, line := range segLines {
		if len(line) < 3 {
			continue
		}
		segName := line[:3]
		fields := strings.Split(line, "|")
		// fields[0] is the segment name; shift so fields[1] == field 1.
		fieldMap := make(map[string][]string)
		fieldMap["0"] = []string{segName}
		for i := 1; i < len(fields); i++ {
			key := fmt.Sprintf("%d", i)
			components := strings.Split(fields[i], "^")
			fieldMap[key] = components
		}
		idx := len(msg.Segments)
		msg.Segments = append(msg.Segments, fieldMap)
		msg.segIndex[segName] = append(msg.segIndex[segName], idx)
	}

	// Derive message type from MSH-9 (field 9 of MSH segment).
	msh, ok := msg.GetSegment("MSH")
	if !ok {
		return nil, errors.New("hl7: MSH segment not found after parse")
	}
	if f9, exists := msh["9"]; exists && len(f9) > 0 {
		if len(f9) == 1 {
			msg.Type = f9[0]
		} else {
			// MSH-9 is composite: MessageCode^TriggerEvent^MessageStructure
			msg.Type = strings.Join(f9[:min(len(f9), 2)], "^")
		}
	}

	return msg, nil
}

// GetSegment returns the first segment with the given name (e.g. "PID") and
// true, or nil and false if no such segment exists.
func (m *Message) GetSegment(name string) (map[string][]string, bool) {
	indices, ok := m.segIndex[name]
	if !ok || len(indices) == 0 {
		return nil, false
	}
	return m.Segments[indices[0]], true
}

// GetAllSegments returns all segments with the given name.
// Returns an empty slice if no segments with that name exist.
func (m *Message) GetAllSegments(name string) []map[string][]string {
	indices, ok := m.segIndex[name]
	if !ok {
		return nil
	}
	result := make([]map[string][]string, 0, len(indices))
	for _, idx := range indices {
		result = append(result, m.Segments[idx])
	}
	return result
}

// GetField returns the value of a field within the first matching segment.
// segment is the 3-character segment name (e.g. "PID"), field is the 1-based
// field number as a string (e.g. "3"). Returns the first component of the
// field value, or empty string if not found.
func (m *Message) GetField(segment, field string) string {
	seg, ok := m.GetSegment(segment)
	if !ok {
		return ""
	}
	components, exists := seg[field]
	if !exists || len(components) == 0 {
		return ""
	}
	return components[0]
}

// Raw returns the original unparsed HL7 message string.
func (m *Message) Raw() string {
	return m.raw
}

// ParseZNZL parses a ZNZL NZ Lab Z-segment field slice (as returned by
// splitting the raw segment on "|") into a human-readable key/value map.
//
// The ZNZL segment is used by NZ laboratory providers Labtests, Healthscope,
// and SCL to carry NZ-specific lab metadata not supported by the base HL7 v2
// specification.
//
// fields[0] is expected to be "ZNZL"; subsequent fields carry the values
// defined in the NZ Lab Messaging Standard.
func ParseZNZL(fields []string) map[string]string {
	result := make(map[string]string)
	if len(fields) == 0 || (len(fields) > 0 && fields[0] != "ZNZL") {
		return result
	}

	// NZ Lab Messaging Standard ZNZL field definitions.
	// Field positions are 1-based relative to the segment start.
	znzlFields := map[int]string{
		1:  "SegmentName",      // ZNZL
		2:  "LabOrderNumber",   // NZ lab order/accession number
		3:  "CollectionSite",   // Collection site / lab branch code
		4:  "FundingCode",      // NZ funding source code (e.g. "PHO", "DHB")
		5:  "CopyToProvider",   // Copy-to GP/provider NHI or HPI
		6:  "ReferralType",     // Referral type code
		7:  "SpecimenState",    // Specimen state (e.g. "F"=Final, "P"=Preliminary)
		8:  "UrgencyIndicator", // Urgency indicator
		9:  "ReportFormat",     // Report format code
		10: "LabSite",          // Laboratory site identifier
		11: "PatientCategory",  // Patient category (e.g. inpatient / outpatient)
		12: "NHINumber",        // Patient NHI number (NZ National Health Index)
		13: "CopyFlag",         // Copy report flag
		14: "CommentText",      // Free-text comment
	}

	for i, val := range fields {
		if name, ok := znzlFields[i]; ok {
			result[name] = val
		} else if i > 0 {
			result[fmt.Sprintf("Field%d", i)] = val
		}
	}
	return result
}

// min returns the smaller of two ints (backfill for Go < 1.21 built-in).
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
