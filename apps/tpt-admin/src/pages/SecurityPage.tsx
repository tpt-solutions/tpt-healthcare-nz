import { formatDateTime, formatDate } from '../utils/format';

/**
 * Security & compliance dashboard.
 * Surfaces HIPC compliance checklist, breach notifications (core/breach/),
 * data retention status, and active sessions.
 */

type CheckStatus = 'pass' | 'fail' | 'warning' | 'na';

interface ComplianceCheck {
  id: string;
  rule: string;
  description: string;
  status: CheckStatus;
  detail: string;
  lastChecked: string;
}

interface BreachNotification {
  id: string;
  detectedAt: string;
  reportedAt: string | null;
  severity: 'low' | 'medium' | 'high';
  description: string;
  status: 'detected' | 'reported' | 'resolved';
  notifiedPrivacyCommissioner: boolean;
}

interface ActiveSession {
  id: string;
  user: string;
  role: string;
  ip: string;
  startedAt: string;
  lastActivity: string;
  userAgent: string;
}

interface RetentionStatus {
  table: string;
  oldestRecord: string;
  totalRecords: number;
  retentionYears: number;
  status: 'ok' | 'warning';
}

const complianceChecks: ComplianceCheck[] = [
  { id: 'enc-rest', rule: 'HIPC Rule 5', description: 'PHI encrypted at rest (AES-256-GCM)', status: 'pass', detail: 'All PHI fields encrypted via core/encryption/. Key loaded from ENCRYPTION_KEY env var.', lastChecked: '2026-06-03T08:00:00Z' },
  { id: 'enc-transit', rule: 'HIPC Rule 5', description: 'TLS 1.2+ on all external connections', status: 'pass', detail: 'TLS 1.3 active on all endpoints. TLS 1.0/1.1 disabled.', lastChecked: '2026-06-03T08:00:00Z' },
  { id: 'audit-trail', rule: 'Privacy Act 2020', description: 'Audit trail for all health record access', status: 'pass', detail: 'Synchronous audit writes to audit_events table within the same DB transaction. Verified immutable.', lastChecked: '2026-06-03T08:01:00Z' },
  { id: 'consent-gate', rule: 'HIPC Rule 11', description: 'Consent gate active for third-party disclosures', status: 'pass', detail: 'consent.Check() called before all third-party disclosures. Mental health extra-sensitive flag enforced.', lastChecked: '2026-06-03T08:01:00Z' },
  { id: 'nhi-encrypt', rule: 'HIPC Rule 12', description: 'NHI stored encrypted and from assigning agency only', status: 'pass', detail: 'NHI encrypted at rest. All lookups via core/nhi/ NHI FHIR API client.', lastChecked: '2026-06-03T08:01:00Z' },
  { id: 'hpi-apc', rule: 'HPCA Act 2003', description: 'APC validation before clinical actions', status: 'warning', detail: '1 practitioner (Dr. Sione Tuilagi) has APC expiring in 27 days. No expired APCs blocking access currently.', lastChecked: '2026-06-03T09:00:00Z' },
  { id: 'retention', rule: 'Privacy Act 2020 / HIPC Rule 6', description: 'Records retained for minimum 10 years', status: 'pass', detail: 'Oldest audit record: 2016-01-01. Automated retention policy active. No purge operations on clinical data.', lastChecked: '2026-06-03T08:00:00Z' },
  { id: 'breach-72h', rule: 'Privacy Act 2020 s113', description: 'Breach notification within 72 hours of detection', status: 'pass', detail: 'No open breach notifications. Last breach resolved 2025-11-14.', lastChecked: '2026-06-03T08:00:00Z' },
  { id: 'mfa', rule: 'HIPC Rule 5', description: 'MFA enforced for all clinical users', status: 'pass', detail: 'TOTP-based MFA via core/auth/jwt/ enforced for all users with clinical role.', lastChecked: '2026-06-03T08:00:00Z' },
  { id: 'backup', rule: 'HIPC Rule 5', description: 'Encrypted backups to object storage', status: 'warning', detail: 'Last successful backup: 2026-06-03T03:00:00Z (OK). Backup encryption key rotation due in 14 days.', lastChecked: '2026-06-03T03:05:00Z' },
];

const breachNotifications: BreachNotification[] = [
  {
    id: 'breach-001',
    detectedAt: '2025-11-10T14:22:00Z',
    reportedAt: '2025-11-11T09:00:00Z',
    severity: 'low',
    description: 'Single patient record accessed by staff member outside their care team. No data exfiltration. Staff counselled.',
    status: 'resolved',
    notifiedPrivacyCommissioner: false,
  },
];

