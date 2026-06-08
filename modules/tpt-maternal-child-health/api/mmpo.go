package api

import "net/http"

// MMPOClaimType identifies the service category under the LMC Schedule of Payments.
type MMPOClaimType string

const (
	MMPOClaimBooking               MMPOClaimType = "booking"
	MMPOClaimAntenatalVisit        MMPOClaimType = "antenatal-visit"
	MMPOClaimIntrapartumPrimary    MMPOClaimType = "intrapartum-primary"
	MMPOClaimIntrapartumSecondary  MMPOClaimType = "intrapartum-secondary"
	MMPOClaimPostnatalVisit        MMPOClaimType = "postnatal-visit"
	MMPOClaimOnCall                MMPOClaimType = "on-call"
	MMPOClaimRuralPremium          MMPOClaimType = "rural-premium"
	MMPOClaimOther                 MMPOClaimType = "other"
)

// MMPOClaimStatus tracks the lifecycle of an MMPO funding claim.
type MMPOClaimStatus string

const (
	MMPOClaimStatusDraft     MMPOClaimStatus = "draft"
	MMPOClaimStatusSubmitted MMPOClaimStatus = "submitted"
	MMPOClaimStatusAccepted  MMPOClaimStatus = "accepted"
	MMPOClaimStatusRejected  MMPOClaimStatus = "rejected"
	MMPOClaimStatusPaid      MMPOClaimStatus = "paid"
	MMPOClaimStatusWithdrawn MMPOClaimStatus = "withdrawn"
)

// mmpoHandler manages MMPO LMC funding claims.
type mmpoHandler struct {
	handlerDeps
}

func (h *mmpoHandler) List(w http.ResponseWriter, r *http.Request)     { notImplemented(w, r) }
func (h *mmpoHandler) Create(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *mmpoHandler) Get(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *mmpoHandler) Update(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *mmpoHandler) Submit(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *mmpoHandler) Withdraw(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
