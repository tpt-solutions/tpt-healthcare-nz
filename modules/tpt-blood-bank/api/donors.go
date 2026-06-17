// Package api — DonorsHandler is split across:
//   - donors_types.go   — BloodGroup, DeferralReason, DonorStatus constants; ValidBloodGroups, DeferralDuration maps; Donor, DonationRecord, request structs
//   - donors_handler.go — DonorsHandler struct + List, Create, Get, Update, Defer, Reinstate, DonationHistory, ListEligible handlers
//   - donors_query.go   — DB helpers (listDonors, listEligibleDonors, getDonorByID, insertDonor, updateDonor, getDonations)
package api
