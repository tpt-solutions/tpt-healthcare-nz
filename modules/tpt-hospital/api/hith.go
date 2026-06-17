// Package api — Hospital in the Home (HITH): virtual ward episodes and nursing visits.
// HITH allows patients to receive hospital-level acute care at home, reducing bed
// pressure and improving patient outcomes. Nurses visit daily and monitor via telehealth.
//
// HITHHandler is split across:
//   - hith_types.go   — Vitals, HITHEpisodeStatus, HITHVisitType, HITHEpisode, HITHVisit, request structs
//   - hith_handler.go — HITHHandler struct + 8 handlers (ListEpisodes, CreateEpisode, GetEpisode, UpdateEpisode, AddVisit, ListVisits, UpdateVisit, Discharge)
//   - hith_query.go   — listEpisodes, getEpisodeByID, insertEpisode, updateEpisode, insertVisit, listVisits, updateVisit, scanHITHEpisodeRow, scanHITHVisitRow
package api
