package api

import "net/http"

// HDAccessType identifies the vascular access used for haemodialysis.
type HDAccessType string

const (
	HDAccessTypeAVF  HDAccessType = "AVF"  // arteriovenous fistula
	HDAccessTypeAVG  HDAccessType = "AVG"  // arteriovenous graft
	HDAccessTypeCVC  HDAccessType = "CVC"  // central venous catheter
	HDAccessTypePort HDAccessType = "port" // implanted port
)

// HDSessionStatus tracks the lifecycle of a haemodialysis session.
type HDSessionStatus string

const (
	HDSessionStatusScheduled   HDSessionStatus = "scheduled"
	HDSessionStatusInProgress  HDSessionStatus = "in-progress"
	HDSessionStatusCompleted   HDSessionStatus = "completed"
	HDSessionStatusAborted     HDSessionStatus = "aborted"
)

// hdSessionHandler handles haemodialysis session scheduling and charting.
type hdSessionHandler struct {
	handlerDeps
}

func (h *hdSessionHandler) List(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *hdSessionHandler) Create(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *hdSessionHandler) Get(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *hdSessionHandler) Update(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *hdSessionHandler) Complete(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *hdSessionHandler) Abort(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
