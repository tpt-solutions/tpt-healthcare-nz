# tpt-healthcare — Task Checklist

## Infrastructure & Scaffolding
- [ ] Initialise `tpt-healthcare` Git repository
- [ ] Create `go.work` linking `core/`, `interop/`, all `modules/`
- [ ] Create `pnpm-workspace.yaml` linking `apps/` and `packages/`
- [ ] Create root `Makefile` (`make dev`, `make test`, `make build`, `make install`)
- [ ] Write `CLAUDE.md` with codebase conventions
- [ ] Write `CONTRIBUTING.md`
- [ ] Write `SECURITY.md` (vulnerability disclosure policy)
- [ ] Create `deploy/docker-compose.dev.yml` (PostgreSQL + Redis + interop)

## core/ — Shared Kernel
- [ ] `core/go.mod` (`github.com/PhillipC05/tpt-healthcare/core`, go 1.22)
- [ ] `core/db/connect.go` — pgx connection pool
- [ ] `core/db/migrate/` — embedded SQL migration runner (mirror tpt-identity pattern)
- [ ] `core/db/migrate/001_fhir_resources.sql`
- [ ] `core/db/migrate/002_audit_events.sql`
- [ ] `core/audit/trail.go` — synchronous audit write (port from tpt-doctor `packages/audit-log/`)
- [ ] `core/encryption/` — AES-256-GCM field encryption (port from tpt-doctor `packages/encryption/`)
- [ ] `core/middleware/` — rate limit, CORS, tenant extraction, audit wrapping
- [ ] `core/auth/provider.go` — AuthProvider interface + Principal struct
- [ ] `core/auth/auth0/` — Auth0 OIDC validation (port from tpt-doctor `packages/auth/`)
- [ ] `core/auth/jwt/` — Standalone Ed25519 JWT + TOTP
- [ ] `core/auth/oidc/` — tpt-identity OIDC client
- [ ] `core/events/` — Internal domain event bus
- [ ] `core/consent/` — Consent management (HIPC Rule 10/11)
- [ ] `core/billing/` — Shared billing primitives

## core/ — NZ National Integrations
- [ ] `core/nhi/` — NHI FHIR API client (port from tpt-doctor country-profiles NZ service)
  - [ ] NHI format validation (ABC12D checksum pattern)
  - [ ] `GET /Patient/{nhi}` and `$match` operations
  - [ ] SMART on FHIR bearer token handling
- [ ] `core/nes/` — NES client (enrol, update, transfer, status)
- [ ] `core/acc/` — ACC FHIR claim lodgement and ClaimResponse polling (port from tpt-doctor MOH claiming)
- [ ] `core/hpi/` — Health Practitioner Index APC validation, 24h Redis cache
- [ ] `core/pharmac/` — PHARMAC formulary + subsidy lookup (port from tpt-doctor prescriptions)

## core/ — FHIR
- [ ] `tools/gen-fhir-types/main.go` — FHIR R5 Go struct generator from `fhir.schema.json`
- [ ] `core/fhir/r5/` — Generated Go structs (Patient, Practitioner, Encounter, Observation, Condition, MedicationRequest, DiagnosticReport, ServiceRequest, Immunization, Claim, ClaimResponse, ImagingStudy, Subscription, SubscriptionTopic)
- [ ] `core/fhir/r4/` — Minimal R4 types for NHI/NES API compat
- [ ] `core/fhir/translate/` — R4↔R5 translators for Patient and Practitioner
- [ ] `core/repo/store.go` — FHIR repository interface
- [ ] `core/repo/patient.go`, `observation.go`, `encounter.go` etc.
- [ ] `core/repo/search.go` — FHIR search parameter engine over PostgreSQL JSONB

## core/ — Supporting Services
- [ ] `core/hl7/` — HL7 v2 parser: ORU^R01, ADT^A01/A08, ORM^O01
  - [ ] NZ lab Z-segment handlers (Labtests Auckland, Healthscope, Southern Community Labs)
  - [ ] MLLP TCP listener
