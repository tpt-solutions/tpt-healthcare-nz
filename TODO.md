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
- Also noted (not fixed, out of scope): `tpt-clinic`'s production build (`tsc && vite build`) currently hangs indefinitely during bundling for reasons unrelated to the above — the e2e webServer uses the Vite dev server instead. Worth its own investigation.

## Deferred follow-ups (tracked, not forgotten)
- [ ] Real-backend e2e integration: seed data/test practitioner, containerize the `interop` binary in `deploy/docker-compose.dev.yml` (currently a `sleep infinity` placeholder), re-point `patient-create.spec.ts` at a real Postgres + interop stack
- [ ] `tpt-admin` e2e coverage
- [ ] Deeper `tpt-portal` coverage (booking, records, consent, messages)
- [ ] Appointments/queue/encounter/prescriptions e2e flows in `tpt-clinic`
- [ ] E2E coverage for specialty-module pages (blocked on their backend services still being placeholder containers)
- [ ] Visual regression + multi-browser/mobile matrix
- [ ] Go unit/integration test coverage for the 34 currently-untested `modules/tpt-*` packages (separate effort from e2e, flagged by the same platform review)
- [ ] Add ErrorBoundary components to `tpt-clinic`/`tpt-portal`/`tpt-admin` (none exist today — flagged by the same platform review, not part of the e2e PR)
