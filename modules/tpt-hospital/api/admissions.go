// Package api — AdmissionsHandler is split across:
//   - admissions_types.go   — AdmissionStatus, AdmissionType, DischargeDestination constants; Admission, DischargeSummary, request structs
//   - admissions_handler.go — AdmissionsHandler struct + List, Create, Get, Update, Discharge, Transfer, GetDischargeSummary, CreateDischargeSummary handlers
//   - admissions_query.go   — DB helpers (list, get, insert, update, discharge admissions; get/insert discharge summaries), dbRow interface, scanAdmissionRow
package api