- [ ] `core/terminology/snomed.go` — SNOMED CT NZ Edition RF2 loader
- [ ] `core/terminology/loinc.go` — LOINC CSV loader
- [ ] `core/terminology/icd10.go` — ICD-10-AM loader
- [ ] `core/terminology/nzmt.go` — NZ Medicines Terminology (NZULM)
- [ ] `core/subscription/engine.go` — FHIR R5 subscription engine (Redis pub/sub)
  - [ ] rest-hook channel
  - [ ] websocket channel
  - [ ] email channel

## tpt-health-interop (Milestone 1)
- [ ] `interop/go.mod`
- [ ] `interop/cmd/tpt-health-interop/main.go` — Cobra root (serve, migrate, validate)
- [ ] `interop/api/server.go` — HTTP server, middleware chain (mirror tpt-identity api/server.go)
- [ ] `interop/api/fhir.go` — FHIR REST API (R5 + R4 compat)
- [ ] `interop/api/nhi.go` — NHI lookup endpoint
- [ ] `interop/api/terminology.go` — Terminology service endpoints
- [ ] `interop/api/subscription.go` — Subscription management
- [ ] Tests: FHIR CRUD round-trip, NHI lookup (UAT), R4↔R5 translation, audit trail
- [ ] `deploy/docker-compose.yml` — Full interop stack

## tpt-doctor (Milestone 2 — port from tpt-doctor TypeScript)
- [ ] `modules/tpt-doctor/go.mod`
- [ ] `modules/tpt-doctor/cmd/tpt-doctor/main.go` — embeds frontend, migrations, first-run wizard
- [ ] First-run wizard (`installer/wizard/index.html`) — practice setup, auth mode selection
- [ ] `modules/tpt-doctor/api/server.go` — routes for all GP workflows
- [ ] Patient management (NHI lookup, registration, demographics)
- [ ] NES enrolment (enrol, update, transfer patients)
- [ ] Appointments (scheduling, reminders, calendar)
- [ ] EHR / consultation notes (SOAP notes, vitals, medical history)
- [ ] e-Prescribing (PHARMAC formulary check, drug interactions, MedicationRequest)
- [ ] Referrals (ServiceRequest to specialist)
- [ ] ACC claim generation (port from tpt-doctor MOH claiming)
- [ ] PHO reporting extracts (capitation, FFS)
- [ ] Lab order + results (FHIR DiagnosticReport integration)
- [ ] Immunisation records (FHIR Immunization, NIR submission)
- [ ] Medical certificates
- [ ] Multi-tenant (row-level security per practice)
- [ ] After Hours / Urgent Care workflow variant
- [ ] Occupational Health workflow variant

## Frontend — apps/ (port from tpt-doctor React/Vite)
- [ ] `apps/tpt-clinic/` — clinician-facing app (port from tpt-doctor apps/web/)
- [ ] `apps/tpt-portal/` — patient portal (port from tpt-doctor apps/patient-portal/)
- [ ] `apps/tpt-admin/` — practice admin + billing dashboard
- [ ] `packages/fhir-types/` — @tpt/fhir-types (FHIR R5 + NZ extensions, using @medplum/fhirtypes as base)
- [ ] `packages/ui/` — @tpt/ui shared React component library (Tailwind)
- [ ] `packages/api-client/` — @tpt/api-client (openapi-typescript generated from Go server spec)
- [ ] `packages/nz-codes/` — @tpt/nz-codes (NHI format validation, ACC codes, NZ identifier URIs)

## Installer
- [ ] `installer/scripts/install.sh` — curl | bash for Linux/macOS
- [ ] `installer/scripts/install.ps1` — PowerShell for Windows
- [ ] First-run wizard HTML/CSS/JS (go:embed)
- [ ] Systemd service unit file (Linux)
- [ ] LaunchAgent plist (macOS)
- [ ] Windows .msi installer (WiX or NSIS)
- [ ] Test: clean Ubuntu 22.04 VM, run installer, verify first-run wizard

## tpt-pharmacy (Milestone 3)
- [ ] PHARMAC formulary dispensing
- [ ] Prescription receive from GP (FHIR MedicationRequest)
- [ ] FHIR MedicationDispense recording
- [ ] Schedule 2 drug two-pharmacist check + extended audit
- [ ] HSD (Health Survey and Dispensing) reporting
- [ ] PHARMAC subsidy claiming

