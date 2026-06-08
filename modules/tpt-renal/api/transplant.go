package api

import "net/http"

// TransplantType identifies the donor source.
type TransplantType string

const (
	TransplantTypeLiving        TransplantType = "living"
	TransplantTypeDeceasedDonor TransplantType = "deceased-donor"
)

// WaitlistStatus tracks the transplant waitlist entry lifecycle.
type WaitlistStatus string

const (
	WaitlistStatusListed      WaitlistStatus = "listed"
	WaitlistStatusOnHold      WaitlistStatus = "on-hold"
	WaitlistStatusTransplanted WaitlistStatus = "transplanted"
	WaitlistStatusRemoved     WaitlistStatus = "removed"
)

// GraftFunction classifies early graft function post-transplant.
type GraftFunction string

const (
	GraftFunctionImmediate         GraftFunction = "immediate"
	GraftFunctionDelayed           GraftFunction = "delayed"
	GraftFunctionPrimaryNonFunction GraftFunction = "primary-non-function"
)

// transplantHandler handles renal transplant waitlist management.
type transplantHandler struct {
	handlerDeps
}

func (h *transplantHandler) List(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *transplantHandler) Register(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *transplantHandler) Get(w http.ResponseWriter, r *http.Request)        { notImplemented(w, r) }
func (h *transplantHandler) Update(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *transplantHandler) Transplant(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *transplantHandler) Hold(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *transplantHandler) Remove(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
