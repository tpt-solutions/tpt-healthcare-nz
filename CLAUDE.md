# CLAUDE.md — tpt-healthcare-nz Codebase Conventions

This file documents conventions, layout, and patterns for the tpt-healthcare-nz project.
Read this before making any changes.

---

## Repository Layout

```
tpt-healthcare-nz/
├── core/                        # Shared kernel Go module
│   ├── go.mod                   # module github.com/PhillipC05/tpt-healthcare/core
│   ├── db/                      # pgx connection pool + migration runner
│   │   ├── connect.go
│   │   └── migrate/             # Embedded SQL migrations (*.sql files)
│   ├── audit/                   # Synchronous audit trail writes
│   ├── auth/                    # AuthProvider interface + implementations
│   │   ├── provider.go          # Interface + Principal struct
│   │   ├── auth0/               # Auth0 OIDC validation
│   │   ├── jwt/                 # Ed25519 JWT + TOTP
│   │   └── oidc/                # tpt-identity OIDC client
│   ├── consent/                 # HIPC Rule 10/11 consent management
│   ├── encryption/              # AES-256-GCM field encryption
│   ├── events/                  # Internal domain event bus
│   ├── fhir/
│   │   ├── r5/                  # Generated FHIR R5 Go structs
│   │   ├── r4/                  # Minimal R4 types for NHI/NES API compat
│   │   └── translate/           # R4<->R5 translators
│   ├── hpi/                     # Health Practitioner Index APC validation
│   ├── middleware/              # rate limit, CORS, tenant extraction, audit wrapping
│   ├── nhi/                     # NHI FHIR API client
│   ├── nes/                     # NES enrolment client
│   ├── acc/                     # ACC FHIR claim lodgement
│   ├── pharmac/                 # PHARMAC formulary + subsidy lookup
│   ├── hl7/                     # HL7 v2 parser (ORU^R01, ADT, ORM)
│   ├── repo/                    # FHIR repository interfaces and implementations
│   ├── subscription/            # FHIR R5 subscription engine (Redis pub/sub)
│   └── terminology/             # SNOMED CT, LOINC, ICD-10-AM, NZMT loaders
│
├── interop/                     # tpt-health-interop service Go module
│   ├── go.mod                   # module github.com/PhillipC05/tpt-healthcare/interop
│   ├── cmd/tpt-health-interop/  # Cobra CLI entrypoint (serve, migrate, validate)
│   └── api/                     # HTTP server and FHIR REST API handlers
│
├── modules/                     # Per-specialty Go modules (tpt-doctor, tpt-pharmacy, etc.)
│   └── tpt-doctor/
│       └── go.mod               # module github.com/PhillipC05/tpt-healthcare/modules/tpt-doctor
│
├── apps/                        # Frontend applications (pnpm workspace)
│   ├── tpt-clinic/              # Clinician-facing React/Vite app
│   ├── tpt-portal/              # Patient portal
│   └── tpt-admin/               # Practice admin + billing dashboard
│
├── packages/                    # Shared frontend packages (pnpm workspace)
│   ├── fhir-types/              # @tpt/fhir-types — FHIR R5 + NZ extensions
│   ├── ui/                      # @tpt/ui — shared React component library (Tailwind)
│   ├── api-client/              # @tpt/api-client — openapi-typescript generated client
│   └── nz-codes/                # @tpt/nz-codes — NHI validation, ACC codes, NZ URIs
│
├── deploy/
│   ├── docker-compose.dev.yml   # Local dev stack (Postgres + Redis + interop)
│   └── docker-compose.yml       # Full production-like stack
│
├── installer/                   # curl|bash + PowerShell installers, first-run wizard
├── tools/                       # Code generation tools (gen-fhir-types, etc.)
├── go.work                      # Go workspace linking core/, interop/, modules/*
├── pnpm-workspace.yaml          # pnpm workspace linking apps/* and packages/*
├── Makefile                     # make dev / test / build / lint / install
├── CLAUDE.md                    # This file
├── CONTRIBUTING.md
└── SECURITY.md
```

---

## Go Module Structure

