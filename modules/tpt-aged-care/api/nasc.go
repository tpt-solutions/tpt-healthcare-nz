// Package api — NASC handler is split across:
//   - nasc_types.go         — NASCReferralStatus, SupportNeedsLevel, ServicePlanStatus constants; public and internal record structs
//   - nasc_handler.go       — NASCHandler struct + referral handler methods (List, Get, Create, Update, Complete)
//   - nasc_service_plans.go — service plan handler methods + DB helpers (scan, decrypt, referralToResponse, validNeedsLevel)
package api
