package repo

import (
	"fmt"
	"strings"
	"time"
)

// paramKind classifies how a FHIR search parameter maps to a JSONB query strategy.
type paramKind int

const (
	// paramID matches data->>'id' (or the resource_id column).
	paramID paramKind = iota
	// paramLastUpdated matches the updated_at column.
	paramLastUpdated
	// paramJSONBText matches a JSONB text value via ->> operator.
	paramJSONBText
	// paramJSONBContains matches a JSONB sub-document via @> operator.
	paramJSONBContains
	// paramIdentifier matches within the identifier array.
	paramIdentifier
)

// paramDef describes how to map a FHIR search parameter to SQL.
type paramDef struct {
	Kind    paramKind
	// JSONPath is the dot-separated path used for ->> or @> queries, e.g. "name,0,family".
	// For paramIdentifier, this is ignored; the identifier array is searched instead.
	JSONPath string
}

// knownParams maps resourceType → paramName → paramDef.
// Extend this map to add new search parameters.
var knownParams = map[string]map[string]paramDef{
	// Params shared across all resource types
	"*": {
		"_id": {Kind: paramID},
		"_lastUpdated": {Kind: paramLastUpdated},
	},
	"Patient": {
		"name":      {Kind: paramJSONBText, JSONPath: "name"},
		"family":    {Kind: paramJSONBText, JSONPath: "name"},
		"given":     {Kind: paramJSONBText, JSONPath: "name"},
		"birthdate": {Kind: paramJSONBText, JSONPath: "birthDate"},
		"gender":    {Kind: paramJSONBText, JSONPath: "gender"},
		"identifier": {Kind: paramIdentifier},
		"active":    {Kind: paramJSONBText, JSONPath: "active"},
	},
	"Practitioner": {
		"name":      {Kind: paramJSONBText, JSONPath: "name"},
		"family":    {Kind: paramJSONBText, JSONPath: "name"},
		"given":     {Kind: paramJSONBText, JSONPath: "name"},
		"birthdate": {Kind: paramJSONBText, JSONPath: "birthDate"},
		"gender":    {Kind: paramJSONBText, JSONPath: "gender"},
		"identifier": {Kind: paramIdentifier},
		"active":    {Kind: paramJSONBText, JSONPath: "active"},
	},
	"Encounter": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"patient":    {Kind: paramJSONBContains, JSONPath: "subject"},
		"identifier": {Kind: paramIdentifier},
		"date":       {Kind: paramJSONBText, JSONPath: "period"},
		"class":      {Kind: paramJSONBText, JSONPath: "class"},
		"type":       {Kind: paramJSONBText, JSONPath: "type"},
	},
	"Observation": {
		"status":       {Kind: paramJSONBText, JSONPath: "status"},
		"patient":      {Kind: paramJSONBContains, JSONPath: "subject"},
		"identifier":   {Kind: paramIdentifier},
		"code":         {Kind: paramJSONBText, JSONPath: "code"},
		"category":     {Kind: paramJSONBText, JSONPath: "category"},
		"date":         {Kind: paramJSONBText, JSONPath: "effectiveDateTime"},
	},
	"DiagnosticReport": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"patient":    {Kind: paramJSONBContains, JSONPath: "subject"},
		"identifier": {Kind: paramIdentifier},
		"code":       {Kind: paramJSONBText, JSONPath: "code"},
		"category":   {Kind: paramJSONBText, JSONPath: "category"},
		"date":       {Kind: paramJSONBText, JSONPath: "effectiveDateTime"},
	},
	"ServiceRequest": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"patient":    {Kind: paramJSONBContains, JSONPath: "subject"},
		"identifier": {Kind: paramIdentifier},
		"code":       {Kind: paramJSONBText, JSONPath: "code"},
		"category":   {Kind: paramJSONBText, JSONPath: "category"},
		"authored":   {Kind: paramJSONBText, JSONPath: "authoredOn"},
	},
	"Immunization": {
		"status":           {Kind: paramJSONBText, JSONPath: "status"},
		"patient":          {Kind: paramJSONBContains, JSONPath: "patient"},
		"identifier":       {Kind: paramIdentifier},
		"vaccine-code":     {Kind: paramJSONBContains, JSONPath: "vaccineCode"},
		"date":             {Kind: paramJSONBText, JSONPath: "occurrenceDateTime"},
	},
	"Claim": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"patient":    {Kind: paramJSONBContains, JSONPath: "patient"},
		"identifier": {Kind: paramIdentifier},
		"created":    {Kind: paramJSONBText, JSONPath: "created"},
		"provider":   {Kind: paramJSONBContains, JSONPath: "provider"},
	},
	"ClaimResponse": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"patient":    {Kind: paramJSONBContains, JSONPath: "patient"},
		"identifier": {Kind: paramIdentifier},
		"created":    {Kind: paramJSONBText, JSONPath: "created"},
		"outcome":    {Kind: paramJSONBText, JSONPath: "outcome"},
	},
	"ImagingStudy": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"patient":    {Kind: paramJSONBContains, JSONPath: "subject"},
		"identifier": {Kind: paramIdentifier},
		"modality":   {Kind: paramJSONBText, JSONPath: "modality"},
		"started":    {Kind: paramJSONBText, JSONPath: "started"},
	},
	"Subscription": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"identifier": {Kind: paramIdentifier},
		"topic":      {Kind: paramJSONBText, JSONPath: "topic"},
		"type":       {Kind: paramJSONBText, JSONPath: "channelType"},
	},
	"SubscriptionTopic": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"identifier": {Kind: paramIdentifier},
		"resource":   {Kind: paramJSONBText, JSONPath: "resourceTrigger"},
		"event":      {Kind: paramJSONBText, JSONPath: "eventTrigger"},
	},
	"MedicationRequest": {
		"status":     {Kind: paramJSONBText, JSONPath: "status"},
		"patient":    {Kind: paramJSONBContains, JSONPath: "subject"},
		"identifier": {Kind: paramIdentifier},
		"authoredon": {Kind: paramJSONBText, JSONPath: "authoredOn"},
		"intent":     {Kind: paramJSONBText, JSONPath: "intent"},
	},
}

