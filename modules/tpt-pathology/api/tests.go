package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// TestCatalogEntry is the domain model for a pathology_test_catalog row.
type TestCatalogEntry struct {
	ID              string    `json:"id"`
	LOINCCode       string    `json:"loincCode"`
	DisplayName     string    `json:"displayName"`
	ShortName       string    `json:"shortName"`
	Category        string    `json:"category"`
	SpecimenType    string    `json:"specimenType,omitempty"`
	TurnaroundHours int       `json:"turnaroundHours"`
	IsPanel         bool      `json:"isPanel"`
	Components      []string  `json:"components,omitempty"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"createdAt"`
}

// ReferenceRange is the domain model for a pathology_reference_ranges row.
// Ranges are stratified by lab, sex, age bracket, and clinical condition (e.g. fasting, pregnant).
type ReferenceRange struct {
	ID        string   `json:"id"`
	LOINCCode string   `json:"loincCode"`
	LabID     string   `json:"labId,omitempty"`
	Sex       string   `json:"sex"`
	AgeFrom   int      `json:"ageFrom"`
	AgeTo     int      `json:"ageTo"`
	Condition string   `json:"condition,omitempty"`
	Low       *float64 `json:"low,omitempty"`
	High      *float64 `json:"high,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	TextRange string   `json:"textRange,omitempty"`
}

// testCreateRequest is the body for POST /api/v1/tests.
type testCreateRequest struct {
	LOINCCode       string   `json:"loincCode"`
	DisplayName     string   `json:"displayName"`
	ShortName       string   `json:"shortName,omitempty"`
	Category        string   `json:"category,omitempty"`
	SpecimenType    string   `json:"specimenType,omitempty"`
	TurnaroundHours int      `json:"turnaroundHours,omitempty"`
	IsPanel         bool     `json:"isPanel,omitempty"`
	Components      []string `json:"components,omitempty"`
}

// refRangeCreateRequest is the body for POST /api/v1/tests/{loinc}/reference-ranges.
type refRangeCreateRequest struct {
	LabID     string   `json:"labId,omitempty"`
	Sex       string   `json:"sex,omitempty"`
	AgeFrom   int      `json:"ageFrom,omitempty"`
	AgeTo     int      `json:"ageTo,omitempty"`
	Condition string   `json:"condition,omitempty"`
	Low       *float64 `json:"low,omitempty"`
	High      *float64 `json:"high,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	TextRange string   `json:"textRange,omitempty"`
}

// TestsHandler handles /api/v1/tests routes.
type TestsHandler struct {
	pool   db.Pool
	logger *slog.Logger
}

// List handles GET /api/v1/tests.
// Supports query params: category, active (bool), q (name search).
func (h *TestsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	q := r.URL.Query()
	categoryFilter := q.Get("category")
	nameSearch := q.Get("q")
	activeOnly := q.Get("active") != "false" // default: only active tests

	tests, err := h.listTests(ctx, categoryFilter, nameSearch, activeOnly)
	if err != nil {
		h.logger.Error("list test catalog", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list tests"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tests": tests,
		"total": len(tests),
	})
}

// Get handles GET /api/v1/tests/{loinc}.
func (h *TestsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loincCode := r.PathValue("loinc")
	if loincCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_LOINC", Message: "LOINC code is required"})
		return
	}

	test, err := h.getTest(ctx, loincCode)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "test not found"})
			return
		}
		h.logger.Error("get test", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve test"})
		return
	}

	writeJSON(w, http.StatusOK, test)
}

// Create handles POST /api/v1/tests.
// Adds a new LOINC test entry to the catalog.
func (h *TestsHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	var req testCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.LOINCCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "loincCode is required"})
		return
	}
	if req.DisplayName == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "displayName is required"})
		return
	}
	if req.Category == "" {
		req.Category = "laboratory"
	}

	test, err := h.insertTest(ctx, req)
	if err != nil {
		h.logger.Error("insert test", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create test"})
		return
	}

	writeJSON(w, http.StatusCreated, test)
}

