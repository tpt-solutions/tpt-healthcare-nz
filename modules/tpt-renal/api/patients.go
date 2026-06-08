package api

import "net/http"

// CKDStage represents the KDIGO CKD staging classification.
type CKDStage string

const (
	CKDStage1  CKDStage = "1"
	CKDStage2  CKDStage = "2"
	CKDStage3a CKDStage = "3a"
	CKDStage3b CKDStage = "3b"
	CKDStage4  CKDStage = "4"
	CKDStage5  CKDStage = "5"
	// 5D indicates the patient is on dialysis.
	CKDStage5D CKDStage = "5D"
)

// AlbuminuriaCategory maps to KDIGO albuminuria categories A1-A3.
type AlbuminuriaCategory string

const (
	AlbuminuriaCategoryA1 AlbuminuriaCategory = "A1" // <30 mg/g — normal to mildly increased
	AlbuminuriaCategoryA2 AlbuminuriaCategory = "A2" // 30-300 mg/g — moderately increased
	AlbuminuriaCategoryA3 AlbuminuriaCategory = "A3" // >300 mg/g — severely increased
)

// DialysisModality is the current renal replacement therapy in use.
type DialysisModality string

const (
	DialysisModalityNone          DialysisModality = "none"
	DialysisModalityHaemodialysis DialysisModality = "haemodialysis"
	DialysisModalityPeritoneal    DialysisModality = "peritoneal"
	DialysisModalityTransplant    DialysisModality = "transplant"
)

// RenalPatientStatus tracks the overall care status.
type RenalPatientStatus string

const (
	RenalPatientStatusActive      RenalPatientStatus = "active"
	RenalPatientStatusTransplanted RenalPatientStatus = "transplanted"
	RenalPatientStatusDeceased    RenalPatientStatus = "deceased"
)

// patientHandler handles renal patient registration and CKD staging.
type patientHandler struct {
	handlerDeps
}

func (h *patientHandler) List(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *patientHandler) Register(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *patientHandler) Get(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *patientHandler) Update(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
