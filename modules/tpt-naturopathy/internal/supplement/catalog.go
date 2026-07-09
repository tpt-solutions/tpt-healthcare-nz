// Package supplement provides natural supplement tracking for naturopathy.
package supplement

type Supplement struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Brand             string `json:"brand"`
	Category          string `json:"category"` // vitamin, mineral, herbal, botanical, probiotic, amino_acid, enzyme, other
	Form              string `json:"form"`     // capsule, tablet, liquid, powder, tincture, cream
	Strength          string `json:"strength"` // e.g. "500mg", "1000IU"
	Ingredients       string `json:"ingredients"`
	Dosage            string `json:"dosage"`
	Contraindications string `json:"contraindications,omitempty"`
	Interactions      string `json:"interactions,omitempty"`
	NZMedicinesCode   string `json:"nzMedicinesCode,omitempty"` // NZ Medicines Terminology code
	Active            bool   `json:"active"`
	CreatedAt         int64  `json:"createdAt"`
	UpdatedAt         int64  `json:"updatedAt"`
}

type SupplementPrescription struct {
	ID             string `json:"id"`
	PatientNHI     string `json:"patientNhi"`
	SupplementID   string `json:"supplementId"`
	SupplementName string `json:"supplementName"`
	Dosage         string `json:"dosage"`
	Frequency      string `json:"frequency"` // daily, twice_daily, weekly, as_needed
	Duration       string `json:"duration"`
	Reason         string `json:"reason"`
	ClinicianID    string `json:"clinicianId"`
	PracticeID     string `json:"practiceId"`
	Active         bool   `json:"active"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}
