package api

import "net/http"

// PalliativeCarePlanStatus tracks the active state of a palliative care plan.
type PalliativeCarePlanStatus string

const (
	PalliativeCarePlanStatusActive     PalliativeCarePlanStatus = "active"
	PalliativeCarePlanStatusReview     PalliativeCarePlanStatus = "under-review"
	PalliativeCarePlanStatusSuspended  PalliativeCarePlanStatus = "suspended"
	PalliativeCarePlanStatusCompleted  PalliativeCarePlanStatus = "completed"
)

// GoalOfCareCategory classifies each documented patient goal.
type GoalOfCareCategory string

const (
	GoalOfCareCategorySymptomControl  GoalOfCareCategory = "symptom-control"
	GoalOfCareCategoryFunctionality   GoalOfCareCategory = "functionality"
	GoalOfCareCategoryPsychosocial    GoalOfCareCategory = "psychosocial"
	GoalOfCareCategorySpiritual       GoalOfCareCategory = "spiritual"
	GoalOfCareCategoryFamilySupport   GoalOfCareCategory = "family-support"
	GoalOfCareCategoryResuscitation   GoalOfCareCategory = "resuscitation"   // DNAR/DNACPR status
	GoalOfCareCategoryPreferredPlace  GoalOfCareCategory = "preferred-place" // home/hospice/hospital
	GoalOfCareCategoryAdvanceCare     GoalOfCareCategory = "advance-care"    // ACP linkage
)

// GoalStatus tracks progress against an individual goal of care.
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "active"
	GoalStatusAchieved  GoalStatus = "achieved"
	GoalStatusCancelled GoalStatus = "cancelled"
)

// SymptomName covers the common oncology palliative symptom burden domains.
type SymptomName string

const (
	SymptomPain          SymptomName = "pain"
	SymptomNausea        SymptomName = "nausea"
	SymptomVomiting      SymptomName = "vomiting"
	SymptomFatigue       SymptomName = "fatigue"
	SymptomAnorexia      SymptomName = "anorexia"
	SymptomConstipation  SymptomName = "constipation"
	SymptomDyspnoea      SymptomName = "dyspnoea"
	SymptomAnxiety       SymptomName = "anxiety"
	SymptomDepression    SymptomName = "depression"
	SymptomSleepDisturbance SymptomName = "sleep-disturbance"
	SymptomDelirium      SymptomName = "delirium"
	SymptomMucositis     SymptomName = "mucositis"
	SymptomPeripheralNeuropathy SymptomName = "peripheral-neuropathy"
	SymptomOther         SymptomName = "other"
)

// palliativeHandler manages palliative oncology care plans, goals of care, and
// symptom burden assessments. Plans follow the NZ Palliative Care Strategy and
// reference Te Ara Whakapiri (Pathway for Last Days of Life) for terminal phase.
// Advance Care Plans (ACPs) are referenced by ID but stored in the core consent
// module.
type palliativeHandler struct {
	handlerDeps
}

func (h *palliativeHandler) List(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *palliativeHandler) Create(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *palliativeHandler) Get(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *palliativeHandler) Update(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *palliativeHandler) ListGoals(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
func (h *palliativeHandler) AddGoal(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *palliativeHandler) UpdateGoal(w http.ResponseWriter, r *http.Request)   { notImplemented(w, r) }
func (h *palliativeHandler) ListSymptoms(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
func (h *palliativeHandler) RecordSymptom(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
