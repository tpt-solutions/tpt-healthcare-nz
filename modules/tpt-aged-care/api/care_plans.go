// Package api — CarePlansHandler is split across:
//   - care_plans_types.go   — CarePlanType, CarePlanStatus, GoalStatus constants; CareGoal, CareIntervention, CarePlan, carePlanRecord, request structs
//   - care_plans_handler.go — CarePlansHandler struct + List, Get, Create, Update, RecordReview, AddGoal, UpdateGoal handlers
//   - care_plans_query.go   — getByID, decrypt, scanCarePlan, validPlanType
package api