The Go workspace (`go.work`) links all Go modules so they can be developed together without publishing:

```
go 1.22

use (
    ./core
    ./interop
    ./modules/tpt-doctor
    ./modules/tpt-pharmacy
    // ... future modules
)
```

### Module naming convention

| Module | Path |
|--------|------|
| `github.com/PhillipC05/tpt-healthcare/core` | `./core` |
| `github.com/PhillipC05/tpt-healthcare/interop` | `./interop` |
| `github.com/PhillipC05/tpt-healthcare/modules/tpt-doctor` | `./modules/tpt-doctor` |

Downstream modules depend on `core` via a `replace` directive pointing to `../core` in their `go.mod`, in addition to the workspace. This ensures the module resolves correctly both inside and outside the workspace.

---

## Frontend Workspace

pnpm workspaces (`pnpm-workspace.yaml`) include:

- `apps/*` — deployable applications
- `packages/*` — shared libraries

Always run `pnpm install` from the repository root. Never run `npm install` or `yarn`; use `pnpm` exclusively.

Package naming: all shared packages use the `@tpt/` scope (e.g., `@tpt/ui`, `@tpt/fhir-types`).

---

## Coding Conventions

### Go

- **Go version**: 1.22 (see `go.work` and each `go.mod`)
- **Formatting**: `gofmt` — no exceptions. Run `gofmt -w ./...` before committing.
- **Linting**: `golangci-lint run ./...`. The `.golangci.yml` config at root governs enabled linters.
- **Database**: `github.com/jackc/pgx/v5` for all PostgreSQL access. Never use `database/sql` directly.
  - Use `pgx.Pool` (not single connections) for all service-level code.
  - Prefer named parameters (`@param_name`) with `pgx` named argument syntax.
  - All queries go in `*_query.go` files alongside the struct they serve.
- **Error handling**: always wrap errors with context using `fmt.Errorf("operation: %w", err)`. Never swallow errors silently.
- **Context propagation**: every function that does I/O must accept `context.Context` as its first argument.
- **Logging**: use `slog` (stdlib). Pass a `*slog.Logger` via context or dependency injection. No global loggers.
- **Configuration**: `github.com/spf13/viper` for all config. Environment variables take precedence over config files.
- **HTTP**: use `net/http` + `otelhttp` middleware. No external HTTP router framework required unless complexity demands it.
- **Testing**: `github.com/stretchr/testify` for assertions. Integration tests use `testcontainers-go` to spin up real Postgres/Redis.

### FHIR R5

- All FHIR resource types live in `core/fhir/r5/`. These are generated from the FHIR R5 JSON schema (`tools/gen-fhir-types/`). Do not hand-edit generated files.
- NZ-specific FHIR extensions use the canonical URI base `https://standards.digital.health.nz/`.
- Always validate FHIR resources against the relevant NZ Base IG profile before persisting.
- FHIR search parameters are implemented in `core/repo/search.go` over PostgreSQL JSONB columns.
- For R4 compatibility (NHI/NES APIs), use types from `core/fhir/r4/` and translate via `core/fhir/translate/`.

### NZ-Specific Patterns

#### NHI (National Health Index)

- NHI format: `[A-Z]{3}[0-9]{4}` (old) or `[A-Z]{3}[0-9]{2}[A-Z]{2}` (new Luhn-based). Always validate checksum before sending to the Ministry API.
- Use the client in `core/nhi/` for all NHI lookups and `$match` operations.
- Never store a patient's NHI in plaintext outside the encrypted FHIR `Patient` resource and the `nhi` index column (which is itself encrypted at rest).

#### HPI (Health Practitioner Index)

- Validate practitioner APC (Annual Practising Certificate) status via `core/hpi/` before allowing clinical actions.
- Results are cached in Redis for 24 hours (TTL configurable). The cache key is `hpi:apc:{hpi_cpn}`.

#### ACC Claiming

- All ACC claims must reference a valid ACC45 or ACC6 form number.
- ACC FHIR resources use the `https://standards.digital.health.nz/ns/acc-` URI namespace for identifiers.
- Claim status polling is handled by a background worker in `core/acc/`.

