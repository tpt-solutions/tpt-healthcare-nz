package api

import "net/http"

// CTCAEGrade is the CTCAE v5.0 severity grade for an adverse event (1–5).
// Grade 1 = mild; Grade 2 = moderate; Grade 3 = severe; Grade 4 = life-threatening;
// Grade 5 = death related to adverse event.
type CTCAEGrade int

const (
	CTCAEGrade1 CTCAEGrade = 1 // mild, asymptomatic or mild symptoms
	CTCAEGrade2 CTCAEGrade = 2 // moderate, minimal local/non-invasive intervention
	CTCAEGrade3 CTCAEGrade = 3 // severe, hospitalisation indicated
	CTCAEGrade4 CTCAEGrade = 4 // life-threatening, urgent intervention required
	CTCAEGrade5 CTCAEGrade = 5 // death related to adverse event
)

// CTCAESystem is the organ system or category from CTCAE v5.0.
type CTCAESystem string

const (
	CTCAESystemBlood            CTCAESystem = "blood-lymphatic"
	CTCAESystemCardiac          CTCAESystem = "cardiac"
	CTCAESystemCongenital       CTCAESystem = "congenital-familial"
	CTCAESystemEar              CTCAESystem = "ear-labyrinth"
	CTCAESystemEndocrine        CTCAESystem = "endocrine"
	CTCAESystemEye              CTCAESystem = "eye"
	CTCAESystemGastrointestinal CTCAESystem = "gastrointestinal"
	CTCAESystemGeneral          CTCAESystem = "general-admin-site"
	CTCAESystemHepatobiliary    CTCAESystem = "hepatobiliary"
	CTCAESystemImmune           CTCAESystem = "immune"
	CTCAESystemInfection        CTCAESystem = "infections-infestations"
	CTCAESystemInjury           CTCAESystem = "injury-poisoning"
	CTCAESystemInvestigations   CTCAESystem = "investigations"
	CTCAESystemMetabolic        CTCAESystem = "metabolism-nutrition"
	CTCAESystemMusculoskeletal  CTCAESystem = "musculoskeletal-connective"
	CTCAESystemNeoplasm         CTCAESystem = "neoplasms"
	CTCAESystemNervous          CTCAESystem = "nervous"
	CTCAESystemPregnancy        CTCAESystem = "pregnancy-puerperium"
	CTCAESystemPsychiatric      CTCAESystem = "psychiatric"
	CTCAESystemRenal            CTCAESystem = "renal-urinary"
	CTCAESystemReproductive     CTCAESystem = "reproductive"
	CTCAESystemRespiratory      CTCAESystem = "respiratory-thoracic"
	CTCAESystemSkin             CTCAESystem = "skin-subcutaneous"
	CTCAESystemSurgical         CTCAESystem = "surgical-medical"
	CTCAESystemVascular         CTCAESystem = "vascular"
)

// ToxicityAssessmentStatus tracks the assessment workflow state.
type ToxicityAssessmentStatus string

const (
	ToxicityAssessmentStatusDraft     ToxicityAssessmentStatus = "draft"
	ToxicityAssessmentStatusFinalised ToxicityAssessmentStatus = "finalised"
	ToxicityAssessmentStatusAmended   ToxicityAssessmentStatus = "amended"
)

// toxicityHandler manages CTCAE v5.0 toxicity assessments and adverse event grading.
// Each assessment is linked to a treatment cycle or immunotherapy episode.
// Grade 3+ events trigger clinical review flags; Grade 4+ events require
// immediate escalation per PHARMAC and NZGG oncology guidelines.
type toxicityHandler struct {
	handlerDeps
}

func (h *toxicityHandler) List(w http.ResponseWriter, r *http.Request)        { notImplemented(w, r) }
func (h *toxicityHandler) Create(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *toxicityHandler) Get(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *toxicityHandler) Update(w http.ResponseWriter, r *http.Request)      { notImplemented(w, r) }
func (h *toxicityHandler) ListEvents(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *toxicityHandler) AddEvent(w http.ResponseWriter, r *http.Request)    { notImplemented(w, r) }
