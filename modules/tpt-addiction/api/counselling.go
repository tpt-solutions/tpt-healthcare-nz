// Package api — counsellingHandler is split across:
//   - counselling_types.go   — CounsellingSession, GroupSession, TreatmentPlan, Goal, RelapseEvent structs; sessionSelectCols, groupSelectCols, planSelectCols consts; scanSession helper
//   - counselling_handler.go — counsellingHandler struct + 13 handlers (ListSessions, CreateSession, GetSession, UpdateSession, ListGroupSessions, CreateGroupSession, GetGroupSession, ListTreatmentPlans, CreateTreatmentPlan, GetTreatmentPlan, UpdateTreatmentPlan, AddGoal, RecordRelapse)
package api
