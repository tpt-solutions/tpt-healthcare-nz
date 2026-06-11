# tpt-healthcare — Task Checklist

## Infrastructure & Scaffolding
- [x] Initialise `tpt-healthcare` Git repository
- [x] Create `go.work` linking `core/`, `interop/`, all `modules/`
- [x] Create `pnpm-workspace.yaml` linking `apps/` and `packages/`
- [x] Create root `Makefile` (`make dev`, `make test`, `make build`, `make install`)
- [x] Write `CLAUDE.md` with codebase conventions
- [x] Write `CONTRIBUTING.md`
- [x] Write `SECURITY.md` (vulnerability disclosure policy)
- [x] Create `deploy/docker-compose.dev.yml` (PostgreSQL + Redis + interop)

## core/ — Shared Kernel
- [x] `core/go.mod` (`github.com/PhillipC05/tpt-healthcare/core`, go 1.22)
- [x] `core/db/connect.go` — pgx connection pool
- [x] `core/db/migrate/` — embedded SQL migration runner (mirror tpt-identity pattern)
- [x] `core/db/migrate/001_fhir_resources.sql`
- [x] `core/db/migrate/002_audit_events.sql`
- [x] `core/audit/trail.go` — synchronous audit write (port from tpt-doctor `packages/audit-log/`)
- [x] `core/encryption/` — AES-256-GCM field encryption (port from tpt-doctor `packages/encryption/`)
- [x] `core/middleware/` — rate limit, CORS, tenant extraction, audit wrapping
- [x] `core/auth/provider.go` — AuthProvider interface + Principal struct
- [x] `core/auth/auth0/` — Auth0 OIDC validation (port from tpt-doctor `packages/auth/`)
- [x] `core/auth/jwt/` — Standalone Ed25519 JWT + TOTP
- [x] `core/auth/oidc/` — tpt-identity OIDC client
- [x] `core/events/` — Internal domain event bus
- [x] `core/consent/` — Consent management (HIPC Rule 10/11)
- [x] `core/billing/` — Shared billing primitives

## core/ — NZ National Integrations
- [x] `core/nhi/` — NHI FHIR API client (port from tpt-doctor country-profiles NZ service)
  - [x] NHI format validation (ABC12D checksum pattern)
  - [x] `GET /Patient/{nhi}` and `$match` operations
  - [x] SMART on FHIR bearer token handling
- [x] `core/nes/` — NES client (enrol, update, transfer, status)
- [x] `core/acc/` — ACC FHIR claim lodgement and ClaimResponse polling (port from tpt-doctor MOH claiming)
- [x] `core/hpi/` — Health Practitioner Index APC validation, 24h Redis cache
- [x] `core/pharmac/` — PHARMAC formulary + subsidy lookup (port from tpt-doctor prescriptions)

## core/ — FHIR
- [x] `tools/gen-fhir-types/main.go` — FHIR R5 Go struct generator from `fhir.schema.json`
- [x] `core/fhir/r5/` — Generated Go structs (Patient, Practitioner, Encounter, Observation, Condition, MedicationRequest, DiagnosticReport, ServiceRequest, Immunization, Claim, ClaimResponse, ImagingStudy, Subscription, SubscriptionTopic)
- [x] `core/fhir/r4/` — Minimal R4 types for NHI/NES API compat
- [x] `core/fhir/translate/` — R4↔R5 translators for Patient and Practitioner
- [x] `core/repo/store.go` — FHIR repository interface
- [x] `core/repo/patient.go`, `observation.go`, `encounter.go` etc.
- [x] `core/repo/search.go` — FHIR search parameter engine over PostgreSQL JSONB

## core/ — Supporting Services
- [x] `core/hl7/` — HL7 v2 parser: ORU^R01, ADT^A01/A08, ORM^O01
  - [x] NZ lab Z-segment handlers (Labtests Auckland, Healthscope, Southern Community Labs)
  - [x] MLLP TCP listener
- [x] `core/terminology/snomed.go` — SNOMED CT NZ Edition RF2 loader
- [x] `core/terminology/loinc.go` — LOINC CSV loader
- [x] `core/terminology/icd10.go` — ICD-10-AM loader
- [x] `core/terminology/nzmt.go` — NZ Medicines Terminology (NZULM)
- [x] `core/subscription/engine.go` — FHIR R5 subscription engine (Redis pub/sub)
  - [x] rest-hook channel
  - [x] websocket channel
  - [x] email channel

