package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/PhillipC05/tpt-healthcare/core/audit"
	"github.com/PhillipC05/tpt-healthcare/core/db"
	"github.com/PhillipC05/tpt-healthcare/core/encryption"
	"github.com/PhillipC05/tpt-healthcare/core/middleware"
)

// Observation LOINC codes for supported remote monitoring types.
// Source: https://loinc.org/
const (
	LOINCBloodPressureSystolic  = "8480-6"
	LOINCBloodPressureDiastolic = "8462-4"
	LOINCHeartRate              = "8867-4"
	LOINCOxygenSaturation       = "59408-5"
	LOINCBloodGlucose           = "15074-8"
	LOINCBodyWeight             = "29463-7"
	LOINCBodyTemperature        = "8310-5"
	LOINCRespiratoryRate        = "9279-1"
	LOINCPeakExpiratoryFlow     = "19935-6"
)

// knownLoincCodes is the set of LOINC codes accepted for remote monitoring.
var knownLoincCodes = map[string]string{
	LOINCBloodPressureSystolic:  "Blood Pressure (Systolic)",
	LOINCBloodPressureDiastolic: "Blood Pressure (Diastolic)",
	LOINCHeartRate:              "Heart Rate",
	LOINCOxygenSaturation:       "Oxygen Saturation",
	LOINCBloodGlucose:           "Blood Glucose",
	LOINCBodyWeight:             "Body Weight",
	LOINCBodyTemperature:        "Body Temperature",
	LOINCRespiratoryRate:        "Respiratory Rate",
	LOINCPeakExpiratoryFlow:     "Peak Expiratory Flow",
}

// DeviceType enumerates the supported remote monitoring device categories.
type DeviceType string

const (
	DeviceTypeBPMonitor     DeviceType = "bp_monitor"
	DeviceTypePulseOximeter DeviceType = "pulse_oximeter"
	DeviceTypeGlucoseMeter  DeviceType = "glucose_meter"
	DeviceTypeScale         DeviceType = "scale"
	DeviceTypeThermometer   DeviceType = "thermometer"
	DeviceTypePeakFlowMeter DeviceType = "peak_flow_meter"
	DeviceTypeSpirometer    DeviceType = "spirometer"
)

