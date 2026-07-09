// Package fhir adds missing FHIR resource types.
// Location resource for both R4 and R5.
package r4

// Location represents a FHIR R4 Location resource.
type Location struct {
	ResourceType string           `json:"resourceType"`
	ID           string           `json:"id,omitempty"`
	Meta         *Meta            `json:"meta,omitempty"`
	Identifier   []Identifier     `json:"identifier,omitempty"`
	Status       string           `json:"status"` // active, suspended, inactive
	Name         string           `json:"name"`
	Alias        []string         `json:"alias,omitempty"`
	Description  string           `json:"description,omitempty"`
	Mode         string           `json:"mode,omitempty"` // instance, kind
	Type         []CodeableConcept `json:"type,omitempty"`
	Telecom      []ContactPoint   `json:"telecom,omitempty"`
	Address      *Address         `json:"address,omitempty"`
	PhysicalType CodeableConcept  `json:"physicalType,omitempty"`
	Position     *LocationPosition `json:"position,omitempty"`
	PartOf       *Reference       `json:"partOf,omitempty"`
	Manages      []Reference      `json:"manages,omitempty"`
	EndOfPeriod  *Period          `json:"endOfPeriod,omitempty"`
}

// LocationPosition holds geographic coordinates.
type LocationPosition struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Altitude  float64 `json:"altitude,omitempty"`
}

// NewLocation creates a new FHIR R4 Location resource.
func NewLocation(id, name, status string) Location {
	return Location{
		ResourceType: "Location",
		ID:           id,
		Name:         name,
		Status:       status,
	}
}

// WithAddress sets the address on a Location.
func (l Location) WithAddress(line1, city, state, postalCode, country string) Location {
	l.Address = &Address{
		Use:        "work",
		Type:       "physical",
		Line:       []string{line1},
		City:       city,
		State:      state,
		PostalCode: postalCode,
		Country:    country,
	}
	return l
}

// WithPosition sets the geographic coordinates.
func (l Location) WithPosition(lat, lon float64) Location {
	l.Position = &LocationPosition{
		Latitude:  lat,
		Longitude: lon,
	}
	return l
}

// WithType sets the location type (e.g. ward, bed, room).
func (l Location) WithType(coding Coding, text string) Location {
	l.Type = append(l.Type, CodeableConcept{
		Coding: []Coding{coding},
		Text:   text,
	})
	return l
}
