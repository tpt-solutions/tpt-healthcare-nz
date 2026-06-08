package api

import "net/http"

// ProtocolCategory groups chemotherapy protocols by clinical domain.
type ProtocolCategory string

const (
	ProtocolCategoryBreast      ProtocolCategory = "breast"
	ProtocolCategoryLung        ProtocolCategory = "lung"
	ProtocolCategoryColorectal  ProtocolCategory = "colorectal"
	ProtocolCategoryLymphoma    ProtocolCategory = "lymphoma"
	ProtocolCategoryLeukemia    ProtocolCategory = "leukemia"
	ProtocolCategoryGynae       ProtocolCategory = "gynaecological"
	ProtocolCategoryUrological  ProtocolCategory = "urological"
	ProtocolCategoryUpperGI     ProtocolCategory = "upper-gi"
	ProtocolCategoryHeadNeck    ProtocolCategory = "head-and-neck"
	ProtocolCategoryOther       ProtocolCategory = "other"
)

// ProtocolStatus controls whether a protocol is available for assignment.
type ProtocolStatus string

const (
	ProtocolStatusActive     ProtocolStatus = "active"
	ProtocolStatusDraft      ProtocolStatus = "draft"
	ProtocolStatusRetired    ProtocolStatus = "retired"
	ProtocolStatusSuperseded ProtocolStatus = "superseded"
)

// DrugRoute is the administration route for a chemotherapy drug.
type DrugRoute string

const (
	DrugRouteIV    DrugRoute = "iv"    // intravenous
	DrugRouteIVCI  DrugRoute = "ivci"  // IV continuous infusion
	DrugRouteSC    DrugRoute = "sc"    // subcutaneous
	DrugRouteIM    DrugRoute = "im"    // intramuscular
	DrugRouteOral  DrugRoute = "oral"
	DrugRouteIT    DrugRoute = "it"    // intrathecal
	DrugRouteInhaled DrugRoute = "inhaled"
)

// PatientProtocolStatus tracks the assignment of a protocol to a patient.
type PatientProtocolStatus string

const (
	PatientProtocolStatusPlanned    PatientProtocolStatus = "planned"
	PatientProtocolStatusActive     PatientProtocolStatus = "active"
	PatientProtocolStatusCompleted  PatientProtocolStatus = "completed"
	PatientProtocolStatusDiscontinued PatientProtocolStatus = "discontinued"
	PatientProtocolStatusModified   PatientProtocolStatus = "modified"
)

// protocolHandler manages the chemotherapy protocol library and patient protocol assignments.
// Built-in protocols include: CHOP, R-CHOP, CHOP-14, FOLFOX, FOLFIRI, FOLFOXIRI,
// ICON6, ICON8, AC, AC-T, EC, ECF, FLOT, CAPOX, CAPIRI, BEP, VIP, ICE, DHAP,
// GDP, GemCarbo, GemCis, PCV, TMZ, MVAC, and others.
type protocolHandler struct {
	handlerDeps
}

func (h *protocolHandler) List(w http.ResponseWriter, r *http.Request)            { notImplemented(w, r) }
func (h *protocolHandler) Create(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *protocolHandler) Get(w http.ResponseWriter, r *http.Request)             { notImplemented(w, r) }
func (h *protocolHandler) Update(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *protocolHandler) ListForPatient(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
func (h *protocolHandler) AssignToPatient(w http.ResponseWriter, r *http.Request) { notImplemented(w, r) }