## tpt-health-interop (Milestone 1)
- [x] `interop/go.mod`
- [x] `interop/cmd/tpt-health-interop/main.go` — Cobra root (serve, migrate, validate)
- [x] `interop/api/server.go` — HTTP server, middleware chain (mirror tpt-identity api/server.go)
- [x] `interop/api/fhir.go` — FHIR REST API (R5 + R4 compat)
- [x] `interop/api/nhi.go` — NHI lookup endpoint
- [x] `interop/api/terminology.go` — Terminology service endpoints
- [x] `interop/api/subscription.go` — Subscription management
- [x] Tests: NHI lookup (UAT), R4↔R5 translation, audit trail
  - [x] FHIR CRUD round-trip (`fhir_test.go`: Create, Read, NotFound, Metadata)
  - [x] NHI format validation + nil-client tests (`nhi_test.go`)
  - [x] R4↔R5 round-trip + metadata version tests (`translate_test.go`)
  - [x] Audit trail mock-recording tests (`audit_test.go`)
- [x] `deploy/docker-compose.yml` — Full interop stack

## tpt-doctor (Milestone 2 — port from tpt-doctor TypeScript)
- [x] `modules/tpt-doctor/go.mod`
- [x] `modules/tpt-doctor/cmd/tpt-doctor/main.go` — embeds frontend, migrations, first-run wizard
- [x] First-run wizard (`installer/wizard/index.html`) — practice setup, auth mode selection
- [x] `modules/tpt-doctor/api/server.go` — routes for all GP workflows
- [x] Patient management (NHI lookup, registration, demographics)
- [x] NES enrolment (enrol, update, transfer patients)
- [x] Appointments (scheduling, reminders, calendar)
- [x] EHR / consultation notes (SOAP notes, vitals, medical history)
- [x] e-Prescribing (PHARMAC formulary check, drug interactions, MedicationRequest)
- [x] Referrals (ServiceRequest to specialist)
- [x] ACC claim generation (port from tpt-doctor MOH claiming)
- [x] PHO reporting extracts (capitation, FFS)
- [x] Lab order + results (FHIR DiagnosticReport integration)
- [x] Immunisation records (FHIR Immunization, NIR submission)
- [x] Medical certificates
- [x] Multi-tenant (row-level security per practice)
- [x] After Hours / Urgent Care workflow variant
- [x] Occupational Health workflow variant

## Frontend — apps/ (port from tpt-doctor React/Vite)
- [x] `apps/tpt-clinic/` — clinician-facing app (port from tpt-doctor apps/web/)
- [x] `apps/tpt-portal/` — patient portal (port from tpt-doctor apps/patient-portal/)
- [x] `apps/tpt-admin/` — practice admin + billing dashboard
- [x] `packages/fhir-types/` — @tpt/fhir-types (FHIR R5 + NZ extensions, using @medplum/fhirtypes as base)
- [x] `packages/ui/` — @tpt/ui shared React component library (Tailwind)
- [x] `packages/api-client/` — @tpt/api-client (openapi-typescript generated from Go server spec)
- [x] `packages/nz-codes/` — @tpt/nz-codes (NHI format validation, ACC codes, NZ identifier URIs)

## Installer
- [x] `installer/scripts/install.sh` — curl | bash for Linux/macOS
- [x] `installer/scripts/install.ps1` — PowerShell for Windows
- [x] First-run wizard HTML/CSS/JS (go:embed)
- [x] Systemd service unit file (Linux)
- [x] LaunchAgent plist (macOS)
- [ ] Windows .msi installer (WiX or NSIS)
- [ ] Test: clean Ubuntu 22.04 VM, run installer, verify first-run wizard

## tpt-pharmacy (Milestone 3)
- [x] PHARMAC formulary dispensing
- [x] Prescription receive from GP (FHIR MedicationRequest)
- [x] FHIR MedicationDispense recording
- [x] Schedule 2 drug two-pharmacist check + extended audit
- [x] HSD (Health Survey and Dispensing) reporting
- [x] PHARMAC subsidy claiming

