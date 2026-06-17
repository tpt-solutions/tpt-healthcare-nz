// Package api — PrescriptionsHandler is split across:
//   - prescriptions_types.go   — PrescriptionStatus, Dosage, Prescription, and request/response structs
//   - prescriptions_handler.go — PrescriptionsHandler struct + List, Create, Get, Update, Print, ReportADE, Dispatch handlers
//   - prescriptions_query.go   — DB helpers (list, get, insert, update) and scanPrescription
package api
