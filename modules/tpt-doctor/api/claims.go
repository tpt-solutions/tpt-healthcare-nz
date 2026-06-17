// Package api — ClaimsHandler is split across:
//   - claims_types.go   — ClaimDestination, ClaimStatus, ACCFormType constants; Claim, claimCreateRequest, claimStatusResponse structs
//   - claims_handler.go — ClaimsHandler struct + List, Create, Get, Submit, Status handlers; lodge/poll helpers; validateClaimCreate, mapACCStatus, mapWorkSafeStatus
//   - claims_query.go   — DB helpers (list, get, insert, reserve, reset, updateAfterSubmit, updateStatus) and scanClaim
package api
