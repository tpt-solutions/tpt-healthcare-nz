package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// VaccineScheduleEntry describes the vaccines due at a specific age point in the
// New Zealand National Immunisation Schedule, as published by the Ministry of Health /
// Te Whatu Ora (Health New Zealand).
//
// Vaccine names use NZMT (New Zealand Medicines Terminology) common names.
// The full NZMT codes are resolved at runtime from core/terminology.
//
// Schedule source: https://www.health.govt.nz/our-work/preventative-health-wellness/immunisation/new-zealand-immunisation-schedule
type VaccineScheduleEntry struct {
	// AgeMonths is the age in months at which these vaccines are due.
	// For sub-monthly ages (e.g., 6 weeks = 1.5 months), the nearest integer month
	// rounded up is used. Callers should use DueVaccines() which handles the tolerance window.
	AgeMonths int `json:"ageMonths"`

	// AgeLabel is a human-readable label for the schedule point (e.g., "6 weeks", "3 months").
	AgeLabel string `json:"ageLabel"`

	// Vaccines is a list of vaccine common names from NZMT.
	Vaccines []string `json:"vaccines"`

	// Notes contains clinical guidance, catch-up information, or eligibility criteria
	// specific to this schedule point.
	Notes string `json:"notes,omitempty"`
}

// NZImmunisationSchedule is the complete NZ National Immunisation Schedule.
// Ages are represented in whole months for schedule comparison; fractional months
// (6 weeks ≈ 1.5) are rounded to the nearest schedule window.
//
// The schedule is effective from the 2023 revision. Always verify against the
// Te Whatu Ora schedule publication before clinical use.
//
// Vaccines listed use NZMT common names. SNOMED CT and NZMT CT codes are resolved
// at runtime via core/terminology for FHIR resource population.
var NZImmunisationSchedule = []VaccineScheduleEntry{
	{
		AgeMonths: 2, // 6 weeks
		AgeLabel:  "6 weeks",
		Vaccines: []string{
			"Rotarix (rotavirus)",
			"Infanrix-hexa (diphtheria, tetanus, pertussis, hepatitis B, polio, Hib)",
			"Synflorix (pneumococcal)",
		},
		Notes: "First dose. Rotarix must not be given after 24 weeks of age. " +
			"Infanrix-hexa provides hepatitis B primary series; birth dose required if mother is HBsAg positive.",
	},
	{
		AgeMonths: 3, // 3 months
		AgeLabel:  "3 months",
		Vaccines: []string{
			"Rotarix (rotavirus)",
			"Infanrix-hexa (diphtheria, tetanus, pertussis, hepatitis B, polio, Hib)",
			"Synflorix (pneumococcal)",
		},
		Notes: "Second dose. Minimum interval from first dose is 4 weeks.",
	},
	{
		AgeMonths: 5, // 5 months
		AgeLabel:  "5 months",
		Vaccines: []string{
			"Infanrix-hexa (diphtheria, tetanus, pertussis, hepatitis B, polio, Hib)",
			"Synflorix (pneumococcal)",
		},
		Notes: "Third dose of Infanrix-hexa and Synflorix. Rotarix series is complete by this point.",
	},
	{
		AgeMonths: 12, // 12 months
		AgeLabel:  "12 months",
		Vaccines: []string{
			"Priorix (measles, mumps, rubella)",
			"Varivax (varicella)",
			"Synflorix (pneumococcal)",
			"Hiberix (Haemophilus influenzae type b)",
		},
		Notes: "First MMR and varicella dose. Synflorix and Hib booster. " +
			"MMR should not be given within 4 weeks of another live vaccine.",
	},
	{
		AgeMonths: 15, // 15 months
		AgeLabel:  "15 months",
		Vaccines: []string{
			"Priorix (measles, mumps, rubella)",
		},
		Notes: "Second MMR dose. Minimum interval from first MMR is 4 weeks. " +
			"Early second dose is offered at 15 months as part of the elimination strategy.",
	},
	{
		AgeMonths: 48, // 4 years
		AgeLabel:  "4 years",
		Vaccines: []string{
			"Infanrix-IPV (diphtheria, tetanus, pertussis, polio)",
			"Priorix (measles, mumps, rubella)",
		},
		Notes: "Pre-school booster. Third MMR dose offered if not previously given. " +
			"Check immunisation history before administering.",
	},
	{
		AgeMonths: 132, // 11 years
		AgeLabel:  "11 years",
		Vaccines: []string{
			"Boostrix (diphtheria, tetanus, pertussis)",
			"Gardasil 9 (HPV — first dose)",
		},
		Notes: "School-based immunisation programme (Year 7). " +
			"HPV vaccination: two doses required if starting before age 15; " +
			"three doses if immunocompromised or starting at age 15 or older. " +
			"Gardasil 9 protects against HPV types 6, 11, 16, 18, 31, 33, 45, 52, 58.",
	},
	{
		AgeMonths: 540, // 45 years
		AgeLabel:  "45 years",
		Vaccines: []string{
			"Zostavax or Shingrix (herpes zoster / shingles)",
		},
		Notes: "Funded for eligible individuals under the Special Authority criteria. " +
			"Shingrix (recombinant adjuvanted) is preferred over Zostavax (live attenuated) " +
			"for immunocompromised patients. Two doses of Shingrix required (0 and 2–6 months).",
	},
	{
		AgeMonths: 780, // 65 years
		AgeLabel:  "65 years",
		Vaccines: []string{
			"Fluarix Tetra or Afluria Quad (influenza — annual)",
			"Pneumovax 23 (pneumococcal polysaccharide)",
			"Zostavax or Shingrix (herpes zoster / shingles)",
		},
		Notes: "Annual influenza vaccine is funded for all adults 65+. " +
			"Pneumovax 23 is a one-time dose (Prevenar 13 may be given first if not previously received). " +
			"Shingles vaccine: Shingrix is funded for immunocompromised individuals; " +
			"Zostavax is funded for immunocompetent individuals if Shingrix is unavailable.",
	},
}