// Device is the domain model for a registered remote monitoring device.
type Device struct {
	ID           string     `json:"id"`
	PatientNHI   string     `json:"patientNhi"`
	DeviceType   DeviceType `json:"deviceType"`
	Manufacturer string     `json:"manufacturer,omitempty"`
	Model        string     `json:"model,omitempty"`
	SerialNumber string     `json:"serialNumber,omitempty"`
	Status       string     `json:"status"`
	TenantID     string     `json:"tenantId"`
	RegisteredAt time.Time  `json:"registeredAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

// Observation is the domain model for a single remote monitoring measurement.
// ValueQuantity and ValueUnit are set for numeric readings (most vitals).
// ValueString is set for coded or free-text readings.
type Observation struct {
	ID              string    `json:"id"`
	PatientNHI      string    `json:"patientNhi"`
	DeviceID        string    `json:"deviceId,omitempty"`
	LOINCCode       string    `json:"loincCode"`
	ObservationType string    `json:"observationType"`
	ValueQuantity   *float64  `json:"valueQuantity,omitempty"`
	ValueUnit       string    `json:"valueUnit,omitempty"`
	ValueString     string    `json:"valueString,omitempty"`
	EffectiveAt     time.Time `json:"effectiveAt"`
	Source          string    `json:"source"`
	TenantID        string    `json:"tenantId"`
	CreatedAt       time.Time `json:"createdAt"`
}

// PatientVitalSummary holds the most recent reading for each observation type.
type PatientVitalSummary struct {
	PatientNHI   string                 `json:"patientNhi"`
	GeneratedAt  time.Time              `json:"generatedAt"`
	LatestValues map[string]Observation `json:"latestValues"`
}

// deviceRegisterRequest is the body for POST /api/v1/monitoring/devices.
type deviceRegisterRequest struct {
	PatientNHI   string     `json:"patientNhi"`
	DeviceType   DeviceType `json:"deviceType"`
	Manufacturer string     `json:"manufacturer,omitempty"`
	Model        string     `json:"model,omitempty"`
	SerialNumber string     `json:"serialNumber,omitempty"`
}

// observationSubmitRequest is the body for POST /api/v1/monitoring/observations.
type observationSubmitRequest struct {
	PatientNHI    string   `json:"patientNhi"`
	DeviceID      string   `json:"deviceId,omitempty"`
	LOINCCode     string   `json:"loincCode"`
	ValueQuantity *float64 `json:"valueQuantity,omitempty"`
	ValueUnit     string   `json:"valueUnit,omitempty"`
	ValueString   string   `json:"valueString,omitempty"`
	EffectiveAt   time.Time `json:"effectiveAt"`
	Source        string   `json:"source,omitempty"`
}

// MonitoringHandler handles all /api/v1/monitoring routes.
type MonitoringHandler struct {
	pool       db.Pool
	enc        *encryption.Cipher
	auditTrail *audit.Trail
	logger     *slog.Logger
}

// RegisterDevice handles POST /api/v1/monitoring/devices.
func (h *MonitoringHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req deviceRegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validateDeviceRegister(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	encNHI, err := h.enc.Encrypt(req.PatientNHI)
	if err != nil {
		h.logger.Error("encrypt NHI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to secure patient identifier"})
		return
	}

	device, err := h.insertDevice(ctx, req, tenantID, encNHI)
	if err != nil {
		h.logger.Error("insert device", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to register device"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "MonitoringDevice",
		ResourceID:   device.ID,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, device)
}

// ListDevices handles GET /api/v1/monitoring/devices.
// Query params: patient (NHI), status.
func (h *MonitoringHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "active"
	}

	devices, err := h.listDevices(ctx, tenantID, statusFilter)
	if err != nil {
		h.logger.Error("list devices", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list devices"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "MonitoringDevice",
		ResourceID:   "list",
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{"devices": devices, "total": len(devices)})
}

// SubmitObservation handles POST /api/v1/monitoring/observations.
func (h *MonitoringHandler) SubmitObservation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	var req observationSubmitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "INVALID_BODY", Message: err.Error()})
		return
	}
	if err := validateObservationSubmit(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: err.Error()})
		return
	}

	encNHI, err := h.enc.Encrypt(req.PatientNHI)
	if err != nil {
		h.logger.Error("encrypt NHI", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to secure patient identifier"})
		return
	}

	obs, err := h.insertObservation(ctx, req, tenantID, encNHI)
	if err != nil {
		h.logger.Error("insert observation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "INSERT_ERROR", Message: "failed to store observation"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionWrite,
		ResourceType: "MonitoringObservation",
		ResourceID:   obs.ID,
		TenantID:     tenantID,
		Metadata:     map[string]string{"loinc": req.LOINCCode, "source": obs.Source},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusCreated, obs)
}

// ListObservations handles GET /api/v1/monitoring/observations.
// Query params: patient (NHI), loinc (code), from (RFC3339), to (RFC3339).
func (h *MonitoringHandler) ListObservations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	q := r.URL.Query()
	loincFilter := q.Get("loinc")
	fromFilter := q.Get("from")
	toFilter := q.Get("to")

	observations, err := h.listObservations(ctx, tenantID, loincFilter, fromFilter, toFilter)
	if err != nil {
		h.logger.Error("list observations", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "LIST_ERROR", Message: "failed to list observations"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "MonitoringObservation",
		ResourceID:   "list",
		TenantID:     tenantID,
		Metadata:     map[string]string{"loinc": loincFilter, "from": fromFilter, "to": toFilter},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, map[string]any{"observations": observations, "total": len(observations)})
}

// GetObservation handles GET /api/v1/monitoring/observations/{id}.
func (h *MonitoringHandler) GetObservation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_ID", Message: "observation ID is required"})
		return
	}

	obs, err := h.getObservationByID(ctx, id, tenantID)
	if err != nil {
		if errors.Is(err, errNotFound) {
			writeJSON(w, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "observation not found"})
			return
		}
		h.logger.Error("get observation", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "GET_ERROR", Message: "failed to retrieve observation"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "MonitoringObservation",
		ResourceID:   id,
		TenantID:     tenantID,
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, obs)
}

// PatientSummary handles GET /api/v1/monitoring/patients/{nhi}/summary.
// Returns the most recent observation for each LOINC type for the patient.
func (h *MonitoringHandler) PatientSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID, ok := middleware.TenantFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_TENANT", Message: "tenant ID is required"})
		return
	}
	principal, ok := middleware.PrincipalFromContext(ctx)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Code: "UNAUTHENTICATED", Message: "authentication required"})
		return
	}

	nhi := r.PathValue("nhi")
	if nhi == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Code: "MISSING_NHI", Message: "patient NHI is required"})
		return
	}

	encNHI, err := h.enc.Encrypt(nhi)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "ENCRYPT_ERROR", Message: "failed to secure patient identifier"})
		return
	}

	latestValues, err := h.latestObservationsByNHI(ctx, tenantID, encNHI, nhi)
	if err != nil {
		h.logger.Error("patient summary", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, apiError{Code: "SUMMARY_ERROR", Message: "failed to generate patient summary"})
		return
	}

	if err := h.auditTrail.Write(ctx, audit.Event{
		Actor:        principal,
		Action:       audit.ActionRead,
		ResourceType: "MonitoringObservation",
		ResourceID:   "patient-summary",
		TenantID:     tenantID,
		Metadata:     map[string]string{"action": "patient_summary"},
	}); err != nil {
		h.logger.Error("audit write", slog.Any("error", err))
	}

	writeJSON(w, http.StatusOK, PatientVitalSummary{
		PatientNHI:   nhi,
		GeneratedAt:  time.Now().UTC(),
		LatestValues: latestValues,
	})
}

// validateDeviceRegister checks required device registration fields.
func validateDeviceRegister(req *deviceRegisterRequest) error {
	if req.PatientNHI == "" {
		return fmt.Errorf("patientNhi is required")
	}
	if req.DeviceType == "" {
		return fmt.Errorf("deviceType is required")
	}
	validTypes := map[DeviceType]bool{
		DeviceTypeBPMonitor:     true,
		DeviceTypePulseOximeter: true,
		DeviceTypeGlucoseMeter:  true,
		DeviceTypeScale:         true,
		DeviceTypeThermometer:   true,
		DeviceTypePeakFlowMeter: true,
		DeviceTypeSpirometer:    true,
	}
	if !validTypes[req.DeviceType] {
		return fmt.Errorf("unknown deviceType %q", req.DeviceType)
	}
	return nil
}

// validateObservationSubmit checks required observation fields.
func validateObservationSubmit(req *observationSubmitRequest) error {
	if req.PatientNHI == "" {
		return fmt.Errorf("patientNhi is required")
	}
	if req.LOINCCode == "" {
		return fmt.Errorf("loincCode is required")
	}
	if _, ok := knownLoincCodes[req.LOINCCode]; !ok {
		return fmt.Errorf("unsupported loincCode %q — see supported codes documentation", req.LOINCCode)
	}
	if req.ValueQuantity == nil && req.ValueString == "" {
		return fmt.Errorf("either valueQuantity or valueString is required")
	}
	if req.EffectiveAt.IsZero() {
		req.EffectiveAt = time.Now().UTC()
	}
	if req.Source == "" {
		req.Source = "device"
	}
	return nil
}

// insertDevice persists a new monitoring device.
func (h *MonitoringHandler) insertDevice(ctx context.Context, req deviceRegisterRequest, tenantID, encNHI string) (Device, error) {
	var d Device
	var encNHIOut string
	err := h.pool.QueryRow(ctx,
		`INSERT INTO monitoring_devices
		   (patient_nhi, device_type, manufacturer, model, serial_number, tenant_id)
		 VALUES
		   (@patient_nhi, @device_type, @manufacturer, @model, @serial_number, @tenant_id)
		 RETURNING id, patient_nhi, device_type, manufacturer, model, serial_number,
		           status, tenant_id, registered_at, updated_at`,
		db.NamedArgs{
			"patient_nhi":   encNHI,
			"device_type":   req.DeviceType,
			"manufacturer":  req.Manufacturer,
			"model":         req.Model,
			"serial_number": req.SerialNumber,
			"tenant_id":     tenantID,
		},
	).Scan(
		&d.ID, &encNHIOut, &d.DeviceType, &d.Manufacturer, &d.Model, &d.SerialNumber,
		&d.Status, &d.TenantID, &d.RegisteredAt, &d.UpdatedAt,
	)
	if err != nil {
		return Device{}, fmt.Errorf("insert device: %w", err)
	}
	nhi, err := h.enc.Decrypt(encNHIOut)
	if err != nil {
		return Device{}, fmt.Errorf("decrypt nhi: %w", err)
	}
	d.PatientNHI = nhi
	return d, nil
}

// listDevices queries monitoring devices for a tenant.
func (h *MonitoringHandler) listDevices(ctx context.Context, tenantID, statusFilter string) ([]Device, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_nhi, device_type, manufacturer, model, serial_number,
		        status, tenant_id, registered_at, updated_at
		 FROM monitoring_devices
		 WHERE tenant_id = @tenant_id
		   AND (@status_filter = '' OR status = @status_filter)
		 ORDER BY registered_at DESC
		 LIMIT 500`,
		db.NamedArgs{"tenant_id": tenantID, "status_filter": statusFilter},
	)
	if err != nil {
		return nil, fmt.Errorf("query devices: %w", err)
	}
	defer rows.Close()

	var results []Device
	for rows.Next() {
		var d Device
		var encNHI string
		if err := rows.Scan(
			&d.ID, &encNHI, &d.DeviceType, &d.Manufacturer, &d.Model, &d.SerialNumber,
			&d.Status, &d.TenantID, &d.RegisteredAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}
		nhi, err := h.enc.Decrypt(encNHI)
		if err != nil {
			return nil, fmt.Errorf("decrypt nhi: %w", err)
		}
		d.PatientNHI = nhi
		results = append(results, d)
	}
	return results, rows.Err()
}

