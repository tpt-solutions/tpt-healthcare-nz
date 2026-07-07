# Contributing to tpt-healthcare-nz

Thank you for your interest in tpt-healthcare-nz. Because this codebase handles sensitive New Zealand health information, contributions are held to a higher standard than a typical open-source project.

**This repository does not accept external pull requests.** Code changes are made internally (by tpt-solutions maintainers and their AI agents) in response to filed issues. If you've found a bug, have a feature request, or a security concern, please open an issue — see [Issue Process](#issue-process) below.

---

## Table of Contents

1. [NZ Healthcare Domain Knowledge](#nz-healthcare-domain-knowledge)
2. [Branching Strategy](#branching-strategy)
3. [Development Setup](#development-setup)
4. [Code Style](#code-style)
5. [Testing Requirements](#testing-requirements)
6. [Commit Message Format](#commit-message-format)
7. [Issue Process](#issue-process)
8. [Adding New Modules](#adding-new-modules)
9. [Compliance Checklist](#compliance-checklist)

---

## NZ Healthcare Domain Knowledge

Contributors working on clinical or integration features must have working knowledge of:

- **NHI (National Health Index)** — the unique patient identifier assigned by Te Whatu Ora. All patient records must be linked to or reconciled against the NHI.
- **HPI (Health Practitioner Index)** — the national register of health practitioners. APC (Annual Practising Certificate) status must be validated before clinical actions.
- **NES (National Enrolment Service)** — manages patient enrolment with PHOs and general practices.
- **ACC (Accident Compensation Corporation)** — NZ's no-fault accident compensation scheme. Many clinical modules must support ACC claim lodgement.
- **PHARMAC** — the Pharmaceutical Management Agency. Prescribing modules must check PHARMAC formulary and subsidy rules.
- **HIPC (Health Information Privacy Code 2020)** — the primary legal instrument governing health information privacy in NZ. All 13 rules must be considered when handling health data.
- **Privacy Act 2020** — governs personal information broadly; health information is treated as sensitive.
- **FHIR R5** — the HL7 FHIR Release 5 standard. This project uses R5 as the canonical data model, with R4 support for NHI/NES APIs.
- **NZ Base Implementation Guide** — Te Whatu Ora's NZ-specific FHIR profiles and extensions at `https://standards.digital.health.nz/`.

If you are unfamiliar with these systems, review the linked resources in `docs/references.md` (to be created) before working on relevant code.

---

## Branching Strategy

We use trunk-based development. Maintainers and their AI agents commit or push directly to `master`; there is no external pull request review step.

| Branch | Purpose |
|--------|---------|
| `master` | Production-ready code. Protected: direct pushes require CI to pass; force-pushes and deletion are blocked. |
| `feat/<scope>/<description>` | New features (e.g., `feat/nhi/patient-match-endpoint`), merged internally once CI passes |
| `fix/<scope>/<description>` | Bug fixes (e.g., `fix/acc/claim-status-polling`) |
| `chore/<description>` | Tooling, dependencies, CI (e.g., `chore/update-pgx-v5`) |
| `docs/<description>` | Documentation-only changes |
| `compliance/<description>` | Compliance or security-driven changes |

Branch naming rules:
- Use lowercase and hyphens only (no underscores, no slashes within the description segment).
- Keep descriptions concise (3–6 words).
- Delete branches after merging into `master`.

---

## Development Setup

### Prerequisites

- Go 1.22+
- Node.js 20+ and pnpm 9+
- Docker Desktop (or Docker Engine + Compose plugin)
- `golangci-lint` v1.60+ (`go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest`)

### First-time setup

```bash
# Clone the repo
git clone https://github.com/tpt-solutions/tpt-healthcare-nz.git
cd tpt-healthcare-nz

# Install frontend dependencies
pnpm install

# Start the local dev stack (Postgres + Redis + interop)
make dev

# Run all tests
make test
```

### Environment variables

Copy `.env.example` to `.env` and fill in values. Never commit `.env`. Key variables:

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | pgx-format Postgres URL (`postgres://user:pass@localhost:5432/tpt_healthcare_dev`) |
| `REDIS_URL` | Redis URL (`redis://localhost:6379`) |
| `ENCRYPTION_KEY` | 32-byte base64-encoded AES-256 key for PHI field encryption |
| `AUTH0_DOMAIN` | Auth0 tenant domain (optional, for Auth0 auth mode) |
| `HPI_API_BASE_URL` | HPI FHIR API base URL (use UAT endpoint during development) |
| `NHI_API_BASE_URL` | NHI FHIR API base URL (use UAT endpoint during development) |

Always use UAT/sandbox credentials for NHI, NES, HPI, and ACC during development. Never use production credentials on a developer machine.

---

## Code Style

### Go

- **Formatting**: `gofmt -w ./...` — enforced by CI. No change will be merged with formatting errors.
- **Linting**: `golangci-lint run ./core/... ./interop/...` must pass with zero errors.
- **Package names**: short, lowercase, no underscores (e.g., `package nhi`, `package audit`).
- **Exported symbols**: document all exported functions, types, and constants with a godoc comment.
- **Error wrapping**: use `fmt.Errorf("context: %w", err)` consistently so error chains are inspectable.
- **No global state**: avoid package-level variables (except `var ErrXxx = errors.New(...)` sentinel errors). Pass dependencies explicitly.
- **Interfaces**: define interfaces at the point of use, not in the implementing package.
- **Database queries**: all SQL queries in `*_query.go` files, no inline SQL in business logic.
- **Secrets**: never log, serialize to JSON, or include in error messages any secret, encryption key, or health information.

### TypeScript / React (apps/ and packages/)

- **Formatter**: Prettier with project config. Run `pnpm format` before committing.
- **Linter**: ESLint with project config. Run `pnpm lint` before committing.
- **Types**: strict TypeScript (`"strict": true`). No `any` without a documented justification.
- **FHIR types**: import from `@tpt/fhir-types` for all FHIR resources. Never hand-roll FHIR types.
- **Component naming**: PascalCase for components, camelCase for utilities.
- **No PHI in localStorage or sessionStorage**: health information must not be persisted client-side in any form.

---

## Testing Requirements

All changes must include tests. The bar varies by change type:

| Change type | Required tests |
|-------------|----------------|
| New Go function | Unit test with table-driven cases |
| New HTTP endpoint | Integration test with real Postgres (testcontainers-go) |
| New NZ national integration (NHI, NES, HPI, ACC) | Integration test using the UAT environment with VCR-style HTTP recording for CI |
| New migration | Test that migration runs cleanly on a fresh DB and is idempotent |
| FHIR resource handling | Round-trip test (create → read → validate against NZ Base IG profile) |
| Audit trail | Test that every write produces the correct AuditEvent |
| Frontend component | Vitest unit test + Playwright e2e coverage for critical paths |

### Test conventions

- Test files are `*_test.go` in the same package as the code under test.
- Use `t.Parallel()` in unit tests.
- Integration tests are tagged with `//go:build integration` and must be runnable with `go test -tags=integration ./...`.
- Use `testify/assert` for assertions; `testify/require` when test cannot continue after failure.
- Test helpers in `testutil/` within each module — do not export test helpers into production packages.

### End-to-end tests (`e2e/`)

The `e2e/` package holds Playwright specs covering critical clinical workflows across `apps/tpt-clinic` and `apps/tpt-portal`. It's its own pnpm workspace member (`@tpt/e2e`), separate from each app's own `test` script, so it doesn't get swept into a plain `pnpm test`.

- Run the full suite: `pnpm test:e2e` (from repo root) or `pnpm --filter @tpt/e2e test`.
- Interactive/debug mode: `pnpm --filter @tpt/e2e test:ui`.
- View the last HTML report: `pnpm --filter @tpt/e2e report`.
- Each spec's `webServer` builds and serves the real app via `vite preview` — no backend is required to run these today (see below).

**Default scope — mocked API boundary.** Most specs stub `fetch` calls via Playwright's `page.route()` (see `e2e/fixtures/auth.ts`) rather than hitting a running backend. This protects routing/guards, form validation, and render logic — the highest-churn source of frontend regressions — but does **not** exercise real backend behaviour (audit trail writes, encryption, HIPC consent checks). `pnpm test:e2e` / CI run only this mocked variant; no backend is required.

**Real-backend variant.** `e2e/specs/clinic/patient-create.spec.ts` also has a real-backend mode that exercises the actual `tpt-doctor` service and a real Postgres database end to end (registration form → `POST /api/v1/patients` → row in `patients` → navigation to the new patient's real UUID). To run it:

```bash
docker compose -f deploy/docker-compose.dev.yml up -d postgres redis tpt-doctor
E2E_REAL_BACKEND=true pnpm test:e2e --grep "New patient registration"
```

The `tpt-doctor` container runs migrations and seeds a fixed dev tenant (`deploy/seed/e2e_seed.sql`) on start, and exposes a dev-only `POST /api/v1/auth/token` login endpoint (`core/auth/jwt`-issued, gated by `TPT_DOCTOR_DEV_AUTH_ENABLED` — never enable this in production) so the suite can authenticate without Auth0. Add new real-backend specs by following the same pattern: import `test`/`expect` from `e2e/fixtures/real-backend-auth.ts` instead of `e2e/fixtures/auth.ts`, and gate the `describe` block behind `process.env.E2E_REAL_BACKEND`.

---

## Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

```
<type>(<scope>): <short summary>

[optional body]

[optional footer]
```

**Types**: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `perf`, `compliance`, `security`

**Scopes** (examples): `nhi`, `acc`, `hpi`, `nes`, `pharmac`, `fhir`, `audit`, `consent`, `encryption`, `interop`, `core`, `frontend`, `deploy`, `ci`

**Rules**:
- Summary in imperative mood, lowercase, no trailing period.
- Max 72 characters on the first line.
- Reference issues/tickets in the footer: `Closes #123` or `Refs #456`.
- Breaking changes: add `BREAKING CHANGE:` in the footer with a description.

**Examples**:

```
feat(nhi): add $match operation with Luhn checksum validation

Implements the FHIR $match operation against the NHI FHIR API.
Validates both old-format (ABC1234) and new Luhn-based NHI numbers
before sending requests to the Ministry API.

Closes #42
```

```
compliance(audit): enforce append-only constraint on audit_events table

Adds a PostgreSQL rule that prevents UPDATE and DELETE on audit_events,
satisfying HIPC Rule 5 and audit trail retention requirements.
```

```
fix(acc): correct claim status polling interval on 429 responses

The ACC claiming service was not backing off on 429 (rate limited)
responses, causing repeated failures. Now uses exponential backoff
with jitter capped at 5 minutes.
```

---

## Issue Process

This repository does not accept external pull requests. All contributions — bug reports, feature requests, and security concerns (non-sensitive ones; see [SECURITY.md](SECURITY.md) for vulnerability disclosure) — go through GitHub Issues.

1. **Search existing issues** first to avoid duplicates.
2. **Use the appropriate issue template** (bug report or feature request) and fill it in completely — the domain context (NHI, HPI, ACC, FHIR resource type, etc.) helps maintainers triage faster.
3. **Be specific**: steps to reproduce, expected vs. actual behaviour, and any relevant module (e.g. `tpt-doctor`, `core/nhi`).
4. A maintainer or an internal AI agent will pick up the issue, implement the change directly against `master` (or a short-lived branch merged internally), and reference the issue number in the commit (`Closes #123` / `Refs #456`, per the commit message format above).
5. **Compliance-sensitive changes** (anything touching consent, audit, encryption, breach notification, or national integrations) get an additional internal compliance review before merging, regardless of how the change was proposed.
6. **CI must be green** (`make lint`, `make test`, `make build`, and the frontend `pnpm lint`/`pnpm build`) before a change lands on `master`.

---

## Adding New Modules

A "module" is a specialty-specific Go module in `modules/` (e.g., `tpt-doctor`, `tpt-pharmacy`).

### Steps to add a new module

1. Create the directory: `modules/<module-name>/`
2. Create `modules/<module-name>/go.mod`:
   ```
   module github.com/PhillipC05/tpt-healthcare/modules/<module-name>

   go 1.22

   require (
       github.com/PhillipC05/tpt-healthcare/core v0.0.0
   )

   replace github.com/PhillipC05/tpt-healthcare/core => ../../core
   ```
3. Add the module to `go.work`:
   ```
   use (
       ...
       ./modules/<module-name>
   )
   ```
4. Add tests and a `cmd/` entrypoint.
5. Add the module's binary to the root `Makefile` (`build`, `test`, `lint` targets).
6. Add a `deploy/docker-compose.<module>.yml` or extend `deploy/docker-compose.dev.yml`.
7. Document the module's clinical domain, NZ regulatory requirements, and any national integrations in `modules/<module-name>/README.md`.

### Module structure conventions

```
modules/<module-name>/
├── go.mod
├── cmd/<module-name>/
│   └── main.go          # Cobra root command
├── api/
│   ├── server.go        # HTTP server setup and middleware chain
│   └── *.go             # Route handlers
├── domain/              # Business logic (no I/O dependencies)
├── store/               # Database access (pgx)
└── testutil/            # Test helpers (not exported outside tests)
```

---

## Compliance Checklist

Before merging or pushing any change touching consent, audit, encryption, or national integrations, confirm:

- [ ] No PHI is logged at any log level.
- [ ] All new clinical writes produce an `AuditEvent` via `core/audit/trail.go`.
- [ ] Any new endpoint that returns health information checks consent via `core/consent/`.
- [ ] Any new prescribing or dispensing action validates the practitioner APC via `core/hpi/`.
- [ ] Any new field storing PHI uses `core/encryption/` for at-rest encryption.
- [ ] New migrations are append-only (no DROP COLUMN, no column renames without deprecation path).
- [ ] New national integration endpoints use UAT credentials in tests, not production.
- [ ] The change does not introduce any hardcoded secrets, keys, or credentials.
- [ ] The Privacy Act 2020 and HIPC data minimisation principle has been considered — only necessary data is collected and stored.
