// Package translate provides R4↔R5 FHIR resource translators for NZ healthcare.
// Translation is best-effort: fields present in one version but absent in the other
// are dropped or mapped to the nearest equivalent.
package translate

import (
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r4"
	"github.com/PhillipC05/tpt-healthcare/core/fhir/r5"
)

// ---------------------------------------------------------------------------
// Meta conversion helpers
// ---------------------------------------------------------------------------

func metaR4ToR5(m *r4.Meta) *r5.Meta {
	if m == nil {
		return nil
	}
	out := &r5.Meta{
		VersionID:   m.VersionID,
		LastUpdated: m.LastUpdated,
		Source:      m.Source,
		Profile:     m.Profile,
	}
	for _, t := range m.Tag {
		out.Tag = append(out.Tag, r5.Coding{System: t.System, Code: t.Code, Display: t.Display, Version: t.Version})
	}
	for _, s := range m.Security {
		out.Security = append(out.Security, r5.Coding{System: s.System, Code: s.Code, Display: s.Display, Version: s.Version})
	}
	return out
}

func metaR5ToR4(m *r5.Meta) *r4.Meta {
	if m == nil {
		return nil
	}
	out := &r4.Meta{
		VersionID:   m.VersionID,
		LastUpdated: m.LastUpdated,
		Source:      m.Source,
		Profile:     m.Profile,
	}
	for _, t := range m.Tag {
		out.Tag = append(out.Tag, r4.Coding{System: t.System, Code: t.Code, Display: t.Display, Version: t.Version})
	}
	for _, s := range m.Security {
		out.Security = append(out.Security, r4.Coding{System: s.System, Code: s.Code, Display: s.Display, Version: s.Version})
	}
	return out
}

// ---------------------------------------------------------------------------
// Coding / CodeableConcept helpers
// ---------------------------------------------------------------------------

func codingR4ToR5(c r4.Coding) r5.Coding {
	return r5.Coding{System: c.System, Version: c.Version, Code: c.Code, Display: c.Display}
}

func codingR5ToR4(c r5.Coding) r4.Coding {
	return r4.Coding{System: c.System, Version: c.Version, Code: c.Code, Display: c.Display}
}

func ccR4ToR5(c r4.CodeableConcept) r5.CodeableConcept {
	out := r5.CodeableConcept{Text: c.Text}
	for _, cd := range c.Coding {
		out.Coding = append(out.Coding, codingR4ToR5(cd))
	}
	return out
}

func ccR5ToR4(c r5.CodeableConcept) r4.CodeableConcept {
	out := r4.CodeableConcept{Text: c.Text}
	for _, cd := range c.Coding {
		out.Coding = append(out.Coding, codingR5ToR4(cd))
	}
	return out
}

func ccPtrR4ToR5(c *r4.CodeableConcept) *r5.CodeableConcept {
	if c == nil {
		return nil
	}
	v := ccR4ToR5(*c)
	return &v
}

func ccPtrR5ToR4(c *r5.CodeableConcept) *r4.CodeableConcept {
	if c == nil {
		return nil
	}
	v := ccR5ToR4(*c)
	return &v
}

// ---------------------------------------------------------------------------
// Identifier helpers
// ---------------------------------------------------------------------------

func identifierR4ToR5(id r4.Identifier) r5.Identifier {
	out := r5.Identifier{
		Use:    id.Use,
		System: id.System,
		Value:  id.Value,
		Type:   ccR4ToR5(id.Type),
	}
	if id.Period != nil {
		out.Period = &r5.Period{Start: id.Period.Start, End: id.Period.End}
	}
	return out
}

func identifierR5ToR4(id r5.Identifier) r4.Identifier {
	out := r4.Identifier{
		Use:    id.Use,
		System: id.System,
		Value:  id.Value,
		Type:   ccR5ToR4(id.Type),
	}
	if id.Period != nil {
		out.Period = &r4.Period{Start: id.Period.Start, End: id.Period.End}
	}
	return out
}

func identifiersR4ToR5(ids []r4.Identifier) []r5.Identifier {
	if ids == nil {
		return nil
	}
	out := make([]r5.Identifier, len(ids))
	for i, id := range ids {
		out[i] = identifierR4ToR5(id)
	}
	return out
}

