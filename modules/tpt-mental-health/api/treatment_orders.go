// Package api — compulsory orders under the Mental Health (Compulsory Assessment
// and Treatment) Act 1992 (MHCAA 1992). TreatmentOrdersHandler is split across:
//   - treatment_orders_types.go   — OrderType, OrderStatus constants; CompulsoryOrder, request, and orderRecord types
//   - treatment_orders_handler.go — TreatmentOrdersHandler struct + List, Get, Create, Update, RecordReview, Revoke handlers
//   - treatment_orders_query.go   — DB helpers (listOrders, insertOrder, updateOrder, etc.) and scan/decrypt functions
//
// Order types:
//
//	CAO           — Compulsory Assessment Order (s11): initial 5-day assessment.
//	CTO-inpatient — Compulsory Treatment Order, inpatient (s30): treatment required as an inpatient.
//	CTO-community — Compulsory Treatment Order, community (s29): conditions on living in the community.
//	SPO           — Special Patient Order (s34): for persons acquitted on insanity grounds or transferred from a penal institution.
package api