#### Audit Trail

- Every write to a clinical resource must produce an `AuditEvent` (FHIR R5) via `core/audit/trail.go`.
- Audit records are append-only. No update or delete operations are permitted on the `audit_events` table.
- The audit writer is synchronous within the same database transaction as the clinical write — no eventual consistency.

#### Consent

- `core/consent/` implements HIPC Rules 10 and 11 (access and disclosure).
- Before returning any health information to a third party, check consent with `consent.Check(ctx, patientID, requesterID, purpose)`.
- Mental health records carry an extra-sensitive flag; access requires an elevated consent check.

---

## Running Tests

```bash
# All Go unit + integration tests
make test

# With race detector (recommended before PRs)
make test-race

# Single package
go test ./core/nhi/...

# With verbose output
go test -v ./interop/...
```

Integration tests require Docker (for `testcontainers-go`). They spin up ephemeral Postgres and Redis containers automatically.

---

## Adding Migrations

SQL migrations live in `core/db/migrate/` and are embedded into the binary via `go:embed`.

1. Create a new file: `core/db/migrate/NNN_description.sql` where `NNN` is the next sequential number (zero-padded to 3 digits, e.g., `003_consent_table.sql`).
2. Write ANSI-compatible SQL. Use `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` so migrations are idempotent on re-run.
3. Never drop columns in a migration. Prefer soft deprecation (add new column, migrate data, remove old column in a later release after all services have updated).
4. Run migrations locally: `make migrate` (requires the interop binary to be built first via `make build`).
5. In tests, migrations run automatically via the `testcontainers-go` setup helper.

Migration filename examples:
```
001_fhir_resources.sql
002_audit_events.sql
003_consent_table.sql
004_hpi_cache.sql
```

---

## Compliance Notes

### Privacy Act 2020 (NZ)

- Health information is "sensitive personal information" under the Privacy Act 2020.
- All collection must have a clear, documented lawful purpose.
- Data minimisation: collect and store only what is necessary for the stated purpose.
- Individuals have the right to access and correct their own health information (FHIR Patient `$everything` supports this).
- Notifiable privacy breaches (privacy breach that has caused or is likely to cause serious harm) must be reported to the Privacy Commissioner within 72 hours. The workflow lives in `core/breach/`.

### Health Information Privacy Code (HIPC) 2020

The HIPC has 13 rules that govern health information. Key rules for this codebase:

| Rule | Implication |
|------|-------------|
| Rule 1 | Collect only for a lawful purpose directly related to the agency's function |
| Rule 2 | Collect from the individual unless an exception applies |
| Rule 5 | Keep health information secure (encryption at rest + in transit) |
| Rule 6 | No longer keep than necessary |
| Rule 10 | Use only for the purpose for which collected |
| Rule 11 | Disclose only with consent or under a specific exception |
| Rule 12 | Unique identifiers (NHI) only from the assigning agency |

### Encryption Requirements

- **At rest**: All PHI (protected health information) fields are encrypted with AES-256-GCM via `core/encryption/`. The encryption key is loaded from environment (`ENCRYPTION_KEY`, 32-byte base64).
- **In transit**: TLS 1.2+ required for all external connections. TLS 1.3 preferred.
- **Database**: Full-disk encryption on the Postgres host is a deployment requirement.
- **Backups**: Backups are encrypted with the same AES-256-GCM key before being written to object storage.

### HPCA (Health Practitioners Competence Assurance Act 2003)

- Each module must enforce that clinical actions are only available to practitioners with the appropriate APC and scope of practice.
- The HPI validation in `core/hpi/` returns the practitioner's registration authority and scope of practice. The calling module is responsible for asserting the scope matches the action.

### Audit Trail Requirements

- All reads and writes of health records must be logged in `audit_events`.
- Audit records must include: timestamp (UTC), actor (Principal), patient NHI (encrypted), resource type, resource ID, action (read/write/delete), source IP, and request correlation ID.
- Audit records must be retained for a minimum of 10 years.
- Audit records are immutable (no UPDATE or DELETE on `audit_events`).