## tpt-immunisation (Milestone 4)
- [ ] NIR (National Immunisation Register) FHIR API integration
- [ ] Vaccination scheduling (NZ immunisation schedule)
- [ ] FHIR Immunization resource recording
- [ ] Outbreak tracking and recall management
- [ ] NZ childhood immunisation schedule logic

## tpt-mental-health (Milestone 5)
- [ ] Extra-sensitive data flag enforcement (HIPC additional protections)
- [ ] Mental Health (Compulsory Assessment and Treatment) Act 1992 workflows
- [ ] Compulsory treatment order management
- [ ] Inpatient + outpatient psychiatric care workflows
- [ ] Enhanced consent model for mental health records

## tpt-pathology (Milestone 6)
- [ ] MLLP listener for HL7 v2 lab messages
- [ ] ORU^R01 → FHIR DiagnosticReport + Observation conversion
- [ ] NZ lab-specific Z-segment parsers
- [ ] Specimen tracking
- [ ] Result notification to requesting GP (FHIR subscription)
- [ ] Reference range management

## tpt-radiology (Milestone 7)
- [ ] Orthanc DICOM server integration
- [ ] DICOMweb (WADO-RS, STOW-RS, QIDO-RS) endpoints
- [ ] FHIR ImagingStudy resource management
- [ ] Radiology reporting workflow
- [ ] Image sharing (referrer access)
- [ ] RIS (Radiology Information System) workflows

## tpt-aged-care (Milestone 8)
- [ ] interRAI assessment tools
- [ ] NASC (Needs Assessment Service Coordination)
- [ ] Funded hours management
- [ ] Residential and home care workflows

## CAM Modules (Milestone 9)
- [ ] tpt-acupuncture (ACC claiming, needle site documentation)
- [ ] tpt-chiropractic (spinal charting, ACC, X-ray referrals)
- [ ] tpt-osteopathy
- [ ] tpt-massage (ACC registered, SOAP notes, contraindication screening)
- [ ] tpt-counselling (EAP billing, session notes, private practice)
- [ ] tpt-naturopathy (supplement/remedy tracking, private pay)
- [ ] tpt-tcm (herb dispensing, tongue/pulse diagnosis)
- [ ] tpt-nutrition (food diary, meal planning, body composition)

## tpt-hospital (Milestone 10)
- [ ] Inpatient management (admission, discharge, transfer — FHIR Encounter)
- [ ] Ward management and bed management
- [ ] ED triage workflows
- [ ] ICU workflows
- [ ] Surgical scheduling (theatre booking, FHIR Appointment + Schedule)
- [ ] Clinical coding (ICD-10-AM, ACHI)
- [ ] Discharge summaries
- [ ] Hospital billing (casemix, DRG)

## Remaining Modules (Post-Hospital)
- [ ] tpt-blood-bank (cross-matching, blood product inventory, donor management)
- [ ] tpt-dental (FDI tooth charting, ACC, dental-specific workflows)
- [ ] tpt-vision (optometry/ophthalmology, prescription management, optical dispensing)
- [ ] tpt-allied-health (physio, OT, speech therapy, podiatry)
- [ ] tpt-community-health (district nursing, home visits, outreach)
- [ ] tpt-addiction (methadone programme, counselling workflows)
- [ ] tpt-palliative (hospice, advance care planning, pain protocols)
- [ ] tpt-disability (NASC, support plans, funded hours)
- [ ] tpt-screening (national programmes, recall systems, results management)
- [ ] tpt-epidemiology (disease surveillance, outbreak investigation, public health reporting)
- [ ] tpt-telehealth (video consultations, remote monitoring — port Jitsi/WebRTC from tpt-doctor)
- [ ] tpt-clinical-trials (protocol management, participant tracking, adverse events)
- [ ] tpt-health-billing (ACC claiming, PHARMAC subsidies, health insurance — cross-module)
- [ ] tpt-midwifery (LMC model, MMPO claiming, antenatal/intrapartum/postnatal, home birth)

## Compliance & Security
- [ ] `core/breach/` — Privacy Act breach notification workflow (72h to Privacy Commissioner)
- [ ] Penetration testing (before any hosted tier launch)
- [ ] Privacy Impact Assessment documentation
- [ ] Full HIPC compliance audit
- [ ] HPCA scope-of-practice enforcement tested across all practitioner types
