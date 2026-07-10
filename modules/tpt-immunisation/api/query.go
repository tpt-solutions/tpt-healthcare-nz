package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotFound = errors.New("record not found")

// ---- Immunisation Records ----

const immSelectCols = `id, patient_nhi, status, vaccine_code, vaccine_display,
	occurrence_datetime, site_code, site_display, route_code, route_display,
	lot_number, expiry_date, practitioner_hpi_cpn,
	nir_submitted, nir_submitted_at, nir_reference_id,
	note, fhir_resource, created_at, updated_at`

func scanImmunisation(row interface{ Scan(dest ...any) error }) (Immunisation, error) {
	var (
		im        Immunisation
		fhirJSON  []byte
		siteCode  string
		siteDisp  string
		routeCode string
		routeDisp string
	)
	if err := row.Scan(
		&im.ID, &im.PatientNHI, &im.Status,
		&im.VaccineCode.Coding[0].Code, &im.VaccineCode.Coding[0].Display,
		&im.OccurrenceDateTime,
		&siteCode, &siteDisp,
		&routeCode, &routeDisp,
		&im.LotNumber, &im.ExpiryDate,
		&im.PractitionerHPICPN,
		&im.NIRSubmitted, &im.NIRSubmittedAt, &im.NIRReferenceID,
		&im.Note, &fhirJSON, &im.CreatedAt, &im.UpdatedAt,
	); err != nil {
		return Immunisation{}, err
	}

	im.ResourceType = "Immunization"
	im.VaccineCode.Coding[0].System = "https://www.nzmt.org.nz/"
	if im.VaccineCode.Text == "" {
		im.VaccineCode.Text = im.VaccineCode.Coding[0].Display
	}
	if siteCode != "" {
		im.Site.Coding = []struct {
			System  string `json:"system"`
			Code    string `json:"code"`
			Display string `json:"display"`
		}{{System: "http://snomed.info/sct", Code: siteCode, Display: siteDisp}}
	}
	if routeCode != "" {
		im.Route.Coding = []struct {
			System  string `json:"system"`
			Code    string `json:"code"`
			Display string `json:"display"`
		}{{System: "http://snomed.info/sct", Code: routeCode, Display: routeDisp}}
	}
	if fhirJSON != nil {
		_ = json.Unmarshal(fhirJSON, &im)
	}
	return im, nil
}

func listImmunisations(ctx context.Context, pool *pgxpool.Pool, patientNHI string) ([]Immunisation, error) {
	rows, err := pool.Query(ctx,
		`SELECT `+immSelectCols+`
		 FROM immunisation_records
		 WHERE patient_nhi = @nhi
		 ORDER BY occurrence_datetime DESC
		 LIMIT 200`,
		db.NamedArgs{"nhi": patientNHI},
	)
	if err != nil {
		return nil, fmt.Errorf("query immunisations: %w", err)
	}
	defer rows.Close()

	var results []Immunisation
	for rows.Next() {
		im, err := scanImmunisation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan immunisation: %w", err)
		}
		results = append(results, im)
	}
	return results, rows.Err()
}

func getImmunisation(ctx context.Context, pool *pgxpool.Pool, id string) (Immunisation, error) {
	row := pool.QueryRow(ctx,
		`SELECT `+immSelectCols+`
		 FROM immunisation_records
		 WHERE id = @id`,
		db.NamedArgs{"id": id},
	)
	im, err := scanImmunisation(row)
	if err != nil {
		if db.IsNoRows(err) {
			return Immunisation{}, errNotFound
		}
		return Immunisation{}, fmt.Errorf("get immunisation: %w", err)
	}
	return im, nil
}

func createImmunisation(ctx context.Context, pool *pgxpool.Pool, im Immunisation) (Immunisation, error) {
	vaccineCode := ""
	vaccineDisplay := ""
	if len(im.VaccineCode.Coding) > 0 {
		vaccineCode = im.VaccineCode.Coding[0].Code
		vaccineDisplay = im.VaccineCode.Coding[0].Display
	}
	siteCode, siteDisplay := extractCoding(im.Site.Coding)
	routeCode, routeDisplay := extractCoding(im.Route.Coding)

	row := pool.QueryRow(ctx,
		`INSERT INTO immunisation_records
		   (id, patient_nhi, status, vaccine_code, vaccine_display,
		    occurrence_datetime, site_code, site_display, route_code, route_display,
		    lot_number, expiry_date, practitioner_hpi_cpn,
		    nir_submitted, note, created_at, updated_at)
		 VALUES
		   (@id, @patient_nhi, @status, @vaccine_code, @vaccine_display,
		    @occurrence_datetime, @site_code, @site_display, @route_code, @route_display,
		    @lot_number, @expiry_date, @practitioner_hpi_cpn,
		    @nir_submitted, @note, @created_at, @updated_at)
		 RETURNING `+immSelectCols,
		db.NamedArgs{
			"id":                  im.ID,
			"patient_nhi":         im.PatientNHI,
			"status":              im.Status,
			"vaccine_code":        vaccineCode,
			"vaccine_display":     vaccineDisplay,
			"occurrence_datetime": im.OccurrenceDateTime,
			"site_code":           siteCode,
			"site_display":        siteDisplay,
			"route_code":          routeCode,
			"route_display":       routeDisplay,
			"lot_number":          im.LotNumber,
			"expiry_date":         im.ExpiryDate,
			"practitioner_hpi_cpn": im.PractitionerHPICPN,
			"nir_submitted":       im.NIRSubmitted,
			"note":                im.Note,
			"created_at":          im.CreatedAt,
			"updated_at":          im.UpdatedAt,
		},
	)
	result, err := scanImmunisation(row)
	if err != nil {
		return Immunisation{}, fmt.Errorf("insert immunisation: %w", err)
	}
	return result, nil
}

