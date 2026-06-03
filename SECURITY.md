# Security Policy

tpt-healthcare-nz handles sensitive New Zealand health information. Security is a first-class concern. This document describes our security policy, supported versions, vulnerability disclosure process, and the regulatory obligations that govern how we handle security incidents.

---

## Supported Versions

We provide security fixes for the following versions:

| Version | Supported |
|---------|-----------|
| `master` (latest) | Yes — security patches applied immediately |
| Last tagged release | Yes — backported security patches for 6 months post-release |
| Older releases | No — upgrade to the latest release |

We strongly recommend always running the latest release. Given the sensitivity of health information, running an unsupported version in a clinical environment is not acceptable.

---

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

### How to report

Send an encrypted email to:

**security@tpt.nz**

PGP public key fingerprint: *(to be published on keys.openpgp.org once the project is live)*

If you cannot use PGP, email `security@tpt.nz` with the subject line `[SECURITY] Vulnerability Report` and request a secure channel. We will respond with an alternative method (e.g., Signal, encrypted file share).

Include in your report:
- A description of the vulnerability and its potential impact.
- Steps to reproduce (proof-of-concept code or detailed reproduction steps).
- Which component(s) are affected (`core/`, `interop/`, a specific module, or the frontend).
- Any mitigating factors you have identified.
- Your preferred contact method for follow-up.

### Response SLA

| Milestone | Target |
|-----------|--------|
| Acknowledgement of receipt | Within 72 hours of your report |
| Initial severity assessment | Within 5 business days |
| Fix or mitigation timeline communicated | Within 10 business days |
| Critical vulnerabilities (CVSS 9.0+) patched | Within 14 days of confirmation |
| High vulnerabilities (CVSS 7.0–8.9) patched | Within 30 days of confirmation |
| Medium / low vulnerabilities | Next scheduled release |

We treat the 72-hour acknowledgement deadline as a hard commitment. If you do not receive a response within 72 hours, escalate to `phillip@tpt.nz`.

### Coordinated Disclosure

We follow coordinated (responsible) disclosure:

1. You report the vulnerability privately.
2. We confirm receipt and begin investigation within 72 hours.
3. We work with you to understand and reproduce the issue.
4. We develop and test a fix.
5. We notify affected operators if the vulnerability is already deployed.
6. We publish a security advisory and release a patch.
7. You may publish details of the vulnerability after we have released the fix, or after 90 days from the date of your report, whichever comes first.

We will credit you in the security advisory unless you prefer to remain anonymous.

We will not take legal action against researchers who report vulnerabilities in good faith and follow this disclosure policy.

---

## NZ Healthcare Security Obligations

### Privacy Act 2020

The Privacy Act 2020 imposes specific obligations on agencies that hold personal information, including health information:

- **Notifiable privacy breaches**: If a privacy breach has caused or is likely to cause serious harm to an individual, we must notify both the affected individual(s) and the Privacy Commissioner as soon as practicable — and in no case more than 72 hours after becoming aware of the breach. The notification workflow is implemented in `core/breach/`.
- **Serious harm threshold**: Factors that indicate serious harm include sensitivity of the information (health information is always sensitive), the number of individuals affected, and whether the information could be used for identity theft or discrimination.
- **Privacy Commissioner contacts**: Office of the Privacy Commissioner, 0800 803 909, `https://www.privacy.org.nz/`.

### Health Information Privacy Code (HIPC) 2020

The HIPC applies to all health agencies in New Zealand. Security-relevant rules:

**Rule 5 — Security safeguards**

> Health agencies must protect health information against loss, unauthorised access, use, modification, disclosure, or other misuse.

Our obligations under Rule 5:
- Encryption at rest (AES-256-GCM) for all PHI fields.
- Encryption in transit (TLS 1.2+ for all connections; TLS 1.3 preferred).
- Access controls enforced at the API layer; no direct database access for end users.
- Regular access reviews to ensure staff can only access records relevant to their role.
- Audit logging for all access to health records.
- Secure disposal of data when no longer needed.

**Rule 6 — Retention**

Health information must not be kept longer than required. Our retention policy:
- Clinical records: 10 years minimum (or until the patient turns 26, whichever is later) per the Health (Retention of Health Information) Regulations 1996.
- Audit records: 10 years minimum.
- Temporary data (caches, session tokens): purged per their defined TTL.

**Rule 10 — Limits on use**

Health information may only be used for the purpose for which it was collected. The consent module (`core/consent/`) enforces this at the API level.

**Rule 11 — Limits on disclosure**

Health information may only be disclosed to third parties with consent or under a specific HIPC exception. All disclosure endpoints must call `consent.Check()` before returning data.

