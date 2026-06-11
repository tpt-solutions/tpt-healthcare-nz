# tpt-healthcare-nz

Open-source New Zealand healthcare platform built on FHIR R5. Provides a compliant interoperability gateway, national system integrations (NHI, HPI, NES, ACC, PHARMAC), and specialty-specific clinical modules for NZ healthcare providers.

[![Go 1.22](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
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
- **Privacy Act 2020 & HIPC 2020** compliant by design

---

## Clinical Modules

| Module | Specialty |
|--------|-----------|
| `tpt-doctor` | General practice / primary care |
| `tpt-pharmacy` | Pharmacy dispensing and prescriptions |
| `tpt-hospital` | Hospital inpatient and outpatient |
| `tpt-mental-health` | Mental health (extra-sensitive consent) |
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
| `tpt-mental-health` | Addiction and substance use |
| `tpt-counselling` | Counselling and psychotherapy |
| `tpt-nutrition` | Dietetics and nutrition |
| `tpt-vision` | Optometry and ophthalmology |
| `tpt-chiropractic` | Chiropractic |
| `tpt-osteopathy` | Osteopathy |
| `tpt-acupuncture` | Acupuncture and TCM |
| `tpt-massage` | Massage therapy |
| `tpt-naturopathy` | Naturopathy |

---

## Prerequisites

- **Go 1.22+**
- **Node.js 20+** and **pnpm 9+**
- **Docker Desktop** (or Docker Engine + Compose plugin)
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
The clinic frontend starts on `http://localhost:5173`.

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
├── modules/        # Specialty-specific Go modules
├── apps/           # Frontend applications (React + Vite)
├── packages/       # Shared frontend packages (@tpt/*)
├── deploy/         # Docker Compose files
├── tools/          # Code generation (FHIR type generator)
└── installer/      # First-run wizard and installers
```

Full layout and conventions: [CLAUDE.md](CLAUDE.md)

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for branching strategy, code style, testing requirements, and the compliance checklist that all PRs must pass.

## Security

See [SECURITY.md](SECURITY.md) for the vulnerability disclosure policy. Do not open public GitHub issues for security vulnerabilities.

## License

[MIT](LICENSE) — Copyright (c) 2024 tpt-healthcare-nz contributors
