package api

import "net/http"

// OedemaSeverity classifies peripheral oedema on clinical assessment.
type OedemaSeverity string

const (
	OedemaSeverityNone     OedemaSeverity = "none"
	OedemaSeverityMild     OedemaSeverity = "mild"
	OedemaSeverityModerate OedemaSeverity = "moderate"
	OedemaSeveritySevere   OedemaSeverity = "severe"
)

// fluidBalanceHandler handles fluid balance recording and dry-weight tracking.
type fluidBalanceHandler struct {
	handlerDeps
}

func (h *fluidBalanceHandler) List(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *fluidBalanceHandler) Record(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *fluidBalanceHandler) Get(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
