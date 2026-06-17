// Package api — EmergencyHandler is split across:
//   - emergency_types.go   — IncidentType, IncidentStatus, CIMSRole, ResourceRequestStatus constants; all public and request structs
//   - emergency_handler.go — EmergencyHandler struct + all HTTP handler methods
//   - emergency_query.go   — DB helpers, event publishing, and scan helpers
package api
