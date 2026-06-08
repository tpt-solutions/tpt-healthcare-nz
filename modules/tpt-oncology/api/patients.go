package api

import "net/http"

// CancerType classifies the primary malignancy for an oncology patient.
type CancerType string

const (
	CancerTypeBreast       CancerType = "breast"
	CancerTypeLung         CancerType = "lung"
	CancerTypeColorectal   CancerType = "colorectal"
	CancerTypeProstate     CancerType = "prostate"
	CancerTypeLymphoma     CancerType = "lymphoma"
	CancerTypeLeukemia     CancerType = "leukemia"
	CancerTypeMelanoma     CancerType = "melanoma"
	CancerTypeGynae        CancerType = "gynaecological"
	CancerTypeHeadNeck     CancerType = "head-and-neck"
	CancerTypeUpperGI      CancerType = "upper-gi"
	CancerTypePancreatic   CancerType = "pancreatic"
	CancerTypeHepatobilary CancerType = "hepatobiliary"
	CancerTypeRenal        CancerType = "renal"
	CancerTypeBladder      CancerType = "bladder"
	CancerTypeSarcoma      CancerType = "sarcoma"
	CancerTypeBrain        CancerType = "brain"
	CancerTypeThyroid      CancerType = "thyroid"
	CancerTypeOther        CancerType = "other"
)

// TNMStage represents the TNM staging classification (AJCC/UICC).
type TNMStage string

const (
	TNMStageI    TNMStage = "I"
	TNMStageII   TNMStage = "II"
	TNMStageIII  TNMStage = "III"
	TNMStageIV   TNMStage = "IV"
	TNMStageIA   TNMStage = "IA"
	TNMStageIB   TNMStage = "IB"
	TNMStageIIA  TNMStage = "IIA"
	TNMStageIIB  TNMStage = "IIB"
	TNMStageIIIC TNMStage = "IIIC"
	TNMStageIVA  TNMStage = "IVA"
	TNMStageIVB  TNMStage = "IVB"
)

// OncologyPatientStatus tracks the overall care status.
type OncologyPatientStatus string

const (
	OncologyPatientStatusActive      OncologyPatientStatus = "active"
	OncologyPatientStatusRemission   OncologyPatientStatus = "remission"
	OncologyPatientStatusSurveillance OncologyPatientStatus = "surveillance"
	OncologyPatientStatusPalliative  OncologyPatientStatus = "palliative"
	OncologyPatientStatusDeclined    OncologyPatientStatus = "declined"
	OncologyPatientStatusDeceased    OncologyPatientStatus = "deceased"
)

// TumourBoardStatus tracks the MDT referral lifecycle.
type TumourBoardStatus string

const (
	TumourBoardStatusPending   TumourBoardStatus = "pending"
	TumourBoardStatusScheduled TumourBoardStatus = "scheduled"
	TumourBoardStatusReviewed  TumourBoardStatus = "reviewed"
	TumourBoardStatusDeferred  TumourBoardStatus = "deferred"
)

// patientHandler handles oncology patient registration and tumour board referrals.
type patientHandler struct {
	handlerDeps
}

func (h *patientHandler) List(w http.ResponseWriter, r *http.Request)                        { notImplemented(w, r) }
func (h *patientHandler) Register(w http.ResponseWriter, r *http.Request)                    { notImplemented(w, r) }
func (h *patientHandler) Get(w http.ResponseWriter, r *http.Request)                         { notImplemented(w, r) }
func (h *patientHandler) Update(w http.ResponseWriter, r *http.Request)                      { notImplemented(w, r) }
func (h *patientHandler) ListTumourBoardReferrals(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
func (h *patientHandler) CreateTumourBoardReferral(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *patientHandler) GetTumourBoardReferral(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