const activeSessions: ActiveSession[] = [
  { id: 'sess-001', user: 'Dr. Hemi Walker', role: 'GP', ip: '10.1.2.45', startedAt: '2026-06-03T09:30:00Z', lastActivity: '2026-06-03T14:58:00Z', userAgent: 'Chrome 124 / Windows 11' },
  { id: 'sess-002', user: 'Nurse Mere Parata', role: 'Nurse Practitioner', ip: '10.1.2.47', startedAt: '2026-06-03T08:00:00Z', lastActivity: '2026-06-03T14:50:00Z', userAgent: 'Chrome 124 / macOS 14' },
  { id: 'sess-003', user: 'Tama Parata', role: 'Admin', ip: '10.1.4.12', startedAt: '2026-06-03T11:30:00Z', lastActivity: '2026-06-03T15:00:00Z', userAgent: 'Firefox 125 / Windows 11' },
];

const retentionData: RetentionStatus[] = [
  { table: 'audit_events', oldestRecord: '2016-01-15', totalRecords: 2847293, retentionYears: 10, status: 'ok' },
  { table: 'fhir_resources (Patient)', oldestRecord: '2016-03-01', totalRecords: 5210, retentionYears: 10, status: 'ok' },
  { table: 'fhir_resources (Encounter)', oldestRecord: '2016-03-15', totalRecords: 184732, retentionYears: 10, status: 'ok' },
  { table: 'fhir_resources (Consent)', oldestRecord: '2020-01-01', totalRecords: 48230, retentionYears: 10, status: 'ok' },
];

function checkBadge(status: CheckStatus) {
  const map = {
    pass: 'bg-green-100 text-green-700',
    fail: 'bg-red-100 text-red-700',
    warning: 'bg-amber-100 text-amber-700',
    na: 'bg-gray-100 text-gray-500',
  };
  const icons = { pass: '✓', fail: '✗', warning: '!', na: '—' };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium ${map[status]}`}>
      {icons[status]} {status === 'pass' ? 'Pass' : status === 'fail' ? 'Fail' : status === 'warning' ? 'Warning' : 'N/A'}
    </span>
  );
}

function severityBadge(severity: BreachNotification['severity']) {
  const map = { low: 'bg-blue-100 text-blue-700', medium: 'bg-amber-100 text-amber-700', high: 'bg-red-100 text-red-700' };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${map[severity]}`}>
      {severity}
    </span>
  );
}