// DueVaccines returns all NZImmunisationSchedule entries that fall within a
// ±4-week (1-month) tolerance window of the given age in months.
//
// The tolerance window accounts for the practical flexibility in the NZ schedule
// where vaccines can be given slightly early or late. Callers should display all
// returned entries and highlight any that are overdue (entry.AgeMonths < ageMonths).
func DueVaccines(ageMonths int) []VaccineScheduleEntry {
	const toleranceMonths = 1
	var due []VaccineScheduleEntry
	for _, entry := range NZImmunisationSchedule {
		diff := ageMonths - entry.AgeMonths
		if diff >= -toleranceMonths && diff <= toleranceMonths {
			due = append(due, entry)
		}
	}
	return due
}

// --- Outbreak and recall types ---

// DiseaseOutbreak records a notifiable disease outbreak at a location.
// Outbreak data is used to generate immunisation recall lists for under-immunised
// patients in the affected area.
type DiseaseOutbreak struct {
	ID           string    `json:"id"`
	Disease      string    `json:"disease"` // e.g., "measles", "pertussis", "meningococcal"
	// SNOMEDCode is the SNOMED CT code for the disease.
	SNOMEDCode   string    `json:"snomedCode"`
	Region       string    `json:"region"`      // DHB/locality region
	ReportedAt   time.Time `json:"reportedAt"`
	ActiveUntil  time.Time `json:"activeUntil"` // Estimated end of outbreak window
	CasesCount   int       `json:"casesCount"`
	ContactEmail string    `json:"contactEmail,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// RecallEntry identifies a patient who should be recalled for immunisation
// based on an active outbreak and their immunisation history.
type RecallEntry struct {
	PatientNHI      string    `json:"patientNhi"` // Encrypted in storage; decrypted for authorised users
	OutbreakID      string    `json:"outbreakId"`
	Disease         string    `json:"disease"`
	MissingVaccines []string  `json:"missingVaccines"`
	LastContactDate *time.Time `json:"lastContactDate,omitempty"`
	Priority        string    `json:"priority"` // "high" | "medium" | "low"
}

// OutbreakHandler handles /api/v1/outbreaks and /api/v1/recalls.
type OutbreakHandler struct {
	logger *slog.Logger
}

// Record handles POST /api/v1/outbreaks — register a new disease outbreak.
//
// Outbreaks are used by the recall engine to identify patients who are not adequately
// immunised against the outbreak disease in the affected region. When an outbreak is
// registered, a background job queries the immunisation repository for under-immunised
// patients in the region and populates the recall list.
//
// Outbreak reporting is a public health function under the Health Act 1956 s.74. Only
// practitioners with the appropriate HPI scope may submit outbreaks.
func (h *OutbreakHandler) Record(w http.ResponseWriter, r *http.Request) {
	var outbreak DiseaseOutbreak
	if err := json.NewDecoder(r.Body).Decode(&outbreak); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("record outbreak: decode: %v", err))
		return
	}

	if outbreak.Disease == "" {
		writeError(w, http.StatusUnprocessableEntity, "disease is required")
		return
	}
	if outbreak.Region == "" {
		writeError(w, http.StatusUnprocessableEntity, "region is required")
		return
	}
	if outbreak.ActiveUntil.IsZero() {
		writeError(w, http.StatusUnprocessableEntity, "activeUntil is required")
		return
	}

	// In production:
	//   1. Validate practitioner HPI scope (must include public health function).
	//   2. Persist DiseaseOutbreak.
	//   3. Enqueue a background recall-generation job via core/events.
	//   4. Write AuditEvent with action="OUTBREAK-RECORDED".
	//   5. Optionally notify regional public health unit via email/webhook.

	now := time.Now().UTC()
	outbreak.ID = fmt.Sprintf("outbreak-%d", now.UnixNano())
	outbreak.ReportedAt = now
	outbreak.CreatedAt = now

	h.logger.Info("outbreak recorded",
		"id", outbreak.ID,
		"disease", outbreak.Disease,
		"region", outbreak.Region,
		"cases", outbreak.CasesCount,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusCreated, outbreak)
}

// Recalls handles GET /api/v1/recalls — return the current immunisation recall list.
//
// Query parameters:
//   - outbreak_id: filter by specific outbreak (optional)
//   - region: filter by region (optional)
//   - priority: "high" | "medium" | "low" (optional)
//
// The recall list contains patient NHIs that should be prioritised for immunisation
// contact based on active outbreaks in their area. NHIs are returned encrypted; the
// calling application is responsible for decryption using core/encryption.
//
// Privacy Act 2020 Note: The recall list constitutes health information disclosure
// (HIPC Rule 11). It must only be accessed by authorised public health or clinical staff
// with a documented lawful purpose (outbreak response). All access is audit-logged.
func (h *OutbreakHandler) Recalls(w http.ResponseWriter, r *http.Request) {
	outbreakID := r.URL.Query().Get("outbreak_id")
	region := r.URL.Query().Get("region")
	priority := r.URL.Query().Get("priority")

	// In production:
	//   1. Validate requester's authorization (public health scope via HPI).
	//   2. Check consent / HIPC Rule 11 exception for outbreak response.
	//   3. Query recall list from repository, filtered by params.
	//   4. Write AuditEvent (read recall list) via core/audit.

	h.logger.Info("recall list accessed",
		"outbreak_id", outbreakID,
		"region", region,
		"priority", priority,
		"request_id", r.Context().Value(requestIDKey),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"recalls":    []RecallEntry{},
		"total":      0,
		"generatedAt": time.Now().UTC(),
	})
}