// NHI system URL constant for NZ-specific identifier searches.
const nhiSystem = "https://standards.digital.health.nz/ns/nhi-id"

// SearchEngine builds parameterized SQL WHERE clauses from FHIR search parameters.
type SearchEngine struct {
	params map[string]map[string]paramDef
}

// NewSearchEngine creates a SearchEngine pre-loaded with known FHIR parameters.
func NewSearchEngine() *SearchEngine {
	return &SearchEngine{params: knownParams}
}

// RegisterParam registers a custom search parameter for a resource type.
// Use resourceType "*" to register a parameter for all resource types.
func (e *SearchEngine) RegisterParam(resourceType, name string, def paramDef) {
	if e.params[resourceType] == nil {
		e.params[resourceType] = make(map[string]paramDef)
	}
	e.params[resourceType][name] = def
}

// Build converts a SearchParams into a SQL WHERE fragment and positional args.
// The returned query is a safe, parameterized AND-clause ready to be appended
// after the caller's own WHERE conditions (tenant_id, resource_type, deleted_at).
// Args are 1-indexed starting at $1 — the caller must offset them appropriately
// if additional conditions precede this clause.
//
// Unsupported parameters are silently skipped to maintain FHIR's lenient search semantics.
func (e *SearchEngine) Build(params SearchParams) (query string, args []any, err error) {
	var clauses []string

	argIdx := 1

	lookupDef := func(paramName string) (paramDef, bool) {
		// Resource-specific params take precedence over wildcard params.
		if rt, ok := e.params[params.ResourceType]; ok {
			if def, ok := rt[paramName]; ok {
				return def, true
			}
		}
		if global, ok := e.params["*"]; ok {
			if def, ok := global[paramName]; ok {
				return def, true
			}
		}
		return paramDef{}, false
	}

	for paramName, values := range params.Params {
		if len(values) == 0 {
			continue
		}

		def, known := lookupDef(paramName)
		if !known {
			// Unknown params are silently ignored (FHIR lenient search).
			continue
		}

		switch def.Kind {
		case paramID:
			// Match against the resource_id column for efficiency.
			placeholders := make([]string, len(values))
			for i, v := range values {
				placeholders[i] = fmt.Sprintf("$%d", argIdx)
				args = append(args, v)
				argIdx++
			}
			clauses = append(clauses, fmt.Sprintf("AND resource_id IN (%s)", strings.Join(placeholders, ", ")))

		case paramLastUpdated:
			// Supports prefix modifiers: gt, lt, ge, le, eq (default eq).
			for _, v := range values {
				op, dateStr := parsePrefix(v)
				t, parseErr := time.Parse(time.RFC3339, dateStr)
				if parseErr != nil {
					// Try date-only format.
					t, parseErr = time.Parse("2006-01-02", dateStr)
					if parseErr != nil {
						return "", nil, fmt.Errorf("search: invalid _lastUpdated value %q", v)
					}
				}
				clauses = append(clauses, fmt.Sprintf("AND updated_at %s $%d", op, argIdx))
				args = append(args, t)
				argIdx++
			}

		case paramJSONBText:
			// Use ILIKE for name-like fields (array of objects), exact for scalars.
			path := def.JSONPath
			switch paramName {
			case "name", "family", "given":
				// name is an array of HumanName objects; search family + given text.
				var orClauses []string
				for _, v := range values {
					// Match family
					orClauses = append(orClauses,
						fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements(data->'%s') n WHERE n->>'family' ILIKE $%d)", path, argIdx))
					args = append(args, "%"+v+"%")
					argIdx++
					// Match any given element
					orClauses = append(orClauses,
						fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements(data->'%s') n, jsonb_array_elements_text(n->'given') g WHERE g ILIKE $%d)", path, argIdx))
					args = append(args, "%"+v+"%")
					argIdx++
				}
				clauses = append(clauses, "AND ("+strings.Join(orClauses, " OR ")+")")

			default:
				// Scalar text field.
				var orClauses []string
				for _, v := range values {
					orClauses = append(orClauses, fmt.Sprintf("data->>'%s' = $%d", path, argIdx))
					args = append(args, v)
					argIdx++
				}
				clauses = append(clauses, "AND ("+strings.Join(orClauses, " OR ")+")")
			}

		case paramJSONBContains:
			// Use @> for JSONB containment checks on sub-documents.
			path := def.JSONPath
			var orClauses []string
			for _, v := range values {
				// Build a minimal containment document, e.g. {"reference": "Patient/abc"}.
				containDoc := fmt.Sprintf(`{"reference":%q}`, v)
				orClauses = append(orClauses, fmt.Sprintf("data->'%s' @> $%d::jsonb", path, argIdx))
				args = append(args, containDoc)
				argIdx++
			}
			clauses = append(clauses, "AND ("+strings.Join(orClauses, " OR ")+")")

		case paramIdentifier:
			// Search identifier array.
			// Supports value-only or system|value format.
			// NZ-specific: if system is the NHI URL, restrict to that system.
			var orClauses []string
			for _, v := range values {
				system, value, hasSystem := strings.Cut(v, "|")
				if hasSystem {
					// system|value search — use @> containment on identifier array element.
					containDoc := fmt.Sprintf(`{"system":%q,"value":%q}`, system, value)
					orClauses = append(orClauses,
						fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements(data->'identifier') id WHERE id @> $%d::jsonb)", argIdx))
					args = append(args, containDoc)
					argIdx++
				} else {
					// Value-only search — match any identifier with this value.
					orClauses = append(orClauses,
						fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_array_elements(data->'identifier') id WHERE id->>'value' = $%d)", argIdx))
					args = append(args, v)
					argIdx++
				}
			}
			if len(orClauses) > 0 {
				clauses = append(clauses, "AND ("+strings.Join(orClauses, " OR ")+")")
			}
		}
	}

	return strings.Join(clauses, "\n"), args, nil
}

// parsePrefix extracts a FHIR date prefix (gt, lt, ge, le, sa, eb, ap) and returns
// the equivalent SQL comparison operator and the bare date string.
// Defaults to "=" if no known prefix is found.
func parsePrefix(v string) (op, dateStr string) {
	prefixMap := map[string]string{
		"eq": "=",
		"ne": "<>",
		"lt": "<",
		"le": "<=",
		"gt": ">",
		"ge": ">=",
	}
	if len(v) >= 2 {
		if sqlOp, ok := prefixMap[v[:2]]; ok {
			return sqlOp, v[2:]
		}
	}
	return "=", v
}