// insertObservation persists a monitoring observation.
func (h *MonitoringHandler) insertObservation(ctx context.Context, req observationSubmitRequest, tenantID, encNHI string) (Observation, error) {
	obsType := knownLoincCodes[req.LOINCCode]

	var obs Observation
	var encNHIOut string
	var deviceID *string
	if req.DeviceID != "" {
		deviceID = &req.DeviceID
	}
	err := h.pool.QueryRow(ctx,
		`INSERT INTO monitoring_observations
		   (patient_nhi, device_id, loinc_code, observation_type,
		    value_quantity, value_unit, value_string, effective_at, source, tenant_id)
		 VALUES
		   (@patient_nhi, @device_id, @loinc_code, @observation_type,
		    @value_quantity, @value_unit, @value_string, @effective_at, @source, @tenant_id)
		 RETURNING id, patient_nhi, device_id, loinc_code, observation_type,
		           value_quantity, value_unit, value_string, effective_at, source,
		           tenant_id, created_at`,
		db.NamedArgs{
			"patient_nhi":      encNHI,
			"device_id":        deviceID,
			"loinc_code":       req.LOINCCode,
			"observation_type": obsType,
			"value_quantity":   req.ValueQuantity,
			"value_unit":       req.ValueUnit,
			"value_string":     req.ValueString,
			"effective_at":     req.EffectiveAt,
			"source":           req.Source,
			"tenant_id":        tenantID,
		},
	).Scan(
		&obs.ID, &encNHIOut, &deviceID, &obs.LOINCCode, &obs.ObservationType,
		&obs.ValueQuantity, &obs.ValueUnit, &obs.ValueString, &obs.EffectiveAt, &obs.Source,
		&obs.TenantID, &obs.CreatedAt,
	)
	if err != nil {
		return Observation{}, fmt.Errorf("insert observation: %w", err)
	}
	nhi, err := h.enc.Decrypt(encNHIOut)
	if err != nil {
		return Observation{}, fmt.Errorf("decrypt nhi: %w", err)
	}
	obs.PatientNHI = nhi
	if deviceID != nil {
		obs.DeviceID = *deviceID
	}
	return obs, nil
}

