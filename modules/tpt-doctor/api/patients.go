// Package api — PatientsHandler is split across:
//   - patients_types.go   — patientRecord, patientResponse, request structs, enrolmentRequest, transferRequest
//   - patients_handler.go — PatientsHandler struct + List, Get, GetByNHI, Create, Update, GetEnrolment, CreateEnrolment, UpdateEnrolment, TransferEnrolment handlers
//   - patients_query.go   — DB helpers (search, get, persist, update, recordToResponse, checkDisclosureConsent) and validateNHIFormat
package api