func identifiersR5ToR4(ids []r5.Identifier) []r4.Identifier {
	if ids == nil {
		return nil
	}
	out := make([]r4.Identifier, len(ids))
	for i, id := range ids {
		out[i] = identifierR5ToR4(id)
	}
	return out
}

// ---------------------------------------------------------------------------
// HumanName helpers
// ---------------------------------------------------------------------------

func periodR4ToR5(p *r4.Period) *r5.Period {
	if p == nil {
		return nil
	}
	return &r5.Period{Start: p.Start, End: p.End}
}

func periodR5ToR4(p *r5.Period) *r4.Period {
	if p == nil {
		return nil
	}
	return &r4.Period{Start: p.Start, End: p.End}
}

func nameR4ToR5(n r4.HumanName) r5.HumanName {
	return r5.HumanName{
		Use:    n.Use,
		Text:   n.Text,
		Family: n.Family,
		Given:  n.Given,
		Prefix: n.Prefix,
		Suffix: n.Suffix,
		Period: periodR4ToR5(n.Period),
	}
}

func nameR5ToR4(n r5.HumanName) r4.HumanName {
	return r4.HumanName{
		Use:    n.Use,
		Text:   n.Text,
		Family: n.Family,
		Given:  n.Given,
		Prefix: n.Prefix,
		Suffix: n.Suffix,
		Period: periodR5ToR4(n.Period),
	}
}

func namesR4ToR5(names []r4.HumanName) []r5.HumanName {
	if names == nil {
		return nil
	}
	out := make([]r5.HumanName, len(names))
	for i, n := range names {
		out[i] = nameR4ToR5(n)
	}
	return out
}

func namesR5ToR4(names []r5.HumanName) []r4.HumanName {
	if names == nil {
		return nil
	}
	out := make([]r4.HumanName, len(names))
	for i, n := range names {
		out[i] = nameR5ToR4(n)
	}
	return out
}

// ---------------------------------------------------------------------------
// Address helpers
// ---------------------------------------------------------------------------

func addressR4ToR5(a r4.Address) r5.Address {
	return r5.Address{
		Use:        a.Use,
		Type:       a.Type,
		Text:       a.Text,
		Line:       a.Line,
		City:       a.City,
		District:   a.District,
		State:      a.State,
		PostalCode: a.PostalCode,
		Country:    a.Country,
		Period:     periodR4ToR5(a.Period),
	}
}

func addressR5ToR4(a r5.Address) r4.Address {
	return r4.Address{
		Use:        a.Use,
		Type:       a.Type,
		Text:       a.Text,
		Line:       a.Line,
		City:       a.City,
		District:   a.District,
		State:      a.State,
		PostalCode: a.PostalCode,
		Country:    a.Country,
		Period:     periodR5ToR4(a.Period),
	}
}

func addressesR4ToR5(addrs []r4.Address) []r5.Address {
	if addrs == nil {
		return nil
	}
	out := make([]r5.Address, len(addrs))
	for i, a := range addrs {
		out[i] = addressR4ToR5(a)
	}
	return out
}

func addressesR5ToR4(addrs []r5.Address) []r4.Address {
	if addrs == nil {
		return nil
	}
	out := make([]r4.Address, len(addrs))
	for i, a := range addrs {
		out[i] = addressR5ToR4(a)
	}
	return out
}

// ---------------------------------------------------------------------------
// ContactPoint helpers
// ---------------------------------------------------------------------------

func contactR4ToR5(c r4.ContactPoint) r5.ContactPoint {
	return r5.ContactPoint{System: c.System, Value: c.Value, Use: c.Use, Rank: c.Rank, Period: periodR4ToR5(c.Period)}
}

func contactR5ToR4(c r5.ContactPoint) r4.ContactPoint {
	return r4.ContactPoint{System: c.System, Value: c.Value, Use: c.Use, Rank: c.Rank, Period: periodR5ToR4(c.Period)}
}

