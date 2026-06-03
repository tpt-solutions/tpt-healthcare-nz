# Contributing to tpt-healthcare-nz

Thank you for contributing to tpt-healthcare-nz. Because this codebase handles sensitive New Zealand health information, contributions are held to a higher standard than a typical open-source project. Please read this document fully before opening a pull request.

---

## Table of Contents

1. [NZ Healthcare Domain Knowledge](#nz-healthcare-domain-knowledge)
2. [Branching Strategy](#branching-strategy)
3. [Development Setup](#development-setup)
4. [Code Style](#code-style)
5. [Testing Requirements](#testing-requirements)
6. [Commit Message Format](#commit-message-format)
7. [Pull Request Process](#pull-request-process)
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

We use a trunk-based development model with short-lived feature branches.

| Branch | Purpose |
|--------|---------|
| `master` | Production-ready code. Protected. Merges require at least one reviewer approval. |
| `feat/<scope>/<description>` | New features (e.g., `feat/nhi/patient-match-endpoint`) |
| `fix/<scope>/<description>` | Bug fixes (e.g., `fix/acc/claim-status-polling`) |
| `chore/<description>` | Tooling, dependencies, CI (e.g., `chore/update-pgx-v5`) |
| `docs/<description>` | Documentation-only changes |
| `compliance/<description>` | Compliance or security-driven changes |

Branch naming rules:
- Use lowercase and hyphens only (no underscores, no slashes within the description segment).
- Keep descriptions concise (3–6 words).
- Delete feature branches after merge.

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
git clone https://github.com/PhillipC05/tpt-healthcare-nz.git
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

- **Formatting**: `gofmt -w ./...` — enforced by CI. No PRs will be merged with formatting errors.
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

All PRs must include tests. The bar varies by change type:

| Change type | Required tests |
|-------------|----------------|
| New Go function | Unit test with table-driven cases |
| New HTTP endpoint | Integration test with real Postgres (testcontainers-go) |
| New NZ national integration (NHI, NES, HPI, ACC) | Integration test using the UAT environment with VCR-style HTTP recording for CI |
| New migration | Test that migration runs cleanly on a fresh DB and is idempotent |
| FHIR resource handling | Round-trip test (create → read → validate against NZ Base IG profile) |
| Audit trail | Test that every write produces the correct AuditEvent |
| Frontend component | Vitest unit test + Playwright smoke test for critical paths |

### Test conventions

- Test files are `*_test.go` in the same package as the code under test.
- Use `t.Parallel()` in unit tests.
- Integration tests are tagged with `//go:build integration` and must be runnable with `go test -tags=integration ./...`.
- Use `testify/assert` for assertions; `testify/require` when test cannot continue after failure.
- Test helpers in `testutil/` within each module — do not export test helpers into production packages.

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

## Pull Request Process

1. **Open a draft PR** early so reviewers can follow your progress.
2. **Fill in the PR template** completely. Do not delete sections.
3. **Self-review** your diff before requesting review. Remove debug logging, commented-out code, and TODO comments that belong in issues.
4. **CI must be green**: all tests, linting, and formatting checks must pass.
5. **At least one approval** from a maintainer is required before merge.
6. **Compliance changes** (anything touching consent, audit, encryption, breach notification, or national integrations) require an additional review from the compliance lead.
7. **Squash merge** is the default merge strategy. The PR title becomes the commit message — ensure it follows the commit message format above.
8. **Delete the branch** after merge.

### PR template sections

- Summary: what does this PR do and why?
- Related issues
- Type of change (feature / bug fix / refactor / compliance / docs)
- Testing: describe how you tested the changes
- Compliance checklist (see below)
- Screenshots (for UI changes)

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

Before marking a PR as ready for review, confirm:

- [ ] No PHI is logged at any log level.
- [ ] All new clinical writes produce an `AuditEvent` via `core/audit/trail.go`.
- [ ] Any new endpoint that returns health information checks consent via `core/consent/`.
- [ ] Any new prescribing or dispensing action validates the practitioner APC via `core/hpi/`.
- [ ] Any new field storing PHI uses `core/encryption/` for at-rest encryption.
- [ ] New migrations are append-only (no DROP COLUMN, no column renames without deprecation path).
- [ ] New national integration endpoints use UAT credentials in tests, not production.
- [ ] The PR does not introduce any hardcoded secrets, keys, or credentials.
- [ ] The Privacy Act 2020 and HIPC data minimisation principle has been considered — only necessary data is collected and stored.
