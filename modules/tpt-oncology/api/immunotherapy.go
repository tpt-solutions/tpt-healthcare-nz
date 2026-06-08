package api

import "net/http"

// ImmunotherapyClass classifies the type of immunotherapy or targeted agent.
type ImmunotherapyClass string

const (
	// Immune checkpoint inhibitors
	ImmunoClassPD1Inhibitor  ImmunotherapyClass = "pd1-inhibitor"   // pembrolizumab, nivolumab
	ImmunoClassPDL1Inhibitor ImmunotherapyClass = "pdl1-inhibitor"  // atezolizumab, durvalumab
	ImmunoClassCTLA4Inhibitor ImmunotherapyClass = "ctla4-inhibitor" // ipilimumab

	// Targeted small molecules
	ImmunoClassTKI       ImmunotherapyClass = "tki"        // tyrosine kinase inhibitor
	ImmunoClassCDK46     ImmunotherapyClass = "cdk4-6"     // CDK 4/6 inhibitor
	ImmunoClassPARP      ImmunotherapyClass = "parp"       // PARP inhibitor
	ImmunoClassBCLABL    ImmunotherapyClass = "bcl-abl"   // imatinib, dasatinib
	ImmunoClassBRAF      ImmunotherapyClass = "braf"       // vemurafenib, dabrafenib
	ImmunoClassMEK       ImmunotherapyClass = "mek"        // trametinib, cobimetinib
	ImmunoClassALK       ImmunotherapyClass = "alk"        // crizotinib, alectinib
	ImmunoClassEGFR      ImmunotherapyClass = "egfr"       // erlotinib, osimertinib
	ImmunoClassHER2      ImmunotherapyClass = "her2"       // trastuzumab, pertuzumab
	ImmunoClassVEGF      ImmunotherapyClass = "vegf"       // bevacizumab, sunitinib
	ImmunoClassMTOR      ImmunotherapyClass = "mtor"       // everolimus, temsirolimus
	ImmunoClassHedgehog  ImmunotherapyClass = "hedgehog"   // vismodegib

	// Cell-based
	ImmunoClassCARTCell  ImmunotherapyClass = "car-t"      // CAR-T cell therapy
	ImmunoClassBispecific ImmunotherapyClass = "bispecific" // blinatumomab

	ImmunoClassOther ImmunotherapyClass = "other"
)

// ImmunotherapyStatus tracks the episode lifecycle.
type ImmunotherapyStatus string

const (
	ImmunotherapyStatusActive      ImmunotherapyStatus = "active"
	ImmunotherapyStatusOnHold      ImmunotherapyStatus = "on-hold"
	ImmunotherapyStatusCompleted   ImmunotherapyStatus = "completed"
	ImmunotherapyStatusDiscontinued ImmunotherapyStatus = "discontinued"
)

// ImmunotherapyHoldReason documents why an episode was paused.
type ImmunotherapyHoldReason string

const (
	HoldReasonImmuneToxicity    ImmunotherapyHoldReason = "immune-toxicity"
	HoldReasonInfection         ImmunotherapyHoldReason = "infection"
	HoldReasonSurgery           ImmunotherapyHoldReason = "surgery"
	HoldReasonPatientRequest    ImmunotherapyHoldReason = "patient-request"
	HoldReasonClinicalReview    ImmunotherapyHoldReason = "clinical-review"
	HoldReasonSupply            ImmunotherapyHoldReason = "supply"
)

// immunotherapyHandler manages immunotherapy and targeted therapy treatment episodes.
// irAE (immune-related adverse events) monitoring links to toxicity assessments.
type immunotherapyHandler struct {
	handlerDeps
}

func (h *immunotherapyHandler) List(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *immunotherapyHandler) Create(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *immunotherapyHandler) Get(w http.ResponseWriter, r *http.Request)          { notImplemented(w, r) }
func (h *immunotherapyHandler) Update(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *immunotherapyHandler) Hold(w http.ResponseWriter, r *http.Request)         { notImplemented(w, r) }
func (h *immunotherapyHandler) Resume(w http.ResponseWriter, r *http.Request)       { notImplemented(w, r) }
func (h *immunotherapyHandler) Discontinue(w http.ResponseWriter, r *http.Request)  { notImplemented(w, r) }