func contactsR4ToR5(cs []r4.ContactPoint) []r5.ContactPoint {
	if cs == nil {
		return nil
	}
	out := make([]r5.ContactPoint, len(cs))
	for i, c := range cs {
		out[i] = contactR4ToR5(c)
	}
	return out
}

func contactsR5ToR4(cs []r5.ContactPoint) []r4.ContactPoint {
	if cs == nil {
		return nil
	}
	out := make([]r4.ContactPoint, len(cs))
	for i, c := range cs {
		out[i] = contactR5ToR4(c)
	}
	return out
}

// ---------------------------------------------------------------------------
// Reference helpers
// ---------------------------------------------------------------------------

func refR4ToR5(r *r4.Reference) *r5.Reference {
	if r == nil {
		return nil
	}
	out := &r5.Reference{Reference: r.Reference, Type: r.Type, Display: r.Display}
	if r.Identifier != nil {
		id := identifierR4ToR5(*r.Identifier)
		out.Identifier = &id
	}
	return out
}

func refR5ToR4(r *r5.Reference) *r4.Reference {
	if r == nil {
		return nil
	}
	out := &r4.Reference{Reference: r.Reference, Type: r.Type, Display: r.Display}
	if r.Identifier != nil {
		id := identifierR5ToR4(*r.Identifier)
		out.Identifier = &id
	}
	return out
}

// ---------------------------------------------------------------------------
// Extension helpers
// ---------------------------------------------------------------------------

func extensionsR4ToR5(exts []r4.Extension) []r5.Extension {
	if exts == nil {
		return nil
	}
	out := make([]r5.Extension, len(exts))
	for i, e := range exts {
		out[i] = r5.Extension{
			URL:                  e.URL,
			ValueString:          e.ValueString,
			ValueCode:            e.ValueCode,
			ValueCoding:          (*r5.Coding)(nil),
			ValueCodeableConcept: ccPtrR4ToR5(e.ValueCodeableConcept),
			ValueBoolean:         e.ValueBoolean,
			ValueInteger:         e.ValueInteger,
			ValueDecimal:         e.ValueDecimal,
			ValueDateTime:        e.ValueDateTime,
			ValueReference:       refR4ToR5(e.ValueReference),
			Extension:            extensionsR4ToR5(e.Extension),
		}
		if e.ValueCoding != nil {
			cd := codingR4ToR5(*e.ValueCoding)
			out[i].ValueCoding = &cd
		}
	}
	return out
}