func updateNIRSubmission(ctx context.Context, pool *pgxpool.Pool, id string, refID string, submittedAt time.Time) error {
	tag, err := pool.Exec(ctx,
		`UPDATE immunisation_records
		 SET nir_submitted = true,
		     nir_reference_id = @ref_id,
		     nir_submitted_at = @submitted_at,
		     updated_at = now()
		 WHERE id = @id`,
		db.NamedArgs{"id": id, "ref_id": refID, "submitted_at": submittedAt},
	)
	if err != nil {
		return fmt.Errorf("update nir submission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// ---- Outbreaks ----

const outbreakSelectCols = `id, disease, snomed_code, region, reported_at,
	active_until, cases_count, contact_email, notes, created_at`

func scanOutbreak(row interface{ Scan(dest ...any) error }) (DiseaseOutbreak, error) {
	var o DiseaseOutbreak
	if err := row.Scan(
		&o.ID, &o.Disease, &o.SNOMEDCode, &o.Region, &o.ReportedAt,
		&o.ActiveUntil, &o.CasesCount, &o.ContactEmail, &o.Notes, &o.CreatedAt,
	); err != nil {
		return DiseaseOutbreak{}, err
	}
	return o, nil
}

func createOutbreak(ctx context.Context, pool *pgxpool.Pool, o DiseaseOutbreak) (DiseaseOutbreak, error) {
	row := pool.QueryRow(ctx,
		`INSERT INTO outbreaks
		   (id, disease, snomed_code, region, reported_at, active_until,
		    cases_count, contact_email, notes, created_at)
		 VALUES
		   (@id, @disease, @snomed_code, @region, @reported_at, @active_until,
		    @cases_count, @contact_email, @notes, @created_at)
		 RETURNING `+outbreakSelectCols,
		db.NamedArgs{
			"id":            o.ID,
			"disease":       o.Disease,
			"snomed_code":   o.SNOMEDCode,
			"region":        o.Region,
			"reported_at":   o.ReportedAt,
			"active_until":  o.ActiveUntil,
			"cases_count":   o.CasesCount,
			"contact_email": o.ContactEmail,
			"notes":         o.Notes,
			"created_at":    o.CreatedAt,
		},
	)
	result, err := scanOutbreak(row)
	if err != nil {
		return DiseaseOutbreak{}, fmt.Errorf("insert outbreak: %w", err)
	}
	return result, nil
}

// ---- Recalls ----

const recallSelectCols = `id, patient_nhi, outbreak_id, disease,
	missing_vaccines, last_contact_at, priority, created_at, updated_at`

func scanRecall(row interface{ Scan(dest ...any) error }) (RecallEntry, error) {
	var r RecallEntry
	if err := row.Scan(
		&r.PatientNHI, &r.OutbreakID, &r.Disease,
		&r.MissingVaccines, &r.LastContactDate, &r.Priority,
	); err != nil {
		return RecallEntry{}, err
	}
	return r, nil
}

func listRecalls(ctx context.Context, pool *pgxpool.Pool, outbreakID, region, priority string) ([]RecallEntry, error) {
	query := `SELECT r.patient_nhi, r.outbreak_id, r.disease,
		r.missing_vaccines, r.last_contact_at, r.priority
		 FROM recalls r
		 JOIN outbreaks o ON o.id = r.outbreak_id
		 WHERE 1=1`
	args := db.NamedArgs{}

	if outbreakID != "" {
		query += ` AND r.outbreak_id = @outbreak_id`
		args["outbreak_id"] = outbreakID
	}
	if region != "" {
		query += ` AND o.region = @region`
		args["region"] = region
	}
	if priority != "" {
		query += ` AND r.priority = @priority`
		args["priority"] = priority
	}
	query += ` ORDER BY r.created_at DESC LIMIT 500`

	rows, err := pool.Query(ctx, query, args)
	if err != nil {
		return nil, fmt.Errorf("query recalls: %w", err)
	}
	defer rows.Close()

	var results []RecallEntry
	for rows.Next() {
		e, err := scanRecall(rows)
		if err != nil {
			return nil, fmt.Errorf("scan recall: %w", err)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// ---- Helpers ----

func extractCoding(codings []struct {
	System  string `json:"system"`
	Code    string `json:"code"`
	Display string `json:"display"`
}) (code, display string) {
	if len(codings) > 0 {
		return codings[0].Code, codings[0].Display
	}
	return "", ""
}