// listObservations queries observations with optional LOINC and date filters.
func (h *MonitoringHandler) listObservations(ctx context.Context, tenantID, loincFilter, fromFilter, toFilter string) ([]Observation, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT id, patient_nhi, device_id, loinc_code, observation_type,
		        value_quantity, value_unit, value_string, effective_at, source,
		        tenant_id, created_at
		 FROM monitoring_observations
		 WHERE tenant_id = @tenant_id
		   AND (@loinc_filter = '' OR loinc_code = @loinc_filter)
		   AND (@from_filter  = '' OR effective_at >= @from_filter::timestamptz)
		   AND (@to_filter    = '' OR effective_at <= @to_filter::timestamptz)
		 ORDER BY effective_at DESC
		 LIMIT 500`,
		db.NamedArgs{
			"tenant_id":    tenantID,
			"loinc_filter": loincFilter,
			"from_filter":  fromFilter,
			"to_filter":    toFilter,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("query observations: %w", err)
	}
	defer rows.Close()

	var results []Observation
	for rows.Next() {
		obs, err := h.scanObservation(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan observation: %w", err)
		}
		results = append(results, obs)
	}
	return results, rows.Err()
}

// getObservationByID retrieves a single observation with tenant isolation.
func (h *MonitoringHandler) getObservationByID(ctx context.Context, id, tenantID string) (Observation, error) {
	obs, err := h.scanObservation(h.pool.QueryRow(ctx,
		`SELECT id, patient_nhi, device_id, loinc_code, observation_type,
		        value_quantity, value_unit, value_string, effective_at, source,
		        tenant_id, created_at
		 FROM monitoring_observations
		 WHERE id = @id AND tenant_id = @tenant_id`,
		db.NamedArgs{"id": id, "tenant_id": tenantID},
	).Scan)
	if err != nil {
		if db.IsNoRows(err) {
			return Observation{}, errNotFound
		}
		return Observation{}, fmt.Errorf("get observation by id: %w", err)
	}
	return obs, nil
}

// latestObservationsByNHI returns the most recent observation per LOINC code for the patient.
func (h *MonitoringHandler) latestObservationsByNHI(ctx context.Context, tenantID, encNHI, plainNHI string) (map[string]Observation, error) {
	rows, err := h.pool.Query(ctx,
		`SELECT DISTINCT ON (loinc_code)
		        id, patient_nhi, device_id, loinc_code, observation_type,
		        value_quantity, value_unit, value_string, effective_at, source,
		        tenant_id, created_at
		 FROM monitoring_observations
		 WHERE tenant_id = @tenant_id AND patient_nhi = @patient_nhi
		 ORDER BY loinc_code, effective_at DESC`,
		db.NamedArgs{"tenant_id": tenantID, "patient_nhi": encNHI},
	)
	if err != nil {
		return nil, fmt.Errorf("query latest observations: %w", err)
	}
	defer rows.Close()

	result := make(map[string]Observation)
	for rows.Next() {
		obs, err := h.scanObservation(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan latest observation: %w", err)
		}
		obs.PatientNHI = plainNHI
		result[obs.LOINCCode] = obs
	}
	return result, rows.Err()
}

// scanObservation populates an Observation from a row scan function.
func (h *MonitoringHandler) scanObservation(scan func(...any) error) (Observation, error) {
	var obs Observation
	var encNHI string
	var deviceID *string
	if err := scan(
		&obs.ID, &encNHI, &deviceID, &obs.LOINCCode, &obs.ObservationType,
		&obs.ValueQuantity, &obs.ValueUnit, &obs.ValueString, &obs.EffectiveAt, &obs.Source,
		&obs.TenantID, &obs.CreatedAt,
	); err != nil {
		return Observation{}, err
	}
	nhi, err := h.enc.Decrypt(encNHI)
	if err != nil {
		return Observation{}, fmt.Errorf("decrypt nhi: %w", err)
	}
	obs.PatientNHI = nhi
	if deviceID != nil {
		obs.DeviceID = *deviceID
	}
	return obs, nil
}