// ListReferenceRanges handles GET /api/v1/tests/{loinc}/reference-ranges.
// Supports query params: lab_id, sex, age.
func (h *TestsHandler) ListReferenceRanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loincCode := r.PathValue("loinc")
	if loincCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_LOINC", Message: "LOINC code is required"})
		return
	}

	q := r.URL.Query()
	labID := q.Get("lab_id")
	sex := q.Get("sex")

	ranges, err := h.listReferenceRanges(ctx, loincCode, labID, sex)
	if err != nil {
		h.logger.Error("list reference ranges", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list reference ranges"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"loincCode":       loincCode,
		"referenceRanges": ranges,
		"total":           len(ranges),
	})
}

// CreateReferenceRange handles POST /api/v1/tests/{loinc}/reference-ranges.
func (h *TestsHandler) CreateReferenceRange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	_, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}

	loincCode := r.PathValue("loinc")
	if loincCode == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_LOINC", Message: "LOINC code is required"})
		return
	}

	var req refRangeCreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if req.Low == nil && req.High == nil && req.TextRange == "" {
		writeJSON(w, http.StatusBadRequest, apiError{
			Code:    "VALIDATION_ERROR",
			Message: "at least one of low, high, or textRange must be provided",
		})
		return
	}
	if req.Sex == "" {
		req.Sex = "any"
	}
	if req.AgeTo == 0 {
		req.AgeTo = 999
	}

	rr, err := h.insertReferenceRange(ctx, loincCode, req)
	if err != nil {
		h.logger.Error("insert reference range", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to create reference range"})
		return
	}

	writeJSON(w, http.StatusCreated, rr)
}

// ---------------------------------------------------------------------------
// Data access
// ---------------------------------------------------------------------------

