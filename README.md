# tpt-healthcare-nz

Open-source New Zealand healthcare platform built on FHIR R5. Provides a compliant interoperability gateway, national system integrations (NHI, HPI, NES, ACC, PHARMAC), and specialty-specific clinical modules for NZ healthcare providers.

[![Go 1.22](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)](https://go.dev)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![FHIR R5](https://img.shields.io/badge/FHIR-R5-orange)](https://hl7.org/fhir/R5/)
[![HIPC 2020](https://img.shields.io/badge/HIPC-2020-blue)](https://www.privacy.org.nz/privacy-act-2020/codes-of-practice/hipc2020/)

---

## What it is

`tpt-healthcare-nz` is a multi-module Go platform for building NZ-compliant clinical applications. It handles the hard parts of NZ health IT — national system integrations, FHIR R5 persistence, HL7 v2 messaging, HIPC consent enforcement, and audit trail requirements — so specialty modules can focus on clinical logic.

The platform ships with modules for over 20 clinical specialties, a shared React component library, and patient and admin portals.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Frontend (pnpm)                    │
│  tpt-clinic (React)  tpt-portal  tpt-admin           │
│  @tpt/ui  @tpt/fhir-types  @tpt/api-client           │
└──────────────────────┬──────────────────────────────┘
                       │ FHIR R5 REST / WebSocket
┌──────────────────────▼──────────────────────────────┐
│            tpt-health-interop (Go)                   │
│  FHIR REST API · NHI · Terminology · Subscriptions  │
└──────────────────────┬──────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────┐
│                   core/ (Go)                         │
│  db · auth · audit · consent · encryption · events  │
│  nhi · hpi · nes · acc · pharmac · hl7 · fhir       │
└──────────┬──────────────────────────┬───────────────┘
           │                          │
  ┌────────▼────────┐        ┌────────▼────────┐
  │   PostgreSQL    │        │      Redis       │
  │  (FHIR + audit) │        │ (cache + pub/sub)│
  └─────────────────┘        └─────────────────┘
```

Specialty modules (`modules/tpt-doctor`, `modules/tpt-pharmacy`, etc.) import `core/` and are served alongside or independently of the interop gateway.

---

## Key Features

- **FHIR R5** resource store over PostgreSQL JSONB with full search parameter support
- **NHI** client — patient lookup, `$match`, and Luhn checksum validation
- **HPI** client — practitioner APC validation with 24-hour Redis cache
- **NES** — patient PHO enrolment, transfers, and status
- **ACC** — FHIR claim lodgement and ClaimResponse polling
- **PHARMAC** — formulary and subsidy lookup
- **HL7 v2** — ORU^R01, ADT, ORM parser with NZ lab Z-segment support and MLLP listener
- **SNOMED CT NZ**, **LOINC**, **ICD-10-AM**, **NZMT** terminology loaders
- **FHIR R5 Subscriptions** — rest-hook, WebSocket, and email channels via Redis pub/sub
- **Audit trail** — synchronous, append-only `AuditEvent` writes in the same DB transaction
- **Consent** — HIPC Rule 10/11 enforcement via `core/consent/`
- **AES-256-GCM** field encryption for all PHI at rest
- **Multi-tenant** — tenant extraction middleware, per-tenant RBAC
- **Observability** — OpenTelemetry tracing, Prometheus metrics, structured `slog` logging
- **PWA-ready** — encrypted IndexedDB offline store (`@tpt/offline-store`), PIN lock, Web Bluetooth vitals (`@tpt/diagnostics`)
- **Privacy Act 2020 & HIPC 2020** compliant by design

---

## Clinical Modules

| Module | Specialty |
|--------|-----------|
| `tpt-doctor` | General practice / primary care |
| `tpt-pharmacy` | Pharmacy dispensing and prescriptions |
| `tpt-hospital` | Hospital inpatient and outpatient |
| `tpt-mental-health` | Mental health (extra-sensitive consent) |
| `tpt-addiction` | Addiction and substance use |
| `tpt-aged-care` | Aged residential and community care |
| `tpt-maternal-child-health` | Maternity and child health |
| `tpt-oncology` | Oncology and cancer services |
| `tpt-radiology` | Radiology and imaging |
| `tpt-pathology` | Pathology and lab results |
| `tpt-cardiology` | Cardiology |
| `tpt-immunisation` | Immunisation register |
| `tpt-rehabilitation` | Rehabilitation services |
| `tpt-palliative` | Palliative care |
| `tpt-renal` | Renal and dialysis |
| `tpt-allied-health` | Allied health (physiotherapy, OT, etc.) |
| `tpt-community-health` | Community nursing and district health |
| `tpt-counselling` | Counselling and psychotherapy |
| `tpt-nutrition` | Dietetics and nutrition |
| `tpt-vision` | Optometry and ophthalmology |
| `tpt-chiropractic` | Chiropractic |
| `tpt-osteopathy` | Osteopathy |
| `tpt-acupuncture` | Acupuncture |
| `tpt-tcm` | Traditional Chinese Medicine |
| `tpt-massage` | Massage therapy |
| `tpt-naturopathy` | Naturopathy |
| `tpt-dental` | Dentistry |
| `tpt-disability` | Disability support services |
| `tpt-blood-bank` | Blood bank and transfusion |
| `tpt-clinical-trials` | Clinical trials management |
| `tpt-epidemiology` | Epidemiology and public health |
| `tpt-health-billing` | Health billing and claims |
| `tpt-practice` | Practice management |
| `tpt-screening` | Screening programmes |
| `tpt-telehealth` | Telehealth and virtual consultations |

---

## Prerequisites

- **Go 1.22+**
- **Node.js 20+** and **pnpm 9+**
- **PostgreSQL 16+** and **Redis 7+** — via Docker (see Quick Start) or natively (see [Without Docker](#without-docker))
- `golangci-lint` v1.60+ — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

---

## Quick Start

```bash
# 1. Clone
git clone https://github.com/PhillipC05/tpt-healthcare-nz.git
cd tpt-healthcare-nz

# 2. Configure environment
cp .env.example .env
# Edit .env — at minimum set ENCRYPTION_KEY (see .env.example for instructions)

# 3. Install frontend dependencies
pnpm install

# 4. Start the local dev stack (Postgres + Redis + interop)
make dev

# 5. Run database migrations
make migrate

# 6. Run all tests
make test
```

The interop gateway starts on `http://localhost:8080` by default.  
The clinic frontend starts on `http://localhost:3000`, portal on `http://localhost:3001`, and admin on `http://localhost:3002`.

---

## Without Docker

You can run PostgreSQL and Redis natively if Docker Desktop is not available.

### WSL2 (recommended on Windows)

```bash
sudo apt update && sudo apt install -y postgresql-16 redis-server
sudo service postgresql start
sudo service redis-server start
sudo -u postgres psql -c "CREATE USER tpt WITH PASSWORD 'tpt';"
sudo -u postgres psql -c "CREATE DATABASE tpt_healthcare_dev OWNER tpt;"
```

### Native Windows

**PostgreSQL**: `winget install PostgreSQL.PostgreSQL.16` or download from [postgresql.org](https://www.postgresql.org/download/windows/).

**Redis**: install [Memurai](https://www.memurai.com/) (Redis-compatible, free developer edition).

Then create the dev database in psql or pgAdmin:

```sql
CREATE USER tpt WITH PASSWORD 'tpt';
CREATE DATABASE tpt_healthcare_dev OWNER tpt;
```

### Native macOS/Linux

```bash
brew install postgresql@16 redis   # macOS
brew services start postgresql@16 redis
createuser -P tpt   # enter password: tpt
createdb -O tpt tpt_healthcare_dev
```

### After setting up the database

Use these values in your `.env` (matching the `.env.example` defaults):

```
DATABASE_URL=postgres://tpt:tpt@localhost:5432/tpt_healthcare_dev
REDIS_URL=redis://localhost:6379
```

Then build and run the interop server directly (no Docker):

```bash
make build
./bin/tpt-health-interop serve
```

---

### Generating an ENCRYPTION_KEY

```bash
# Generate a cryptographically random 32-byte key, base64-encoded
openssl rand -base64 32
```

Paste the output into `.env` as `ENCRYPTION_KEY=<value>`.

---

## Development Commands

| Command | Description |
|---------|-------------|
| `make dev` | Start Postgres, Redis, and the interop server via Docker Compose |
| `make build` | Build all Go binaries |
| `make test` | Run all Go unit + integration tests |
| `make test-race` | Run tests with the race detector (recommended before PRs) |
| `make lint` | Run `golangci-lint` across all Go modules |
| `make migrate` | Apply pending database migrations |
| `pnpm dev` | Start all frontend apps in watch mode |
| `pnpm build` | Build all frontend apps |
| `pnpm lint` | Lint all frontend packages |

---

## Compliance

This platform is designed for use in New Zealand's regulated health information environment:

| Requirement | Implementation |
|-------------|----------------|
| Privacy Act 2020 | Data minimisation, access rights, breach notification workflow (`core/breach/`) |
| HIPC Rule 5 | AES-256-GCM at-rest encryption, TLS in transit |
| HIPC Rules 10 & 11 | Consent checks before every read or disclosure (`core/consent/`) |
| HPCA 2003 | APC validation before clinical actions (`core/hpi/`) |
| Audit trail | Synchronous, append-only `AuditEvent` in the same DB transaction (`core/audit/`) |
| NHI | Checksum validation, encrypted storage, Ministry API via `core/nhi/` |

> **Note:** Using this platform in a production clinical environment requires valid API credentials from Te Whatu Ora (NHI, HPI, NES) and ACC. Always use UAT endpoints during development.

---

## Project Structure

```
tpt-healthcare-nz/
├── core/           # Shared Go module — DB, auth, FHIR, NZ integrations, audit
├── interop/        # tpt-health-interop service — FHIR REST gateway
├── modules/        # Specialty-specific Go modules (35 clinical specialties)
├── apps/           # Frontend applications (React + Vite PWAs)
├── packages/       # Shared frontend packages (@tpt/*)
├── deploy/         # Docker Compose files
├── tools/          # Code generation (FHIR type generator, icon generator)
└── installer/      # First-run wizard and installers
```

> **Go module namespace**: The GitHub repository is `tpt-healthcare-nz` but the Go module namespace used in all `go.mod` files is `github.com/PhillipC05/tpt-healthcare` (without `-nz`). These are different by design — see [CLAUDE.md](CLAUDE.md) for details.

Full layout and conventions: [CLAUDE.md](CLAUDE.md)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for branching strategy, code style, testing requirements, and the compliance checklist that all PRs must pass.

## Security

See [SECURITY.md](SECURITY.md) for the vulnerability disclosure policy. Do not open public GitHub issues for security vulnerabilities.

## License

[Apache 2.0](LICENSE) — Copyright 2024 tpt-healthcare-nz contributors
