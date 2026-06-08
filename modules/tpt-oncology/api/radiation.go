package api

import "net/http"

// RadiationIntent classifies whether radiation is curative, adjuvant, or palliative.
type RadiationIntent string

const (
	RadiationIntentCurative   RadiationIntent = "curative"
	RadiationIntentAdjuvant   RadiationIntent = "adjuvant"
	RadiationIntentNeoadjuvant RadiationIntent = "neoadjuvant"
	RadiationIntentPalliative RadiationIntent = "palliative"
	RadiationIntentProphylactic RadiationIntent = "prophylactic"
)

// RadiationModality is the type of radiation used.
type RadiationModality string

const (
	RadiationModalityEBRT     RadiationModality = "ebrt"      // external beam
	RadiationModalityIMRT     RadiationModality = "imrt"      // intensity-modulated
	RadiationModalityVMAT     RadiationModality = "vmat"      // volumetric arc
	RadiationModalitySBRT     RadiationModality = "sbrt"      // stereotactic body
	RadiationModalitySRS      RadiationModality = "srs"       // stereotactic radiosurgery
	RadiationModalityBrachytherapy RadiationModality = "brachytherapy"
	RadiationModalityProton   RadiationModality = "proton"
)

// RadiationReferralStatus tracks the referral through the pathway.
type RadiationReferralStatus string

const (
	RadiationReferralStatusReferred   RadiationReferralStatus = "referred"
	RadiationReferralStatusAccepted   RadiationReferralStatus = "accepted"
	RadiationReferralStatusPlanning   RadiationReferralStatus = "planning"   // CT sim + dosimetry
	RadiationReferralStatusInTreatment RadiationReferralStatus = "in-treatment"
	RadiationReferralStatusCompleted  RadiationReferralStatus = "completed"
	RadiationReferralStatusDeclined   RadiationReferralStatus = "declined"
)

// FractionStatus records whether a planned radiation fraction was delivered.
type FractionStatus string

const (
	FractionStatusPlanned   FractionStatus = "planned"
	FractionStatusDelivered FractionStatus = "delivered"
	FractionStatusMissed    FractionStatus = "missed"
	FractionStatusPostponed FractionStatus = "postponed"
)

// radiationHandler manages referrals to radiation oncology and fraction delivery records.
// Referrals originate from medical or surgical oncology; this handler does not perform
// treatment planning (which is the radiation oncology department's responsibility) but
// provides the liaison record visible to the referring oncologist.
type radiationHandler struct {
	handlerDeps
}

func (h *radiationHandler) List(w http.ResponseWriter, r *http.Request)           { notImplemented(w, r) }
func (h *radiationHandler) Create(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *radiationHandler) Get(w http.ResponseWriter, r *http.Request)            { notImplemented(w, r) }
func (h *radiationHandler) Update(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *radiationHandler) ListFractions(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *radiationHandler) RecordFraction(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
