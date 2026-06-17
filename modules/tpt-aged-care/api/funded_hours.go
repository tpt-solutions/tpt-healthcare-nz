// Package api — FundedHoursHandler is split across:
//   - funded_hours_types.go   — FundingType, AllocationStatus, TimesheetStatus constants; FundedHoursAllocation, TimesheetEntry, FundedHoursTimesheet, FundedHoursSummary, allocationRecord, timesheetRecord
//   - funded_hours_handler.go — FundedHoursHandler struct + ListAllocations, GetAllocation, CreateAllocation, UpdateAllocation, ListTimesheets, GetTimesheet, CreateTimesheet, ApproveTimesheet, GetSummary handlers
//   - funded_hours_query.go   — getAllocationByID, getTimesheetByID, scanAllocation, scanTimesheet, allocationToResponse, timesheetToResponse, fundedHoursMax
package api
