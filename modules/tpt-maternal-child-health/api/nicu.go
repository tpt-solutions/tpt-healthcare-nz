package api

import "net/http"

// NICUStatus tracks the clinical status of a NICU admission.
type NICUStatus string

const (
	NICUStatusAdmitted          NICUStatus = "admitted"
	NICUStatusStable            NICUStatus = "stable"
	NICUStatusCritical          NICUStatus = "critical"
	NICUStatusDischargePlanning NICUStatus = "discharge-planning"
	NICUStatusDischarged        NICUStatus = "discharged"
	NICUStatusTransferred       NICUStatus = "transferred"
	NICUStatusDeceased          NICUStatus = "deceased"
)

// NICUAdmissionType classifies whether the neonate was born at this facility.
type NICUAdmissionType string

const (
	NICUAdmissionInborn   NICUAdmissionType = "inborn"
	NICUAdmissionOutborn  NICUAdmissionType = "outborn"
	NICUAdmissionTransfer NICUAdmissionType = "transfer"
)

// RespiratorySupport classifies the level of respiratory assistance.
type RespiratorySupport string

const (
	RespSupportNone            RespiratorySupport = "none"
	RespSupportHFNC            RespiratorySupport = "HFNC"
	RespSupportCPAP            RespiratorySupport = "CPAP"
	RespSupportHFOV            RespiratorySupport = "HFOV"
	RespSupportConventionalVent RespiratorySupport = "conventional-vent"
)

// VentMode classifies the ventilation strategy in use.
type VentMode string

const (
	VentModeNoSupport VentMode = "no-support"
	VentModeHFNC      VentMode = "HFNC"
	VentModeCPAP      VentMode = "CPAP"
	VentModeSIMV      VentMode = "SIMV"
	VentModeACPC      VentMode = "AC-PC"
	VentModeHFOV      VentMode = "HFOV"
)

// nicuHandler manages NICU admissions, ventilation charting, and discharge planning.
type nicuHandler struct {
	handlerDeps
}

func (h *nicuHandler) List(w http.ResponseWriter, r *http.Request)               { notImplemented(w, r) }
func (h *nicuHandler) Admit(w http.ResponseWriter, r *http.Request)              { notImplemented(w, r) }
func (h *nicuHandler) Get(w http.ResponseWriter, r *http.Request)                { notImplemented(w, r) }
func (h *nicuHandler) Update(w http.ResponseWriter, r *http.Request)             { notImplemented(w, r) }
func (h *nicuHandler) Discharge(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *nicuHandler) ListVentilation(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
func (h *nicuHandler) RecordVentilation(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *nicuHandler) GetDischargePlan(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *nicuHandler) UpdateDischargePlan(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
