// Package api — OutpatientHandler is split across:
//   - outpatient_types.go   — OutpatientClinic, OutpatientAppointmentStatus, OutpatientAppointment, WaitlistPriority, WaitlistEntry, request structs
//   - outpatient_handler.go — OutpatientHandler struct + 10 handlers (ListClinics, GetClinic, ListAppointments, BookAppointment, UpdateAppointment, Attend, ListWaitlist, AddToWaitlist, UpdateWaitlistEntry, RemoveFromWaitlist)
//   - outpatient_query.go   — listClinics, getClinicByID, listAppointments, getAppointmentByID, insertAppointment, updateAppointment, listWaitlist, insertWaitlistEntry, updateWaitlistEntry, deleteWaitlistEntry, scanOPApptRow
package api
