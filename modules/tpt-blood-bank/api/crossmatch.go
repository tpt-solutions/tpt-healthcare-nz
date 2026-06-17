// Package api — CrossmatchHandler is split across:
//   - crossmatch_types.go   — CrossmatchStatus constants; Crossmatch, request structs; ABOCompatibilityTable, RhDCompatible
//   - crossmatch_handler.go — CrossmatchHandler struct + List, Create, Get, Issue, Transfuse, Cancel, EmergencyRelease handlers; validateProductCompatibility; errIncompatible
//   - crossmatch_query.go   — dbRow interface; DB helpers (list, get, insert, update, getProductByID, updateProductStatus) and scanCrossmatch
package api