func (h *TestsHandler) listTests(ctx context.Context, category, nameSearch string, activeOnly bool) ([]TestCatalogEntry, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, loinc_code, display_name, short_name, category,
		        specimen_type, turnaround_hours, is_panel, components, active, created_at
		 FROM pathology_test_catalog
		 WHERE (@category    = '' OR category = @category)
		   AND (@name_search = '' OR display_name ILIKE '%' || @name_search || '%'
		                          OR short_name   ILIKE '%' || @name_search || '%')
		   AND (NOT @active_only OR active = true)
		 ORDER BY display_name ASC
		 LIMIT 500`,
		db.NamedArgs{
			"category":    category,
			"name_search": nameSearch,
			"active_only": activeOnly,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query test catalog: %w", err)
	}
	defer rows.Close()

	var results []TestCatalogEntry
	for rows.Next() {
		var t TestCatalogEntry
		if err := rows.Scan(
			&t.ID, &t.LOINCCode, &t.DisplayName, &t.ShortName, &t.Category,
			&t.SpecimenType, &t.TurnaroundHours, &t.IsPanel, &t.Components,
			&t.Active, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan test catalog: %w", err)
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

func (h *TestsHandler) getTest(ctx context.Context, loincCode string) (TestCatalogEntry, error) {
	var t TestCatalogEntry
	err := h.pool.QueryRow(ctx,
		`SELECT id, loinc_code, display_name, short_name, category,
		        specimen_type, turnaround_hours, is_panel, components, active, created_at
		 FROM pathology_test_catalog
		 WHERE loinc_code = @loinc_code`,
		db.NamedArgs{"loinc_code": loincCode},
	).Scan(
		&t.ID, &t.LOINCCode, &t.DisplayName, &t.ShortName, &t.Category,
		&t.SpecimenType, &t.TurnaroundHours, &t.IsPanel, &t.Components,
		&t.Active, &t.CreatedAt,
	)
	if err != nil {
		if db.IsNoRows(err) {
			return TestCatalogEntry{}, errNotFound
		}
		return TestCatalogEntry{}, fmt.Errorf("get test: %w", err)
	}
	return t, nil
}

func (h *TestsHandler) insertTest(ctx context.Context, req testCreateRequest) (TestCatalogEntry, error) {
	comps := req.Components
	if comps == nil {
		comps = []string{}
	}
	var t TestCatalogEntry
	err := h.pool.QueryRow(ctx,
		`INSERT INTO pathology_test_catalog
		   (loinc_code, display_name, short_name, category, specimen_type,
		    turnaround_hours, is_panel, components)
		 VALUES
		   (@loinc_code, @display_name, @short_name, @category, @specimen_type,
		    @turnaround_hours, @is_panel, @components)
		 ON CONFLICT (loinc_code) DO UPDATE
		   SET display_name     = EXCLUDED.display_name,
		       short_name       = EXCLUDED.short_name,
		       category         = EXCLUDED.category,
		       specimen_type    = EXCLUDED.specimen_type,
		       turnaround_hours = EXCLUDED.turnaround_hours,
		       is_panel         = EXCLUDED.is_panel,
		       components       = EXCLUDED.components
		 RETURNING id, loinc_code, display_name, short_name, category,
		           specimen_type, turnaround_hours, is_panel, components, active, created_at`,
		db.NamedArgs{
			"loinc_code":       req.LOINCCode,
			"display_name":     req.DisplayName,
			"short_name":       req.ShortName,
			"category":         req.Category,
			"specimen_type":    req.SpecimenType,
			"turnaround_hours": req.TurnaroundHours,
			"is_panel":         req.IsPanel,
			"components":       comps,
		},
	).Scan(
		&t.ID, &t.LOINCCode, &t.DisplayName, &t.ShortName, &t.Category,
		&t.SpecimenType, &t.TurnaroundHours, &t.IsPanel, &t.Components,
		&t.Active, &t.CreatedAt,
	)
	if err != nil {
		return TestCatalogEntry{}, fmt.Errorf("insert test: %w", err)
	}
	return t, nil
}

func (h *TestsHandler) listReferenceRanges(ctx context.Context, loincCode, labID, sex string) ([]ReferenceRange, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, loinc_code, lab_id, sex, age_from, age_to,
		        condition, low, high, unit, text_range
		 FROM pathology_reference_ranges
		 WHERE loinc_code = @loinc_code
		   AND (@lab_id = '' OR lab_id = @lab_id OR lab_id = '')
		   AND (@sex    = '' OR sex    = @sex    OR sex    = 'any')
		 ORDER BY lab_id ASC, sex ASC, age_from ASC`,
		db.NamedArgs{
			"loinc_code": loincCode,
			"lab_id":     labID,
			"sex":        sex,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query reference ranges: %w", err)
	}
	defer rows.Close()

	var results []ReferenceRange
	for rows.Next() {
		var rr ReferenceRange
		if err := rows.Scan(
			&rr.ID, &rr.LOINCCode, &rr.LabID, &rr.Sex,
			&rr.AgeFrom, &rr.AgeTo, &rr.Condition,
			&rr.Low, &rr.High, &rr.Unit, &rr.TextRange,
		); err != nil {
			return nil, fmt.Errorf("scan reference range: %w", err)
		}
		results = append(results, rr)
	}
	return results, rows.Err()
}

func (h *TestsHandler) insertReferenceRange(ctx context.Context, loincCode string, req refRangeCreateRequest) (ReferenceRange, error) {
	var rr ReferenceRange
	err := h.pool.QueryRow(ctx,
		`INSERT INTO pathology_reference_ranges
		   (loinc_code, lab_id, sex, age_from, age_to, condition, low, high, unit, text_range)
		 VALUES
		   (@loinc_code, @lab_id, @sex, @age_from, @age_to, @condition, @low, @high, @unit, @text_range)
		 RETURNING id, loinc_code, lab_id, sex, age_from, age_to,
		           condition, low, high, unit, text_range`,
		db.NamedArgs{
			"loinc_code": loincCode,
			"lab_id":     req.LabID,
			"sex":        req.Sex,
			"age_from":   req.AgeFrom,
			"age_to":     req.AgeTo,
			"condition":  req.Condition,
			"low":        req.Low,
			"high":       req.High,
			"unit":       req.Unit,
			"text_range": req.TextRange,
		},
	).Scan(
		&rr.ID, &rr.LOINCCode, &rr.LabID, &rr.Sex,
		&rr.AgeFrom, &rr.AgeTo, &rr.Condition,
		&rr.Low, &rr.High, &rr.Unit, &rr.TextRange,
	)
	if err != nil {
		return ReferenceRange{}, fmt.Errorf("insert reference range: %w", err)
	}
	return rr, nil
}