## tpt-immunisation (Milestone 4)
- [x] NIR (National Immunisation Register) FHIR API integration
- [x] Vaccination scheduling (NZ immunisation schedule)
- [x] FHIR Immunization resource recording
- [x] Outbreak tracking and recall management
- [x] NZ childhood immunisation schedule logic

## tpt-mental-health (Milestone 5)
- [x] Extra-sensitive data flag enforcement (HIPC additional protections)
- [x] Mental Health (Compulsory Assessment and Treatment) Act 1992 workflows
- [x] Compulsory treatment order management
- [x] Inpatient + outpatient psychiatric care workflows
- [x] Enhanced consent model for mental health records

## tpt-pathology (Milestone 6)
- [x] MLLP listener for HL7 v2 lab messages
- [x] ORU^R01 → FHIR DiagnosticReport + Observation conversion
- [x] NZ lab-specific Z-segment parsers
- [x] Specimen tracking
- [x] Result notification to requesting GP (FHIR subscription)
- [x] Reference range management

## tpt-radiology (Milestone 7)
- [x] Orthanc DICOM server integration
- [x] DICOMweb (WADO-RS, STOW-RS, QIDO-RS) endpoints
- [x] FHIR ImagingStudy resource management
- [x] Radiology reporting workflow
- [x] Image sharing (referrer access)
- [x] RIS (Radiology Information System) workflows

## PWA + Patient Portal & Virtual Waiting List (Milestone 8)

### PWA Foundation
- [x] `vite-plugin-pwa` + custom service worker added to `apps/tpt-portal`
- [x] `vite-plugin-pwa` + custom service worker added to `apps/tpt-clinic`
- [x] `vite-plugin-pwa` + custom service worker added to `apps/tpt-admin`
- [x] Generate brand icons (192×192, 512×512, 180×180, 72×72, 32×32 PNG) — `tools/gen-icons/`
- [x] Add `workbox-precaching`, `workbox-routing`, `workbox-strategies`, `workbox-expiration` to each app (SW dependencies)

### VAPID Push Notifications
- [x] `core/push/` — VAPID Web Push sender (`github.com/SherClockHolmes/webpush-go`)
- [x] `core/db/migrate/006_queue.sql` — `push_subscriptions` table
- [x] `interop/api/push.go` — `GET /api/v1/push/vapid-key`, `POST /api/v1/push/subscribe`, `DELETE /api/v1/push/subscribe`
- [x] Push permission flow + subscription hook in `apps/tpt-portal/src/hooks/usePushSetup.ts`

### Appointment Reminders
- [x] `core/queue/reminders.go` — background worker: 24h and 1h pre-appointment push reminders
- [x] Extend `appointments` table with `reminder_24h_sent` / `reminder_1h_sent` flags
- [x] Wire reminder worker into `interop` server startup

### Virtual Waiting List — Backend
- [x] `core/db/migrate/006_queue.sql` — `queues`, `queue_entries`, `queue_entry_locations` tables
- [x] `core/queue/model.go` — Queue, QueueEntry, Location domain structs
- [x] `core/queue/repository.go` — Repository interface
- [x] `core/queue/postgres.go` — pgxpool implementation (ephemeral location delete on terminal status)
- [x] `core/queue/service.go` — Business logic + event publishing + VAPID push on "called"
- [x] `core/subscription/bridge.go` — Wire `events.Bus` → `subscription.Engine`
- [x] `interop/api/sse.go` — Patient SSE stream + Staff SSE stream
- [x] `interop/api/queue.go` — Queue CRUD + check-in (by NHI) + call-next + location update
- [x] `core/db/migrate/006_queue.sql` — `fhir_subscriptions` table (replaces in-memory store)
- [x] Update `core/go.mod` for `webpush-go`

### Virtual Waiting List — Patient Portal (tpt-portal)
- [x] `apps/tpt-portal/src/pages/WaitingPage.tsx` — NHI check-in, live queue position, GPS toggle, "called" state
- [x] `apps/tpt-portal/src/pages/BookAppointmentPage.tsx` — Select clinic, date, time slot, confirm
- [x] Extend `DashboardPage.tsx` — health summary card + today's appointment check-in banner
- [x] Update `NavLayout.tsx` — add "Queue / Check-in" and "Book Appointment" nav items

