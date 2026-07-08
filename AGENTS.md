# AGENTS.md — OpenCode session guide

## Quick commands

| Task | Command |
|------|---------|
| Full Go lint+test+build | `make lint && make test && make build` |
| Single Go module test | `make test-<module>` (e.g. `make test-core`) or `go test ./core/nhi/...` |
| Single Go module lint | `make lint-<module>` (e.g. `make lint-doctor`) |
| Frontend lint | `pnpm lint` |
| Frontend typecheck | `pnpm typecheck` |
| Frontend build | `pnpm build` |
| E2E tests (mocked, no backend) | `pnpm test:e2e` |
| E2E tests (single project) | `pnpm test:e2e --project=clinic` / `--project=portal` / `--project=admin` |
| Start dev stack (Docker) | `make dev` |
| Run migrations | `make migrate` |
| Generate FHIR types | `cd tools/gen-fhir-types && go run .` |
| Generate PWA icons | `make icons` |

**Required order before merging**: `gofmt -w ./...` → `make lint` → `make test` → `make build` → `pnpm lint` → `pnpm build`. CI runs all three jobs (go, frontend, e2e).

## Go version

`go.work` declares `go 1.24.0`, but individual `go.mod` files use `go 1.22.0`. CI runs Go 1.22. Do not use Go 1.24-only features in module code — they will break CI.

## Go module paths

The GitHub repo is `tpt-solutions/tpt-healthcare-nz`, but Go module paths use `github.com/PhillipC05/tpt-healthcare` (no `-nz`, different owner). This is intentional. Never change module paths.

Every module's `go.mod` needs a `replace` directive pointing to `../core`:
```go
replace github.com/PhillipC05/tpt-healthcare/core => ../../core
```

## Package manager

Use `pnpm` exclusively. Never `npm` or `yarn`. Workspace config in `pnpm-workspace.yaml` includes `apps/*`, `packages/*`, and `e2e`.

## Architecture

```
apps/          → React+Vite PWAs (tpt-clinic, tpt-portal, tpt-admin)
packages/      → Shared frontend libs (@tpt/ui, @tpt/fhir-types, etc.)
core/          → Shared Go kernel (db, auth, FHIR, NZ integrations)
interop/       → FHIR REST gateway (Go, Cobra CLI)
modules/       → 35 specialty-specific Go modules
deploy/        → Docker Compose files
e2e/           → Playwright specs (its own workspace member @tpt/e2e)
```

Frontend apps talk to the Go backend via FHIR R5 REST. The `tpt-clinic` app is the clinician-facing app; `tpt-portal` is the patient-facing app; `tpt-admin` is the practice admin app.

## Frontend conventions

- All FHIR types come from `@tpt/fhir-types` — never hand-roll FHIR structs.
- Shared UI components live in `@tpt/ui` (Tailwind-based).
- No PHI in localStorage or sessionStorage. Health info must not be persisted client-side.
- `vite-plugin-pwa` is used in all three apps. Service workers are blocked in e2e tests.
- `pnpm install` must be run from the repo root (workspace resolution).

## Go conventions

- Database access via `pgx.Pool`, never `database/sql`.
- SQL queries go in `*_query.go` files alongside the struct they serve.
- Error wrapping: always `fmt.Errorf("context: %w", err)`.
- Every I/O function takes `context.Context` as first arg.
- Logging via `slog` (stdlib). No global loggers.
- Tests use `testify/assert` for assertions, `testify/require` when test cannot continue.
- Integration tests use `testcontainers-go` (requires Docker).
- All clinical writes must produce an `AuditEvent` via `core/audit/trail.go`.
- Consent checks before every third-party disclosure via `core/consent/`.
- PHI fields encrypted with AES-256-GCM via `core/encryption/`.

## E2E test architecture

- Three Playwright projects: `clinic` (port 4173), `portal` (4174), `admin` (4175).
- Vite dev servers are started automatically by Playwright's `webServer` config.
- Most specs use mocked API boundaries via `page.route()` — no backend needed.
- Auth fixtures: `e2e/fixtures/auth.ts` (clinic), `e2e/fixtures/admin-auth.ts` (admin). Portal uses stub auth (any non-empty email).
- Page Object pattern: reusable selectors in `e2e/pages/` and `e2e/pages/admin/`.
- Real-backend tests gated behind `E2E_REAL_BACKEND=true`.

## NZ-specific gotchas

- NHI format: `[A-Z]{3}[0-9]{4}` (old) or `[A-Z]{3}[0-9]{2}[A-Z]{2}` (new Luhn). Always validate checksum.
- Audit records are append-only — no UPDATE or DELETE on `audit_events`.
- Mental health records carry an extra-sensitive flag; access requires elevated consent.
- Never use production NHI/HPI/ACC credentials in development. Always use UAT endpoints.
- All FHIR resources must validate against NZ Base IG profiles before persisting.

## CI

GitHub Actions runs three parallel jobs on push to `master`:
1. **Go**: `make lint` → `make test` → `make build`
2. **Frontend**: `pnpm lint` → `pnpm build`
3. **E2E**: `pnpm test:e2e` (installs Playwright/Chromium, runs mocked specs)

No external PRs are accepted. Changes go through issues and are implemented internally.
