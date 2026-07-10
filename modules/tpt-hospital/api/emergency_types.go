package api

import "time"

// IncidentType classifies the nature of the emergency.
type IncidentType string

const (
	IncidentTypeMCI      IncidentType = "mci"
	IncidentTypeCBRN     IncidentType = "cbrn"
	IncidentTypeFire     IncidentType = "fire"
	IncidentTypeFlood    IncidentType = "flood"
	IncidentTypeCyber    IncidentType = "cyber"
	IncidentTypePandemic IncidentType = "pandemic"
	IncidentTypeOther    IncidentType = "other"
)

// IncidentStatus is the CIMS lifecycle state of an incident.
type IncidentStatus string

const (
	IncidentDeclared  IncidentStatus = "declared"
	IncidentActivated IncidentStatus = "activated"
	IncidentEscalated IncidentStatus = "escalated"
	IncidentStandDown IncidentStatus = "stand_down"
	IncidentClosed    IncidentStatus = "closed"
)

// CIMSRole enumerates the standard NZ CIMS (Coordinated Incident Management System) roles.
type CIMSRole string

const (
	CIMSRoleIC              CIMSRole = "incident_commander"
	CIMSRoleDeputyIC        CIMSRole = "deputy_ic"
	CIMSRoleSafetyOfficer   CIMSRole = "safety_officer"
	CIMSRoleOpsChief        CIMSRole = "operations_chief"
	CIMSRoleLogisticsChief  CIMSRole = "logistics_chief"
	CIMSRolePlanningChief   CIMSRole = "planning_chief"
	CIMSRoleFinanceChief    CIMSRole = "finance_chief"
	CIMSRoleMedicalDirector CIMSRole = "medical_director"
	CIMSRoleLiaison         CIMSRole = "liaison_officer"
	CIMSRolePIO             CIMSRole = "public_info_officer"
	CIMSRoleZoneLeader      CIMSRole = "zone_leader"
)

// EmergencyIncident is the top-level CIMS incident record.
type EmergencyIncident struct {
	ID            string         `json:"id"`
	TenantID      string         `json:"tenantId"`
	Title         string         `json:"title"`
	Type          IncidentType   `json:"type"`
	Status        IncidentStatus `json:"status"`
	Description   string         `json:"description,omitempty"`
	Location      string         `json:"location,omitempty"`
	CBRNAgent     string         `json:"cbrnAgent,omitempty"`
	SurgeLevel    int            `json:"surgeLevel"`
	DeclaredBy    string         `json:"declaredBy"`
	ICPrincipalID string         `json:"icPrincipalId,omitempty"`
	DeclaredAt    time.Time      `json:"declaredAt"`
	ActivatedAt   *time.Time     `json:"activatedAt,omitempty"`
	EscalatedAt   *time.Time     `json:"escalatedAt,omitempty"`
	StandDownAt   *time.Time     `json:"standDownAt,omitempty"`
	ClosedAt      *time.Time     `json:"closedAt,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// IncidentCommandAssignment records who holds a CIMS role during an incident.
type IncidentCommandAssignment struct {
	ID          string     `json:"id"`
	IncidentID  string     `json:"incidentId"`
	TenantID    string     `json:"tenantId"`
	CIMSRole    CIMSRole   `json:"cimsRole"`
	PrincipalID string     `json:"principalId"`
	AssignedBy  string     `json:"assignedBy"`
	AssignedAt  time.Time  `json:"assignedAt"`
	RelievedAt  *time.Time `json:"relievedAt,omitempty"`
}

// IncidentLogEntry is an append-only command-log record (for post-incident debrief).
type IncidentLogEntry struct {
	ID         string    `json:"id"`
	IncidentID string    `json:"incidentId"`
	TenantID   string    `json:"tenantId"`
	AuthorID   string    `json:"authorId"`
	Category   string    `json:"category"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

// ResourceRequestStatus is the lifecycle state of a resource request.
type ResourceRequestStatus string

const (
	ResourceRequested ResourceRequestStatus = "requested"
	ResourceFulfilled ResourceRequestStatus = "fulfilled"
	ResourceCancelled ResourceRequestStatus = "cancelled"
)

// ResourceRequest tracks equipment/staff/supply requests during an incident.
type ResourceRequest struct {
	ID          string                `json:"id"`
	IncidentID  string                `json:"incidentId"`
	TenantID    string                `json:"tenantId"`
	Category    string                `json:"category"`
	Description string                `json:"description"`
	Quantity    int                   `json:"quantity"`
	Status      ResourceRequestStatus `json:"status"`
	Priority    string                `json:"priority"`
	RequestedBy string                `json:"requestedBy"`
	FulfilledBy string                `json:"fulfilledBy,omitempty"`
	Notes       string                `json:"notes,omitempty"`
	RequestedAt time.Time             `json:"requestedAt"`
	FulfilledAt *time.Time            `json:"fulfilledAt,omitempty"`
	UpdatedAt   time.Time             `json:"updatedAt"`
}

type incidentDeclareRequest struct {
	Title       string       `json:"title"`
	Type        IncidentType `json:"type"`
	Description string       `json:"description,omitempty"`
	Location    string       `json:"location,omitempty"`
	CBRNAgent   string       `json:"cbrnAgent,omitempty"`
}

type incidentAssignRoleRequest struct {
	CIMSRole    CIMSRole `json:"cimsRole"`
	PrincipalID string   `json:"principalId"`
}

type incidentLogRequest struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

type resourceRequestCreate struct {
	Category    string `json:"category"`
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	Priority    string `json:"priority"`
	Notes       string `json:"notes,omitempty"`
}

type resourceRequestUpdate struct {
	Status      ResourceRequestStatus `json:"status"`
	FulfilledBy string                `json:"fulfilledBy,omitempty"`
	Notes       string                `json:"notes,omitempty"`
}