func extensionsR5ToR4(exts []r5.Extension) []r4.Extension {
	if exts == nil {
		return nil
	}
	out := make([]r4.Extension, len(exts))
	for i, e := range exts {
		out[i] = r4.Extension{
			URL:                  e.URL,
			ValueString:          e.ValueString,
			ValueCode:            e.ValueCode,
			ValueCoding:          (*r4.Coding)(nil),
			ValueCodeableConcept: ccPtrR5ToR4(e.ValueCodeableConcept),
			ValueBoolean:         e.ValueBoolean,
			ValueInteger:         e.ValueInteger,
			ValueDecimal:         e.ValueDecimal,
			ValueDateTime:        e.ValueDateTime,
			ValueReference:       refR5ToR4(e.ValueReference),
			Extension:            extensionsR5ToR4(e.Extension),
		}
		if e.ValueCoding != nil {
			cd := codingR5ToR4(*e.ValueCoding)
			out[i].ValueCoding = &cd
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Patient translators
// ---------------------------------------------------------------------------

// PatientR4ToR5 converts an R4 Patient to an R5 Patient.
// NZ-specific extension URLs are identical in both versions and are preserved as-is.
func PatientR4ToR5(r4p *r4.Patient) *r5.Patient {
	if r4p == nil {
		return nil
	}
	out := &r5.Patient{
		ResourceType:         "Patient",
		ID:                   r4p.ID,
		Meta:                 metaR4ToR5(r4p.Meta),
		Extension:            extensionsR4ToR5(r4p.Extension),
		Identifier:           identifiersR4ToR5(r4p.Identifier),
		Active:               r4p.Active,
		Name:                 namesR4ToR5(r4p.Name),
		Telecom:              contactsR4ToR5(r4p.Telecom),
		Gender:               r4p.Gender,
		BirthDate:            r4p.BirthDate,
		DeceasedBoolean:      r4p.DeceasedBoolean,
		DeceasedDateTime:     r4p.DeceasedDateTime,
		Address:              addressesR4ToR5(r4p.Address),
		MaritalStatus:        ccPtrR4ToR5(r4p.MaritalStatus),
		ManagingOrganization: refR4ToR5(r4p.ManagingOrganization),
	}
	for _, comm := range r4p.Communication {
		out.Communication = append(out.Communication, r5.PatientCommunication{
			Language:  ccR4ToR5(comm.Language),
			Preferred: comm.Preferred,
		})
	}
	for _, c := range r4p.Contact {
		rc := r5.PatientContact{
			Name:    nil,
			Gender:  c.Gender,
			Address: nil,
		}
		if c.Name != nil {
			n := nameR4ToR5(*c.Name)
			rc.Name = &n
		}
		if c.Address != nil {
			a := addressR4ToR5(*c.Address)
			rc.Address = &a
		}
		for _, rel := range c.Relationship {
			rc.Relationship = append(rc.Relationship, ccR4ToR5(rel))
		}
		rc.Telecom = contactsR4ToR5(c.Telecom)
		out.Contact = append(out.Contact, rc)
	}
	return out
}

// PatientR5ToR4 converts an R5 Patient to an R4 Patient.
// R5-only fields (not present in R4) are dropped.
func PatientR5ToR4(r5p *r5.Patient) *r4.Patient {
	if r5p == nil {
		return nil
	}
	out := &r4.Patient{
		ResourceType:         "Patient",
		ID:                   r5p.ID,
		Meta:                 metaR5ToR4(r5p.Meta),
		Extension:            extensionsR5ToR4(r5p.Extension),
		Identifier:           identifiersR5ToR4(r5p.Identifier),
		Active:               r5p.Active,
		Name:                 namesR5ToR4(r5p.Name),
		Telecom:              contactsR5ToR4(r5p.Telecom),
		Gender:               r5p.Gender,
		BirthDate:            r5p.BirthDate,
		DeceasedBoolean:      r5p.DeceasedBoolean,
		DeceasedDateTime:     r5p.DeceasedDateTime,
		Address:              addressesR5ToR4(r5p.Address),
		MaritalStatus:        ccPtrR5ToR4(r5p.MaritalStatus),
		ManagingOrganization: refR5ToR4(r5p.ManagingOrganization),
	}
	for _, comm := range r5p.Communication {
		out.Communication = append(out.Communication, r4.PatientCommunication{
			Language:  ccR5ToR4(comm.Language),
			Preferred: comm.Preferred,
		})
	}
	for _, c := range r5p.Contact {
		rc := r4.PatientContact{
			Name:    nil,
			Gender:  c.Gender,
			Address: nil,
		}
		if c.Name != nil {
			n := nameR5ToR4(*c.Name)
			rc.Name = &n
		}
		if c.Address != nil {
			a := addressR5ToR4(*c.Address)
			rc.Address = &a
		}
		for _, rel := range c.Relationship {
			rc.Relationship = append(rc.Relationship, ccR5ToR4(rel))
		}
		rc.Telecom = contactsR5ToR4(c.Telecom)
		out.Contact = append(out.Contact, rc)
	}
	return out
}

// ---------------------------------------------------------------------------
// Practitioner translators
// ---------------------------------------------------------------------------

// PractitionerR4ToR5 converts an R4 Practitioner to an R5 Practitioner.
func PractitionerR4ToR5(r4p *r4.Practitioner) *r5.Practitioner {
	if r4p == nil {
		return nil
	}
	out := &r5.Practitioner{
		ResourceType: "Practitioner",
		ID:           r4p.ID,
		Meta:         metaR4ToR5(r4p.Meta),
		Extension:    extensionsR4ToR5(r4p.Extension),
		Identifier:   identifiersR4ToR5(r4p.Identifier),
		Active:       r4p.Active,
		Name:         namesR4ToR5(r4p.Name),
		Telecom:      contactsR4ToR5(r4p.Telecom),
		Gender:       r4p.Gender,
		BirthDate:    r4p.BirthDate,
		Address:      addressesR4ToR5(r4p.Address),
	}
	for _, q := range r4p.Qualification {
		rq := r5.PractitionerQualification{
			Code:   ccR4ToR5(q.Code),
			Period: periodR4ToR5(q.Period),
			Issuer: refR4ToR5(q.Issuer),
		}
		rq.Identifier = identifiersR4ToR5(q.Identifier)
		out.Qualification = append(out.Qualification, rq)
	}
	return out
}

// PractitionerR5ToR4 converts an R5 Practitioner to an R4 Practitioner.
func PractitionerR5ToR4(r5p *r5.Practitioner) *r4.Practitioner {
	if r5p == nil {
		return nil
	}
	out := &r4.Practitioner{
		ResourceType: "Practitioner",
		ID:           r5p.ID,
		Meta:         metaR5ToR4(r5p.Meta),
		Extension:    extensionsR5ToR4(r5p.Extension),
		Identifier:   identifiersR5ToR4(r5p.Identifier),
		Active:       r5p.Active,
		Name:         namesR5ToR4(r5p.Name),
		Telecom:      contactsR5ToR4(r5p.Telecom),
		Gender:       r5p.Gender,
		BirthDate:    r5p.BirthDate,
		Address:      addressesR5ToR4(r5p.Address),
	}
	for _, q := range r5p.Qualification {
		rq := r4.PractitionerQualification{
			Code:   ccR5ToR4(q.Code),
			Period: periodR5ToR4(q.Period),
			Issuer: refR5ToR4(q.Issuer),
		}
		rq.Identifier = identifiersR5ToR4(q.Identifier)
		out.Qualification = append(out.Qualification, rq)
	}
	return out
}

// ---------------------------------------------------------------------------
// Immunization translators
// ---------------------------------------------------------------------------

// ImmunizationR5ToR4 converts an R5 Immunization to an R4 Immunization.
// Both versions are similar; this is a thin wrapper preserving fields.
func ImmunizationR5ToR4(r5im *r5.Immunization) *r4.Immunization {
	if r5im == nil {
		return nil
	}
	out := &r4.Immunization{
		ResourceType:       "Immunization",
		ID:                 r5im.ID,
		Meta:               metaR5ToR4(r5im.Meta),
		Extension:          extensionsR5ToR4(r5im.Extension),
		Identifier:         identifiersR5ToR4(r5im.Identifier),
		Status:             r5im.Status,
		VaccineCode:        ccR5ToR4(r5im.VaccineCode),
		OccurrenceDateTime: r5im.OccurrenceDateTime,
		LotNumber:          r5im.LotNumber,
		Route:              ccPtrR5ToR4(r5im.Route),
		DoseQuantity:       qtyR5ToR4(r5im.DoseQuantity),
	}
	// Convert patient reference
	if r5im.Patient != nil {
		out.Patient = &r4.Reference{
			Reference:  r5im.Patient.Reference,
			Type:       r5im.Patient.Type,
			Identifier: nil,
			Display:    r5im.Patient.Display,
		}
		if r5im.Patient.Identifier != nil {
			id := identifierR5ToR4(*r5im.Patient.Identifier)
			out.Patient.Identifier = &id
		}
	}
	// Convert notes
	for _, n := range r5im.Note {
		ann := r4.Annotation{
			AuthorString: n.AuthorString,
			Text:         n.Text,
		}
		if n.AuthorReference != nil {
			ann.AuthorReference = refR5ToR4(n.AuthorReference)
		}
		if n.Time != nil {
			ann.Time = n.Time
		}
		out.Note = append(out.Note, ann)
	}
	return out
}

// qtyR5ToR4 converts an R5 Quantity to an R4 Quantity.
func qtyR5ToR4(q *r5.Quantity) *r4.Quantity {
	if q == nil {
		return nil
	}
	return &r4.Quantity{
		Value:      q.Value,
		Comparator: q.Comparator,
		Unit:       q.Unit,
		System:     q.System,
		Code:       q.Code,
	}
}
