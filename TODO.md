# TODO — TPT Healthcare NZ Testing Suite

## Phase 1-5: Core + Frontend (COMPLETED)

- [x] Test infrastructure (testutil/helpers, mock/)
- [x] Core pure-logic (rbac, auth, ddi, resilience, translate, hl7)
- [x] Core data-layer (repo, terminology, storage, nhi)
- [x] Frontend package vitest (nz-codes, api-client, offline-store, fhir-types, ui)
- [x] Frontend component tests (@tpt/ui components — Button, Card, Badge, Table, Input, Modal, PatientBanner, NHIInput, ErrorBoundary)

## Phase 6: Module Unit Tests (IN PROGRESS)

### Tier 1 — High Priority (Rich Pure Logic)

- [ ] **tpt-dental** — fdi/chart_test.go (~25 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-dental** — fdi/surface_test.go (~18 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-dental** — procedure/codes_test.go (~10 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-dental** — acc/claim_test.go (~12 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-vision** — refraction/prescription_test.go (~22 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-vision** — optical/dispensing_test.go (~8 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-vision** — ophthalmology/exam_test.go (~5 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-vision** — acc/claim_test.go (~10 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-allied-health** — acc/claim_test.go (~19 tests)
  - [x] Write test file
  - [x] Run & verify all pass

### Tier 1 — Sub-discipline Tests (Repeated Patterns)

- [ ] **tpt-allied-health** — speech/therapy_test.go (~10 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-allied-health** — physio/treatment_test.go (~8 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-allied-health** — ot/assessment_test.go (~8 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-allied-health** — podiatry/care_test.go (~10 tests)
  - [x] Write test file
  - [x] Run & verify all pass

### Tier 2 — Community Health

- [ ] **tpt-community-health** — homevisit/visit_test.go (~12 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-community-health** — outreach/program_test.go (~12 tests)
  - [x] Write test file
  - [x] Run & verify all pass
- [ ] **tpt-community-health** — districtnursing/plan_test.go (~8 tests)
  - [x] Write test file
  - [x] Run & verify all pass

### Tier 2 — Addiction

- [ ] **tpt-addiction** — methadone/programme_test.go (~7 tests)
  - [x] Write test file
  - [x] Run & verify all pass

## Skipped

- [ ] **tpt-palliative** — No testable logic (pure data types, zero functions)

## Phase 7: Hospital Go-Live Gaps (Auckland City / Starship)

Gap analysis for a large adult tertiary hospital (Auckland City) and paediatric tertiary
hospital (Starship) going live on `modules/tpt-hospital` + `modules/tpt-maternal-child-health`.
See plan `lets-say-auckland-city-jolly-pinwheel.md` for full detail.

- [ ] **tpt-hospital / tpt-pathology / tpt-radiology** — No lab/imaging order entry (CPOE) linked to admissions
  - [ ] Design order model (admissionID-linked) and wire to pathology/radiology result callback
- [ ] **core/hl7** — No HL7 ADT (A01/A02/A03/A08) or ORM/ORU message builders (`core/hl7/{client,mllp,parser}.go` is transport/parser only)
  - [ ] Add ADT message builders for admit/transfer/discharge/update events
  - [ ] Add ORM/ORU builders for lab/imaging order and result messages
- [ ] **tpt-hospital/api/billing.go** — DRG/casemix grouper is a placeholder (`deriveDRG` hardcodes ~4 AR-DRG buckets from first letter of diagnosis code)
  - [ ] Implement real AR-DRG/WIES grouper in `core/terminology/`
- [ ] **core/fhir** — No FHIR `Location` resource; `Encounter` only exists in r5, not r4
  - [ ] Add Location resource type; add r4 Encounter for compatibility
- [ ] **tpt-hospital/api/pharmacy_*.go** — eMAR lacks barcode/five-rights verification, controlled-drug (S8) register, IV pump/smart-infusion integration
  - [ ] Scope and implement bedside verification + S8 register
- [ ] **tpt-hospital/api/icu.go** — ICU/PICU charting has no fluid balance charting or EWS/PEWS early-warning score engine; no PICU-specific charting distinct from adult ICU
  - [ ] Add fluid balance charting
  - [ ] Add EWS (adult) / PEWS (paediatric) scoring engine
- [ ] **tpt-hospital + tpt-maternal-child-health** — Paediatric/NICU/PICU tooling (nicu.go, paediatrics_picu.go, paediatrics_growth.go) not linked back to `hospital_admissions`
  - [ ] Wire NICU/PICU/growth-chart records to the hospital admission model
- [ ] **tpt-hospital / tpt-maternal-child-health** — No weight-based/paediatric dosing calculator (mg/kg dosing, max-dose caps)
  - [ ] Implement paediatric dosing calculator
- [ ] **tpt-hospital/api/wards.go** — No bed-board/patient-flow forecasting beyond a live capacity snapshot (no discharge-ETA prediction, escalation triggers)
  - [ ] Design and implement patient-flow forecasting dashboard
- [ ] **tpt-hospital/api/admissions_*.go** — Discharge summaries not confirmed auto-populated from coding/pharmacy/admission data, nor wired to GP transmission (core/gp2gp)
  - [ ] Auto-populate discharge summary from admission/coding/pharmacy data
  - [ ] Confirm/wire GP transmission path

## Phase 8: Replace Stubs & Scaffolds with Real Implementations

Repo-wide audit for placeholder/stub/scaffold code that needs replacing with real, working
implementations. Found via full sweep of `core/`, `interop/`, and all `modules/*`.
See plan `lets-say-auckland-city-jolly-pinwheel.md` for full detail.

### Critical — integration/persistence surfaces

- [ ] **interop/api/fhir.go** — FHIR R4/R5 REST API is entirely non-persistent: Create/Update assign an in-memory counter ID and echo the body back, Read fabricates a stub resource, Search always returns an empty Bundle, Delete is a no-op, R4↔R5 translation is a shallow copy
  - [ ] Wire Create/Read/Update/Delete/Search to a real resource repo (`core/repo`)
  - [ ] Implement real R4↔R5 field translation (`core/fhir/translate`)
- [ ] **interop/api/terminology.go** — Wired to a no-op `stubTermStore` instead of the real, already-implemented `core/terminology` package (SNOMED/LOINC/ICD-10-AM/NZMT all return empty)
  - [ ] Implement `TermStore` using `core/terminology` and pass it into `newTerminologyHandler`
- [ ] **core/db/migrate.go** — `Migrate()` silently no-ops when passed a non-string third argument (several modules pass `*slog.Logger` by mistake instead of a migrations-dir path), so those modules never get their schema applied
  - [ ] Fix `Migrate()` to fail loudly on a bad argument instead of silently skipping
  - [ ] Fix call sites in tpt-chiropractic, tpt-osteopathy, tpt-acupuncture, tpt-tcm, tpt-massage, tpt-naturopathy, tpt-blood-bank

### Fully-scaffolded modules (need full persistence)

- [ ] **tpt-pharmacy** — Dispensing/claims workflow is entirely `// In production: ...` placeholders; no `db/migrate` directory; nothing persisted
- [ ] **tpt-counselling** — EAP claims/session notes/private-practice CRUD all in-memory; list endpoints return hardcoded empty slices; no migrations
- [ ] **tpt-nutrition** — Food diary/meal plans/body composition CRUD in-memory only; no migrations
- [ ] **tpt-immunisation** — Every handler is `// In production: ...`; `nir.go` submits a placeholder struct instead of a real FHIR R4 Immunization to the National Immunisation Register
- [ ] **tpt-health-billing** — ACC/insurance/invoices/PHARMAC billing/reconciliation all `// In production: query billing_...` returning hard-coded JSON; no migrations
- [ ] **tpt-clinical-trials** — Real SQL migrations exist, but every handler (participants, adverse events, protocols, visits — ~43 methods) just returns HTTP 501 via `notImplemented()`
- [ ] **tpt-chiropractic** — Blocked by `core/db/migrate.go` bug above; handlers hold everything in-memory (`internal/spine/chart.go`, `internal/xray/referral.go`)
- [ ] **tpt-osteopathy** — Same pattern as tpt-chiropractic
- [ ] **tpt-acupuncture** — Same pattern as tpt-chiropractic
- [ ] **tpt-tcm** — Same pattern as tpt-chiropractic
- [ ] **tpt-massage** — Same pattern as tpt-chiropractic
- [ ] **tpt-naturopathy** — Same pattern as tpt-chiropractic

### Missing/broken migrations

- [x] **tpt-aged-care** — Handlers issue real SQL against `aged_care_plans`, `aged_care_funded_hours_allocations`, `aged_care_interrai_assessments`, `aged_care_nasc_referrals`, `aged_care_nasc_service_plans` — added `modules/tpt-aged-care/db/migrations` + `embed.go`, wired via `migrate.New(agedcaredb.Migrations, pool)` in `RunMigrations`
- [x] **tpt-blood-bank** — Real query code (crossmatch/donors/inventory) — added `modules/tpt-blood-bank/db/migrations/001_blood_bank_tables.sql` + `embed.go`, wired via `migrate.New(bloodbankdb.Migrations, pool)` in `RunMigrations`
- [x] **tpt-practice** — Real query code (rostering/settings) — tables already exist in `core/db/migrate/007_practice_management.sql`; fixed the broken `db.Migrate(ctx, pool, "")` call site to use `migrate.New(migrate.MigrationsFS, pool)`

### Partial stubs in otherwise-real modules

- [ ] **tpt-allied-health** — One "get by ID" handler each in `acc_handler.go`, `ot.go`, `podiatry.go`, `speech.go` marked `// TODO: fetch from database; stub returns placeholder data.` (11 handlers total)
- [ ] **tpt-vision** — `acc.go` (ListClaims, GetClaimFHIR), `ophth.go` (GetExamFHIR), `optical.go` (GetOrderFHIR), `refraction.go` return hard-coded/placeholder payloads instead of real repo queries
- [ ] **tpt-dental** — `acc.go` (SubmitClaim + claim CRUD) and `procedure.go` (treatment-record CRUD) explicitly commented "Simplified stub", operate on in-memory data
- [ ] **tpt-doctor** — `api/pho.go` (PHO extract "would transmit" but doesn't) and `api/referrals.go` (`Send` doesn't dispatch to receiving provider's inbox)
- [ ] **tpt-pathology** — `api/mllp.go` `resolveTenant` for inbound HL7 MLLP messages doesn't look up the lab site code against a tenant mapping
- [ ] **tpt-cardiology / tpt-rehabilitation / tpt-maternal-child-health** — `api/helpers.go` `recordAudit` isn't in the same DB transaction as the clinical write it audits (durability/consistency gap)

### Moderate stubs in core/

- [ ] **core/subscription/engine.go** — `buildNotificationBundle` emits a minimal FHIR R5 SubscriptionNotification with a hardcoded `"Subscription/unknown"` reference instead of the real subscription ID
- [ ] **core/backup/scheduler.go** — `recordSuccess` records a 0-byte backup as successful (only warns) when no storage provider is configured — silent data-loss risk
- [ ] **core/nhi/nhi.go** — NHI types marked `// TODO: replace with proper FHIR R4 types`

## Phase 9: Tenant Service-Line Profiles

Onboarding a new facility (e.g. a large tertiary hospital vs. a single-service clinic)
currently has no concept of "which service lines does this tenant run." Recommendation:
a service-line profile/enablement layer rather than fixed per-hospital templates, since
real facilities are combinations (e.g. one campus with both adult and paediatric wards)
that a hard-coded template can't cleanly represent. See `core/tenant` for the existing
generic `Tenant` model this would extend.

- [x] **core/tenant** — Design a service-line profile: tenant selects which service lines it runs (ED, ICU, NICU/PICU, theatre, oncology, etc.) at onboarding
  - [x] Define the service-line catalogue/schema — `core/servicelines/catalogue.go`, 16 service lines, each with modules/ward-types/triage-scale/formulary; persisted per-tenant in `tenant_service_lines` (`core/db/migrate/013_tenant_service_lines.sql`)
  - [x] Wire service-line selection to toggle relevant modules/routes per tenant — `PUT /api/v1/practice/service-lines` unions resolved modules into the existing `tenants.settings.activeModules` (additive; manually-enabled modules are preserved)
  - [x] Seed sensible defaults per service line (ward-type list, triage scale, relevant formulary subset) — exposed via `GET /api/v1/practice/service-lines` and `core/servicelines.Resolve{Modules,WardTypes,FormularySubset,TriageScales}`
  - [x] Support facilities with multiple/mixed service lines (no forking or per-site hard-coded templates) — a tenant selects any subset of the catalogue; defaults are unioned, not templated
  - [ ] Frontend: admin UI for selecting service lines during onboarding (backend/API complete; no UI wired yet)

## Final Verification

- [ ] Run `go test ./modules/tpt-dental/...`
- [ ] Run `go test ./modules/tpt-vision/...`
- [ ] Run `go test ./modules/tpt-allied-health/...`
- [ ] Run `go test ./modules/tpt-community-health/...`
- [ ] Run `go test ./modules/tpt-addiction/...`
- [ ] Run `gofmt -w ./modules/...`
- [ ] Run `make lint`