export function SecurityPage() {
  const passCount = complianceChecks.filter(c => c.status === 'pass').length;
  const failCount = complianceChecks.filter(c => c.status === 'fail').length;
  const warnCount = complianceChecks.filter(c => c.status === 'warning').length;
  const openBreaches = breachNotifications.filter(b => b.status !== 'resolved').length;

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Security & Compliance</h1>
        <p className="mt-1 text-sm text-gray-500">
          HIPC compliance checklist, breach notifications, data retention, and active sessions.
        </p>
      </div>

      {/* Summary row */}
      <div className="grid grid-cols-4 gap-4 mb-8">
        <div className={`rounded-xl border shadow-sm p-4 ${failCount > 0 ? 'bg-red-50 border-red-200' : 'bg-green-50 border-green-200'}`}>
          <p className={`text-xs font-medium uppercase tracking-wide ${failCount > 0 ? 'text-red-600' : 'text-green-600'}`}>Compliance</p>
          <p className={`text-2xl font-bold mt-1 ${failCount > 0 ? 'text-red-700' : 'text-green-700'}`}>{passCount}/{complianceChecks.length}</p>
          <p className="text-xs text-gray-500">checks passing</p>
        </div>
        <div className={`rounded-xl border shadow-sm p-4 ${warnCount > 0 ? 'bg-amber-50 border-amber-200' : 'bg-white border-gray-200'}`}>
          <p className="text-xs font-medium text-amber-600 uppercase tracking-wide">Warnings</p>
          <p className="text-2xl font-bold text-amber-700 mt-1">{warnCount}</p>
          <p className="text-xs text-gray-500">need attention</p>
        </div>
        <div className={`rounded-xl border shadow-sm p-4 ${openBreaches > 0 ? 'bg-red-50 border-red-200' : 'bg-white border-gray-200'}`}>
          <p className="text-xs font-medium uppercase tracking-wide text-gray-500">Open Breaches</p>
          <p className={`text-2xl font-bold mt-1 ${openBreaches > 0 ? 'text-red-700' : 'text-gray-900'}`}>{openBreaches}</p>
          <p className="text-xs text-gray-500">requiring notification</p>
        </div>
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-4">
          <p className="text-xs font-medium text-gray-500 uppercase tracking-wide">Active Sessions</p>
          <p className="text-2xl font-bold text-gray-900 mt-1">{activeSessions.length}</p>
          <p className="text-xs text-gray-500">users online now</p>
        </div>
      </div>

      {/* HIPC Compliance checklist */}
      <section className="mb-8">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">HIPC Compliance Checklist</h2>
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 bg-gray-50">
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Rule</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Check</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Detail</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Last Checked</th>
                <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {complianceChecks.map(check => (
                <tr key={check.id} className={check.status === 'fail' ? 'bg-red-50' : check.status === 'warning' ? 'bg-amber-50/50' : ''}>
                  <td className="px-5 py-3">
                    <span className="text-xs font-mono bg-gray-100 rounded px-1.5 py-0.5 text-gray-700 whitespace-nowrap">{check.rule}</span>
                  </td>
                  <td className="px-5 py-3">
                    <p className="text-sm font-medium text-gray-900">{check.description}</p>
                  </td>
                  <td className="px-5 py-3 text-xs text-gray-500 max-w-[280px]">{check.detail}</td>
                  <td className="px-5 py-3 text-xs text-gray-400 whitespace-nowrap">{formatDateTime(check.lastChecked)}</td>
                  <td className="px-5 py-3 text-right">{checkBadge(check.status)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Breach notifications */}
      <section className="mb-8">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">
          Breach Notification Log (core/breach/)
        </h2>
        <div className="bg-blue-50 border border-blue-200 rounded-xl px-4 py-3 mb-3">
          <p className="text-xs text-blue-800">
            <span className="font-semibold">Privacy Act 2020 s113:</span> Notifiable privacy breaches (those that have
            caused or are likely to cause serious harm) must be reported to the Privacy Commissioner within 72 hours of
            detection. The workflow in core/breach/ tracks detection, internal review, and notification status.
          </p>
        </div>
        {breachNotifications.length === 0 ? (
          <div className="bg-white rounded-xl border border-gray-200 p-6 text-center">
            <p className="text-sm text-gray-400">No breach notifications recorded.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {breachNotifications.map(breach => (
              <div key={breach.id} className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      {severityBadge(breach.severity)}
                      <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${
                        breach.status === 'resolved' ? 'bg-green-100 text-green-700' :
                        breach.status === 'reported' ? 'bg-blue-100 text-blue-700' : 'bg-red-100 text-red-700'
                      }`}>{breach.status}</span>
                      {!breach.notifiedPrivacyCommissioner && breach.status !== 'resolved' && (
                        <span className="inline-flex rounded-full bg-red-100 text-red-700 px-2.5 py-0.5 text-xs font-medium">
                          Commissioner not notified
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-gray-700">{breach.description}</p>
                    <div className="flex gap-6 mt-2">
                      <p className="text-xs text-gray-400">Detected: {formatDateTime(breach.detectedAt)}</p>
                      {breach.reportedAt && (
                        <p className="text-xs text-gray-400">Reported internally: {formatDateTime(breach.reportedAt)}</p>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>

      {/* Data retention */}
      <section className="mb-8">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">Data Retention Status</h2>
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 bg-gray-50">
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Table / Resource</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Oldest Record</th>
                <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Total Records</th>
                <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Retention Req.</th>
                <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {retentionData.map(row => (
                <tr key={row.table}>
                  <td className="px-5 py-3">
                    <code className="text-xs text-gray-700">{row.table}</code>
                  </td>
                  <td className="px-5 py-3 text-xs text-gray-600">{formatDate(row.oldestRecord)}</td>
                  <td className="px-5 py-3 text-right text-xs text-gray-600">{row.totalRecords.toLocaleString('en-NZ')}</td>
                  <td className="px-5 py-3 text-right text-xs text-gray-600">{row.retentionYears} years</td>
                  <td className="px-5 py-3 text-right">
                    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      row.status === 'ok' ? 'bg-green-100 text-green-700' : 'bg-amber-100 text-amber-700'
                    }`}>
                      {row.status === 'ok' ? 'OK' : 'Warning'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Active sessions */}
      <section>
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">Active Sessions</h2>
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 bg-gray-50">
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">User</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">IP Address</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Started</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Last Activity</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">User Agent</th>
                <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {activeSessions.map(session => (
                <tr key={session.id}>
                  <td className="px-5 py-3">
                    <p className="text-sm font-medium text-gray-900">{session.user}</p>
                    <p className="text-xs text-gray-400">{session.role}</p>
                  </td>
                  <td className="px-5 py-3">
                    <code className="text-xs text-gray-600">{session.ip}</code>
                  </td>
                  <td className="px-5 py-3 text-xs text-gray-600">{formatDateTime(session.startedAt)}</td>
                  <td className="px-5 py-3 text-xs text-gray-600">{formatDateTime(session.lastActivity)}</td>
                  <td className="px-5 py-3 text-xs text-gray-500">{session.userAgent}</td>
                  <td className="px-5 py-3 text-right">
                    <button className="text-xs text-red-500 hover:text-red-700 hover:underline">
                      Terminate
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