### Health Practitioners Competence Assurance Act (HPCA) 2003

Clinical actions must be restricted to appropriately registered and scoped health practitioners. We enforce this by:
- Validating HPI APC status via `core/hpi/` on every clinical action.
- Asserting that the practitioner's scope of practice covers the action (e.g., a nurse practitioner prescribing within their endorsed scope).
- Logging APC validation failures as security events.

---

## Encryption Requirements

### At Rest

- All PHI fields in the database are encrypted with **AES-256-GCM** via `core/encryption/`.
- The encryption key is a 32-byte key loaded from the `ENCRYPTION_KEY` environment variable (base64-encoded).
- Key rotation is supported: the encryption package maintains a key version tag alongside each ciphertext blob.
- The PostgreSQL host must have full-disk encryption enabled (mandatory deployment requirement).
- Database backups are encrypted before being written to object storage using the same AES-256-GCM scheme.

### In Transit

- All HTTP endpoints must be served over TLS 1.2+. TLS 1.3 is preferred and should be enabled on all new deployments.
- Connections to national APIs (NHI, NES, HPI, ACC) use mutual TLS where the API requires it.
- Internal service-to-service communication within a deployment must use TLS or a secure overlay network.
- Do not disable certificate verification (`InsecureSkipVerify = true`) in any code path.

### Key Management

- Encryption keys must never be committed to version control.
- Keys must never appear in logs, error messages, or API responses.
- For production deployments, keys should be stored in a secrets manager (AWS Secrets Manager, HashiCorp Vault, or equivalent) and injected as environment variables at runtime.
- Key rotation should be performed at least annually or immediately following a suspected compromise.

---

## Audit Trail Requirements

A complete, tamper-evident audit trail is required by HIPC Rule 5 and good clinical governance practice.

- Every read and write of a health record produces an `AuditEvent` (FHIR R5 resource) written to `audit_events`.
- Audit records are **append-only**. PostgreSQL rules enforce that no UPDATE or DELETE is possible on `audit_events`.
- Each audit record includes:
  - Timestamp (UTC, microsecond precision)
  - Actor identity (Principal: user ID, practitioner HPI CPN, or system identity)
  - Patient NHI (encrypted)
  - Resource type and ID
  - Action (read / create / update / delete)
  - Source IP address
  - Request correlation/trace ID
  - Outcome (success / failure)
- Audit records are retained for a minimum of 10 years.
- Audit records must be stored in a separate, access-controlled schema or database to reduce the blast radius of an application-layer compromise.
- Access to raw audit records is restricted to the audit service account and administrators with documented need.

---

## Access Control

- All API endpoints require authentication. There are no unauthenticated endpoints except the OIDC discovery document and health check.
- Role-based access control (RBAC) is enforced at the API layer before any data access.
- The principle of least privilege applies: service accounts have the minimum database permissions required.
- Multi-factor authentication (MFA) is required for all practitioner accounts and all admin accounts.
- Session tokens expire after 8 hours of inactivity. Refresh tokens expire after 30 days.

---

## Dependency Security

- Dependencies are pinned to exact versions in `go.sum` and `pnpm-lock.yaml`.
- Dependabot or Renovate is configured to open PRs for dependency updates automatically.
- `govulncheck` is run in CI on every PR to detect known Go vulnerabilities.
- `pnpm audit` is run in CI for frontend dependencies.
- Any dependency with a known critical or high vulnerability must be updated within 14 days.

---

## Penetration Testing

- A penetration test is required before any production launch of a new module or API surface.
- Penetration tests must cover OWASP Top 10, FHIR-specific attack surfaces (resource enumeration, bulk data export abuse), and NZ-specific risks (NHI enumeration).
- Test results are reviewed by the security lead and tracked as compliance issues.
- Findings of critical or high severity must be remediated before launch.

---

## Incident Response

In the event of a confirmed or suspected security incident:

1. **Contain**: isolate affected systems to prevent further damage.
2. **Assess**: determine the scope and whether PHI was accessed or exfiltrated.
3. **Notify internally**: alert the security lead and management immediately.
4. **Notify affected parties**: if PHI was breached and meets the serious harm threshold, notify the Privacy Commissioner within 72 hours and affected patients as soon as practicable.
5. **Preserve evidence**: do not wipe or restart systems until forensic evidence is preserved.
6. **Remediate**: patch the vulnerability, revoke compromised credentials.
7. **Post-incident review**: document root cause, timeline, and lessons learned within 2 weeks of resolution.

For Privacy Commissioner notification: `https://www.privacy.org.nz/tools/privacy-breach-notification-tool/`
