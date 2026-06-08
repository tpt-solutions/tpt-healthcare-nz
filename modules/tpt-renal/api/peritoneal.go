package api

import "net/http"

// PDModality identifies the peritoneal dialysis technique in use.
type PDModality string

const (
	PDModalityAPD  PDModality = "APD"  // automated peritoneal dialysis
	PDModalityCAPD PDModality = "CAPD" // continuous ambulatory peritoneal dialysis
)

// PDEpisodeStatus tracks the lifecycle of a peritoneal dialysis episode.
type PDEpisodeStatus string

const (
	PDEpisodeStatusActive    PDEpisodeStatus = "active"
	PDEpisodeStatusCompleted PDEpisodeStatus = "completed"
	PDEpisodeStatusChanged   PDEpisodeStatus = "changed"  // modality switch
	PDEpisodeStatusCeased    PDEpisodeStatus = "ceased"
)

// pdHandler handles peritoneal dialysis episode management and exchange records.
type pdHandler struct {
	handlerDeps
}

func (h *pdHandler) List(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *pdHandler) Create(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *pdHandler) Get(w http.ResponseWriter, r *http.Request)            { notImplemented(w, r) }
func (h *pdHandler) Update(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *pdHandler) ListExchanges(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *pdHandler) RecordExchange(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
