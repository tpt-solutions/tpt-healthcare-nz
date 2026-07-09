# E2E Testing (Platform Review — first PR, mocked-backend scope)

> CONTRIBUTING.md long promised a "Playwright smoke test for critical paths" that was never implemented. This tracks standing up a first, real e2e suite (mocked API boundary — no seed data/real backend yet) plus the deferred follow-ups identified during the review.

## Workspace scaffolding
- [x] `e2e/package.json`, `e2e/tsconfig.json`, `e2e/playwright.config.ts` (projects: clinic, portal)
- [x] `e2e/fixtures/auth.ts` — mocked `POST /api/v1/auth/token` fixture
- [x] `e2e/fixtures/test-data.ts` — `TEST_PRACTITIONER`, `TEST_PATIENT` mock payload constants
- [x] `e2e/pages/` page objects: `LoginPage.ts`, `PatientListPage.ts`, `PatientDetailPage.ts`, `NewPatientPage.ts`
- [x] `e2e/.gitignore` (test-results/, playwright-report/, blob-report/)
- [x] Add `e2e` to `pnpm-workspace.yaml`
- [x] Add `test:e2e` script to root `package.json`

## First 5 specs (mocked API, Chromium only)
- [x] `e2e/specs/clinic/login.spec.ts` — success + failure
- [x] `e2e/specs/clinic/patient-list.spec.ts`
- [x] `e2e/specs/clinic/patient-detail.spec.ts`
- [x] `e2e/specs/clinic/patient-create.spec.ts`
- [x] `e2e/specs/portal/login.spec.ts`

## CI & docs
- [x] Add parallel `e2e` job to `.github/workflows/ci.yml` (install, Playwright browsers, run, upload HTML report artifact)
- [x] Update `CONTRIBUTING.md` to describe the real `e2e/` package and how to run it locally
- [x] Run the suite locally to verify it actually passes — all 8 specs green (`pnpm test:e2e`)

## Bugs found and fixed while verifying the suite (not hypothetical — caught by writing real e2e tests)
- [x] `apps/tpt-clinic/src/contexts/ApiContext.tsx` — `buildUrl()` called `new URL(relativePath)` with no base, which throws `TypeError: Invalid URL` in every real browser. **Every API call in tpt-clinic was silently failing before this fix.** Fixed by passing `window.location.origin` as the base.
- [x] `packages/ui/package.json` — `NHIInput` imports `@tpt/nz-codes` but the package never declared it as a dependency; only worked for consumers (like tpt-clinic) that happened to also depend on it directly. Broke `tpt-portal`, which doesn't. Added as a direct dependency of `@tpt/ui`.
- [x] `apps/tpt-clinic/package.json` & `apps/tpt-portal/package.json` — `vite-plugin-pwa`'s registration code imports `workbox-window`, which wasn't declared as a direct dependency of either app, breaking pnpm's strict `node_modules` resolution in dev mode. Added to both.
- [x] Investigated: `tpt-clinic`'s `vite build` (bypassing `tsc`) is **not actually hung** — it's a slow cold-start dependency pre-bundle (~110s with `node_modules/.vite` cleared; the real bundling step itself is ~6s). Confirmed by clearing the cache and timing a fresh build to completion. The e2e webServer still uses the Vite dev server (faster, avoids paying that cold-start cost on every CI run for two apps), so no code change was needed here — this item is resolved as "understood, not a bug."