### Virtual Waiting List — Staff App (tpt-clinic)
- [x] `apps/tpt-clinic/src/pages/QueuePage.tsx` — live queue list (SSE) + Leaflet map with patient pins
- [x] Update `AppShell.tsx` — add "Queue" nav item
- [x] `leaflet@^1.9` + `@types/leaflet` added to `tpt-clinic/package.json`

### PWA Security & Offline Resilience
- [x] `packages/offline-store/` — new shared package: AES-256-GCM crypto helpers, IndexedDB schema, background sync queue
- [x] `packages/offline-store/src/pin-context.tsx` — PINContext with inactivity lock, PBKDF2 key derivation, 5-attempt wipe
- [x] `packages/offline-store/src/LockScreen.tsx` — full-screen numeric keypad lock screen (HISO 10064.1 compliant)
- [x] Update `apps/tpt-clinic/src/sw.ts` — local API fallback (`VITE_LOCAL_API`), Background Sync, power-save mode
- [x] Update `apps/tpt-portal/src/sw.ts` — same
- [x] `apps/tpt-clinic/src/hooks/usePowerSave.ts` — Battery Status API → SW power-save signal
- [x] `apps/tpt-portal/src/hooks/usePowerSave.ts` — same
- [x] `apps/tpt-clinic/src/hooks/useOfflineSync.ts` — prefetch today's patients to IndexedDB on login
- [x] `apps/tpt-portal/src/hooks/useOfflineSync.ts` — cache own patient record
- [x] Wrap `apps/tpt-clinic/src/App.tsx` with `PINProvider` (30s inactivity)
- [x] Wrap `apps/tpt-portal/src/App.tsx` with `PINProvider` (2min inactivity)
- [x] `apps/tpt-admin/src/pages/SettingsPage.tsx` — lock timeout config (15s–5min dropdown)
- [x] `tools/gen-icons/gen-icons.mjs` + `tools/gen-icons/package.json` — icon generation script using `sharp`
- [x] Add `make icons` target to `Makefile`
- [x] Run icon generation → 15 PNGs produced across all three apps (`public/icons/*.png`)

## tpt-aged-care (Milestone 9 — post-PWA)
- [x] interRAI assessment tools
- [x] NASC (Needs Assessment Service Coordination)
- [x] Funded hours management
- [x] Residential and home care workflows

## CAM Modules (Milestone 9)
- [x] tpt-acupuncture (ACC claiming, needle site documentation)
- [x] tpt-chiropractic (spinal charting, ACC, X-ray referrals)
- [x] tpt-osteopathy
- [x] tpt-massage (ACC registered, SOAP notes, contraindication screening)
- [x] tpt-counselling (EAP billing, session notes, private practice)
- [x] tpt-naturopathy (supplement/remedy tracking, private pay)
- [x] tpt-tcm (herb dispensing, tongue/pulse diagnosis)
- [x] tpt-nutrition (food diary, meal planning, body composition)

## tpt-hospital (Milestone 10)
- [x] Inpatient management (admission, discharge, transfer — FHIR Encounter)
- [x] Ward management and bed management
- [x] ED triage workflows
- [x] ICU workflows
- [x] Surgical scheduling (theatre booking, FHIR Appointment + Schedule)
- [x] Pre-admission assessment (pre-operative PAC clinic)
- [x] Clinical coding (ICD-10-AM, ACHI)
- [x] Discharge summaries
- [x] Hospital billing (casemix, DRG)
- [x] Inpatient pharmacy (medication charts, IV pharmacy, reconciliation — NOT community dispensing)
- [x] Infection control (HAI surveillance, isolation precautions)
- [x] Hospital outpatient specialist clinics and waitlists
- [x] Hospital in the Home (HITH / virtual ward)

## Hospital Specialist Modules (Milestone 10b — spun out as independent services)
> These were separated from tpt-hospital so they can be deployed independently
> (e.g. oncology centres, community dialysis, maternity-led units).

- [x] tpt-oncology
  - [x] Oncology patient registration and tumour board referral
  - [x] Chemotherapy protocol library (ICON, CHOP, FOLFOX, etc.)
  - [x] Treatment cycle scheduling and administration recording
  - [x] Immunotherapy / targeted therapy workflows
  - [x] Side-effect and toxicity grading (CTCAE)
  - [x] Radiation therapy referral integration
  - [x] Palliative oncology pathways
