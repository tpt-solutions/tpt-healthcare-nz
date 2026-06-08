package api

import "net/http"

// SCBUStatus tracks the clinical status of a SCBU admission.
// SCBU covers neonates requiring intermediate care: prematurity ~32-36 weeks,
// jaundice, feeding difficulties, temperature regulation.
type SCBUStatus string

const (
	SCBUStatusAdmitted        SCBUStatus = "admitted"
	SCBUStatusStepDown        SCBUStatus = "step-down"
	SCBUStatusTransferredNICU SCBUStatus = "transferred-nicu"
	SCBUStatusDischarged      SCBUStatus = "discharged"
)

// SCBUFeedingMethod classifies how the neonate is being fed.
type SCBUFeedingMethod string

const (
	SCBUFeedingBreast  SCBUFeedingMethod = "breast"
	SCBUFeedingNGT     SCBUFeedingMethod = "ngt"
	SCBUFeedingBottle  SCBUFeedingMethod = "bottle"
	SCBUFeedingMixed   SCBUFeedingMethod = "mixed"
)

// scbuHandler manages SCBU admissions and chart entries.
type scbuHandler struct {
	handlerDeps
}

func (h *scbuHandler) List(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *scbuHandler) Create(w http.ResponseWriter, r *http.Request)        { notImplemented(w, r) }
func (h *scbuHandler) Get(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *scbuHandler) Update(w http.ResponseWriter, r *http.Request)        { notImplemented(w, r) }
func (h *scbuHandler) Discharge(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *scbuHandler) TransferNICU(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *scbuHandler) ListChart(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *scbuHandler) AddChartEntry(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