## Deferred follow-ups (tracked, not forgotten)
- [x] Real-backend e2e integration: seed data/test practitioner, containerize `tpt-doctor` (the service that actually owns `/api/v1/patients` — not `interop`, which has no patients route), re-point `patient-create.spec.ts` at a real Postgres + tpt-doctor stack.
  - **Major pre-existing bug found and fixed while wiring this up**: `modules/tpt-doctor` did not compile at all — `make build-doctor` had presumably never succeeded. Handler code across `patients_handler.go`, `claims_handler.go`, `pho.go`, `prescriptions_handler.go`, `referrals.go`, and `server.go` called methods/types that don't exist on the real `core/nhi`, `core/nes`, `core/acc`, `core/audit`, and `core/fhir/r5` packages (e.g. `nhiClient.Lookup` vs. the real `GetPatient`, `auditTrail.Write(Actor:, Metadata:)` vs. the real `Record(PrincipalID:, Details:)`, `acc.LodgeRequest` which doesn't exist, `patient.MarshalJSON()`/`SearchName()` which aren't defined on the plain `r5.Patient` struct). Fixed all of it so the module now builds and passes `go vet`.
  - Also fixed: `POST /api/v1/patients` expected a `{nhi, patient: <FHIR Patient>}` body, but the clinic UI's `NewPatientPage.tsx` has always sent a flat form (`firstName`, `lastName`, `dateOfBirth`, `gender`, address fields, etc.) — every real patient registration would have 400'd. Added `patientCreateRequest.toFHIRPatient()` to build the FHIR resource server-side from the flat form fields, and made the NHI field genuinely optional (matching the UI's "leave blank to have NHI assigned by the Ministry" copy) with Ministry confirmation only attempted when an NHI client is configured.
  - Also fixed: the clinic app never sent `X-Tenant-ID`, which `TenantExtraction` middleware requires on every authenticated request — every real API call would have 400'd regardless of the above. Added `getTenantId()` to `AuthContext`/`ApiContext`, sourced from a new `tenant_id` field in the login response.
  - Added a dev/test-only `POST /api/v1/auth/token` endpoint (`modules/tpt-doctor/api/dev_auth_handler.go`, `core/auth/jwt`-backed) gated by `TPT_DOCTOR_DEV_AUTH_ENABLED`, since `tpt-doctor` is otherwise wired for Auth0 and there was no local login path for tests. Never enable this in production.
  - Added `modules/tpt-doctor/Dockerfile` + `docker-entrypoint.sh` (migrate → seed → serve) and a `tpt-doctor` service in `deploy/docker-compose.dev.yml`. The pre-existing `interop` placeholder is untouched — it isn't the service this flow needs.
  - Added `deploy/seed/e2e_seed.sql` (a fixed dev tenant row) and `patient-create.spec.ts` now has a real-backend mode gated by `E2E_REAL_BACKEND=true` via a new `e2e/fixtures/real-backend-auth.ts` (default `pnpm test:e2e`/CI still runs the mocked variant, unaffected). See CONTRIBUTING.md for how to run it.
  - **Not done** (flagged, not silently skipped): `core/backup` and all of `interop/api` have their own, unrelated pre-existing compile errors (confirmed via `go build ./core/...` and `go build ./interop/...`) — neither is imported by `tpt-doctor`, so they didn't block this work, but `make build` / `make test` at the repo root will still fail until those are separately fixed.
- [ ] `tpt-admin` e2e coverage
- [ ] Deeper `tpt-portal` coverage (booking, records, consent, messages)
- [ ] Appointments/queue/encounter/prescriptions e2e flows in `tpt-clinic`
- [ ] E2E coverage for specialty-module pages (blocked on their backend services still being placeholder containers)
- [ ] Visual regression + multi-browser/mobile matrix
- [ ] Go unit/integration test coverage for the 34 currently-untested `modules/tpt-*` packages (separate effort from e2e, flagged by the same platform review)
- [x] Go unit test coverage for a first batch of previously-untested `core/*` packages: `acc`, `auth`, `auth/jwt`, `ddi`, `fhir/translate`, `hl7`, `rbac`, `repo/search`, `resilience`, `storage/provider`, `terminology`. Added shared `core/mock/` fakes (checker, repository, storage, token) and `core/testutil/` helpers to support them.
  - Still untested in `core/`: `accounting`, `accounts`, `ai`, `audit`, `backup` (also has pre-existing compile errors, see above), `breach`, `comms-prefs`, `consent`, `db`, `email`, `episurv`, `erms`, `fax`, `fhir` (top-level, as opposed to `fhir/translate`), `forms`, `gp2gp`, `health`, `hpi`, `inventory`, `medsafe`, `messaging`, `middleware`, `msd`, `nes`, `outbox`, `payment`, `payroll`, `pharmac`, `pharmacy-gateway`, `primhd`, `push`, `queue`, `recall`, `scoring`, `sms`, `subscription`, `tenant`, `video`, `worksafe`.
- [x] Vitest coverage added for previously-untested frontend packages: `@tpt/api-client`, `@tpt/fhir-types`, `@tpt/nz-codes`, `@tpt/offline-store`, `@tpt/ui` (components: Badge, Button, Card, ErrorBoundary, Input, Modal, NHIInput, PatientBanner, Table).
- [x] Add ErrorBoundary components to `tpt-clinic`/`tpt-portal`/`tpt-admin` — added a shared `ErrorBoundary` class component to `packages/ui` (`components/ErrorBoundary.tsx`, exported from `src/index.ts`) with a "Try again" / "Reload page" fallback UI matching the existing design system, and wrapped each app's route tree in it (`App.tsx` in all three apps). Verified by simulating a real render crash (patient record missing `name`) end-to-end in a browser: the fallback rendered, "Try again" correctly re-threw (same bad data), and "Reload page" reloaded without losing the URL. Full e2e suite (8/8) still passes after wrapping.
  - Bonus fix found while verifying `tpt-admin` in a real browser: it had the **same missing `workbox-window` dependency bug** as `tpt-clinic`/`tpt-portal` (never caught because nothing had loaded the admin app in a real browser before) — its `main.tsx` was failing to load entirely, leaving a blank page. Fixed the same way (added `workbox-window` to `apps/tpt-admin/package.json`).