- [x] tpt-renal
  - [x] Renal patient registration and CKD staging
  - [x] Haemodialysis session scheduling and charting (Kt/V, UFR, access)
  - [x] Peritoneal dialysis (APD/CAPD) episode management
  - [x] Renal transplant waitlist management
  - [x] Fluid balance and dry-weight tracking
  - [ ] Dialysis machine integration (future)
- [x] tpt-maternal-child-health
  - [x] LMC (Lead Maternity Carer) registration and case-loading
  - [x] Antenatal care (booking, growth scans, screening)
  - [x] Intrapartum care (birthing suite, partogram, CTG)
  - [x] Postnatal care (discharge, community midwife visits)
  - [x] Birth notification (NBRS — National Baby Record System)
  - [x] Neonatal NICU (≤28 days / <44 weeks corrected): ventilation charting, discharge planning
  - [x] SCBU (Special Care Baby Unit, ~32–36 weeks): step-down from NICU
  - [x] MMPO claiming integration
  - [x] Paediatric inpatient admissions with age/weight-adjusted clinical ranges
  - [x] PICU (Paediatric ICU, children >28 days): respiratory support, TPN, inotropes
  - [x] Growth and developmental milestone tracking
  - [x] Consent and assent documentation (parent/guardian proxy)
  - [x] Child protection flagging and reporting (Children's Act 2014)
  - [x] Well Child Tamariki Ora schedule (Plunket checks neonatal → B4 School Check)
  - [x] B4 School Check including SDQ (Strengths and Difficulties Questionnaire)
- [x] tpt-cardiology
  - [x] Cardiology outpatient clinic and follow-up
  - [x] ECG ordering, interpretation and storage
  - [x] Echocardiography requests and reports
  - [x] Holter / ambulatory BP monitoring
  - [x] Cath lab booking, procedure documentation, post-cath care
  - [x] Cardiac rehabilitation programme management
  - [x] Implantable device management (pacemaker, ICD)
- [x] tpt-rehabilitation
  - [x] Inpatient rehabilitation admission and functional assessment
  - [x] Goal setting (STG/LTG) with therapy discipline tracking (physio, OT, speech)
  - [x] FIM (Functional Independence Measure) scoring
  - [x] Community rehabilitation episodes (post-discharge follow-up)
  - [x] ACC rehabilitation plan management
  - [x] Discharge planning and NASC referral

## Remaining Modules (Post-Hospital)
- [x] tpt-blood-bank (cross-matching, blood product inventory, donor management)
- [x] tpt-dental (FDI tooth charting, ACC, dental-specific workflows)
- [x] tpt-vision (optometry/ophthalmology, prescription management, optical dispensing)
- [x] tpt-allied-health (physio, OT, speech therapy, podiatry)
- [x] tpt-community-health (district nursing, home visits, outreach)
- [x] tpt-addiction (methadone programme, counselling workflows)
- [x] tpt-palliative (hospice, advance care planning, pain protocols)
- [x] tpt-disability (NASC, support plans, funded hours)
- [x] tpt-screening (national programmes, recall systems, results management)
- [x] tpt-epidemiology (disease surveillance, outbreak investigation, public health reporting)
- [x] tpt-telehealth (video consultations, remote monitoring — port Jitsi/WebRTC from tpt-doctor)
- [x] tpt-clinical-trials (protocol management, participant tracking, adverse events)
- [x] tpt-health-billing (ACC claiming, PHARMAC subsidies, health insurance — cross-module)

## Compliance & Security
- [x] `core/breach/` — Privacy Act breach notification workflow (72h to Privacy Commissioner)
- [ ] Penetration testing (before any hosted tier launch)
- [ ] Privacy Impact Assessment documentation
- [ ] Full HIPC compliance audit
- [ ] HPCA scope-of-practice enforcement tested across all practitioner types

## Milestone 11 — Practice Management & Operations

### Resilience & Infrastructure
- [x] `core/outbox/` — transactional outbox (model, repository, River worker)
- [x] `core/resilience/` — circuit breaker (gobreaker) + retry with exponential backoff + jitter
- [x] `core/health/` — provider health aggregator (River job + HTTP endpoint + cache table)
- [x] Add `github.com/riverqueue/river` + `riverpgxv5` + `gobreaker` to `core/go.mod`
- [x] Replace `core/queue/reminders.go` `time.Ticker` with River job (at-least-once, retryable)
- [x] `core/backup/` — WAL archiving orchestration, retention policy enforcement, nightly verify River job
- [x] DB migration `008_resilience.sql` — outbox_messages, river schema, provider_health_status, backup_runs, retention_policy
- [x] pg_cron jobs: audit partition rotation (monthly), retention enforcement (nightly), stats refresh (6h)
- [x] Enable pg_cron extension in `deploy/docker-compose.dev.yml` + `deploy/docker-compose.yml`

### Provider Interfaces
- [x] `core/accounting/` — Provider interface + Xero / QBO / FreshBooks backends
- [x] `core/payroll/` — Provider interface + PayHero / iPayroll / FlexiTime / Datacom backends
- [x] `core/sms/` — Provider interface + MessageBird / BurstSMS / Vonage / Twilio / ClickSend / Notifyre backends
  - [x] Wire SMS into appointment reminders + queue "called" + cold-chain breach alerts
- [x] `core/email/` — Provider interface + SendGrid / Postmark / AWS SES / Mailgun backends
  - [x] Wire email into subscription engine `dispatchEmail`, breach notifications, appointment confirmations
- [x] `core/storage/` — Provider interface + S3 (ap-southeast-2) / Azure Blob / MinIO backends (AES-256-GCM pre-upload)
  - [x] Wire storage into consent evidence, radiology attachments, medical cert PDFs, WAL backup uploads
- [x] `core/payment/` — Provider interface + Windcave / Stripe / Paymark backends + webhook handler
  - [x] Wire payments into patient portal invoice payment + reception EFTPOS
- [x] `core/fax/` — Provider interface + Healthlink EDI / eFax backends
  - [x] Wire fax into tpt-doctor referral dispatch + tpt-pathology result delivery
- [x] `core/video/` — Provider interface + Jitsi (self-hosted) / Zoom / Teams backends
  - [x] Wire video into appointment booking (room created on confirmation)

### RBAC & Departments
- [x] `core/rbac/` — Department, RoleAssignment, checker, `RequirePermission` middleware
- [x] Extend `auth.Principal` with `DepartmentIDs []uuid.UUID`; update all three auth providers to inject from DB
- [x] DB migration `007_practice_management.sql` — departments, role_assignments tables (+ all practice tables below)

### Inventory & Accounts
- [x] `core/inventory/` — StockItem, StockMovement, PurchaseOrder, ColdChainLog; low-stock + cold-chain River job
- [x] `core/accounts/` — CostCentre, Budget, BudgetLine, VarianceReport

### tpt-practice Module
- [x] `modules/tpt-practice/` — `go.mod`, Cobra CLI (`serve`, `migrate`), HTTP API server
- [x] Roster API (shifts, on-call; queues timesheet push to payroll via outbox on shift close)
- [x] Room booking API (conflict detection)
- [x] Leave API (approval state machine; syncs to payroll `SubmitLeaveRequest` + `GetLeaveBalance`)
- [x] Inventory API (stock CRUD, PO workflow, cold-chain log)
- [x] Budget API (cost centres, variance report)
- [x] Accounting sync API (outbox status, manual trigger, HealthCheck passthrough)
- [x] Payroll sync API (payslip proxy, leave balance proxy)
- [x] Department management API
- [x] Onboarding wizard state API (per-tenant step progress)
- [x] Add `./modules/tpt-practice` to `go.work`

### Frontend — tpt-admin Expansion
- [x] `OnboardingWizard.tsx` — 7-step wizard (details → departments → staff/roles → accounting → payroll → inventory → launch); shown when `!wizard_complete`; resumable
- [x] `DepartmentsPage.tsx` — department CRUD, parent hierarchy
- [x] `RolesPage.tsx` — role assignment: user → role + optional department
- [x] `RosterPage.tsx` — shift calendar, drag-to-assign, on-call rotation, APC expiry banner
- [x] `RoomsPage.tsx` — room booking grid, conflict detection
- [x] `LeavePage.tsx` — leave requests, approve/decline, calendar overlay, payroll leave balance
- [x] `InvoicesPage.tsx` — full AR lifecycle (draft → issued → overdue → paid), payment plans, aging buckets
- [x] `InventoryPage.tsx` — stock levels, expiry alerts, low-stock indicators, PO list, cold-chain breach log
- [x] `BudgetPage.tsx` — cost-centre selector, monthly actual vs budget variance chart
- [x] `AccountingPage.tsx` — provider connection status, last sync, error log, manual trigger (via IntegrationsPage)
- [x] `PayrollPage.tsx` — provider connection status, payslips, leave balance (via IntegrationsPage)
- [x] Provider settings pages: SMS, Email, Storage, Payment, Fax, Video (via IntegrationsPage)
- [x] Update `NavLayout.tsx` — "Operations" group (Roster, Rooms, Leave, Inventory, Budget) + "Integrations" group
- [x] Backup status widget in `DashboardPage.tsx`

---

## Milestone 12 — NZ Integrations — Tier 2 & Infrastructure Hardening

### ACC Extensions
- [x] `core/acc/schedule.go` — per-discipline treatment codes and session caps (acupuncture, chiropractic, massage, physio schedules differ under the ACC treatment provider schedule)
- [x] `core/acc/purchase_order.go` — PO lifecycle: request approval, session consumption tracking, reconciliation report
- [x] Wire PO management into tpt-acupuncture, tpt-chiropractic, tpt-massage claim submit handlers
- [x] ACC provider registration flow in tpt-admin (verify practice ACC Provider status, store provider number)

### WorkSafe NZ
- [x] `core/worksafe/` — workplace injury claim client (mirrors core/acc/ API shape against api.worksafe.govt.nz)
- [x] Wire into tpt-doctor ACC claims handler as an alternative claim destination for work-related injuries

### Mandatory Regulatory Reporting
- [x] `core/primhd/` — PRIMHD outcomes reporting client (required for all DHB-funded mental health/addiction services)
  - [x] Wire into tpt-counselling session close handler
  - [x] Wire into tpt-addiction discharge handler
- [x] `core/medsafe/` — adverse drug event (ADE) reporting client (mandatory under Medicines Act 1981)
  - [x] Wire into tpt-pharmacy dispensing handler and tpt-doctor prescription handler
- [x] `core/episurv/` — EpiSurv / ESR notifiable disease reporting client
  - [x] Wire into tpt-doctor diagnosis handler for notifiable conditions (measles, TB, COVID, salmonella, etc.)

### Community Pharmacy Dispensing Gateway
- [x] `core/pharmacy-gateway/` — FHIR MedicationRequest dispatch to community pharmacy PMS
  - [x] Fred Dispense connector (`core/pharmacy-gateway/fred/`)
  - [x] Toniq connector (`core/pharmacy-gateway/toniq/`)
  - [x] HL7 v2 RDE^O11 fallback for legacy systems (`core/pharmacy-gateway/hl7v2/`) + `core/hl7/client.go` MLLPClient
- [x] e-Prescription flow: tpt-doctor → pharmacy-gateway → community pharmacy (replaces fax/print for in-network pharmacies)

### Care Coordination
- [x] `core/erms/` — ERMS electronic referral routing (DHB-specific, supplements Healthlink EDI for region-specific workflows)
- [x] `core/msd/` — Community Services Card eligibility check (MSD API) for subsidy verification at reception
- [x] NZBN lookup for practice entity verification (`core/msd/msd.go` — NZBNClient)

### Infrastructure Hardening
- [x] Wire `core/resilience/` (circuit breaker + exponential backoff with jitter) into all five health system clients: NHI, HPI, NES, ACC, PHARMAC via `UseResilience()` method + `core/resilience/transport.go` RoundTripper
- [x] Wire `core/outbox/` River producers into all external API call sites so failed calls are queued and retried — new health system topics added to `core/outbox/model.go`
- [x] `core/health/` — populate health aggregation endpoint with all provider health checks — NHI, HPI, NES, ACC, PHARMAC, PRIMHD, WorkSafe, Medsafe, EpiSurv, ERMS provider types added to `core/health/model.go`
- [x] ACC and PHARMAC: add Redis caching layer — `core/acc/cache.go` (1h TTL for claim/PO status) + `core/pharmac/cache.go` (24h TTL for schedule/medicine lookups)
- [x] FHIR Subscription engine: complete WebSocket hub (`core/subscription/ws.go`) and email dispatch channel implementations (`Engine.SetWSHub`, `Engine.SetEmailSender`)
