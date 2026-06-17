// Package api — inpatient pharmacy: medication charts, administration records,
// IV pharmacy, and admission/discharge medication reconciliation.
// NOTE: Community dispensing is handled by tpt-pharmacy. This package covers
// only in-hospital prescribing and administration.
//
// PharmacyHandler is split across:
//   - pharmacy_types.go   — InpatientMedStatus, RouteOfAdministration constants; InpatientMedication, MedAdministrationRecord, MedReconciliation, request structs
//   - pharmacy_handler.go — PharmacyHandler struct + List, Prescribe, Get, Update, Administer, Cease, GetReconciliation, ReconcileMedications handlers
//   - pharmacy_query.go   — DB helpers (list, get, insert, update medications; insertAdminRecord; get/insertReconciliation) and scanMedRow
package api
