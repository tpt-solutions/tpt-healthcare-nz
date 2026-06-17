// Package api — physio and ACC handlers are split across:
//   - physio_handler.go          — PhysioHandler struct, helpers (requireAPC, checkConsent, parsePagination)
//   - physio_treatment_plans.go  — treatment plan handler methods
//   - physio_session_notes.go    — session note and outcome measure handler methods
//   - acc_handler.go             — ACCHandler struct + claim, review, charge-code methods
//   - acc_sessions.go            — session creation, eligibility check, createSessionTx
package api
