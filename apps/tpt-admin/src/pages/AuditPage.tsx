import { useState } from 'react';
import { formatDateTime } from '../utils/format';

/**
 * Audit log viewer.
 * Reads from GET /api/v1/audit-events with date range, action, resource type,
 * and practitioner filters.
 *
 * Audit records are append-only and retained for 10 years (HIPC + NZ Privacy Act).
 * No UPDATE or DELETE is permitted on the audit_events table (core/audit/trail.go).
 */

type AuditAction = 'read' | 'create' | 'update' | 'delete' | 'login' | 'consent';

interface AuditEvent {
  id: string;
  timestamp: string;
  actor: string;
  actorRole: string;
  action: AuditAction;
  resourceType: string;
  resourceId: string;
  patientNhi: string | null;   // masked for display
  sourceIp: string;
  correlationId: string;
  outcome: 'success' | 'failure';
}

// Stub data — real events fetched from GET /api/v1/audit-events
const auditEvents: AuditEvent[] = [
  { id: 'ae-001', timestamp: '2026-06-03T09:45:12Z', actor: 'Dr. Hemi Walker', actorRole: 'GP', action: 'read', resourceType: 'Patient', resourceId: 'patient-0032', patientNhi: 'ZZZ0032', sourceIp: '10.1.2.45', correlationId: 'req-abc123', outcome: 'success' },
  { id: 'ae-002', timestamp: '2026-06-03T09:45:18Z', actor: 'Dr. Hemi Walker', actorRole: 'GP', action: 'read', resourceType: 'Observation', resourceId: 'obs-00441', patientNhi: 'ZZZ0032', sourceIp: '10.1.2.45', correlationId: 'req-abc123', outcome: 'success' },
  { id: 'ae-003', timestamp: '2026-06-03T09:47:02Z', actor: 'Dr. Hemi Walker', actorRole: 'GP', action: 'create', resourceType: 'MedicationRequest', resourceId: 'rx-00998', patientNhi: 'ZZZ0032', sourceIp: '10.1.2.45', correlationId: 'req-def456', outcome: 'success' },
  { id: 'ae-004', timestamp: '2026-06-03T10:12:33Z', actor: 'Dr. Piripi Te Aho', actorRole: 'GP', action: 'read', resourceType: 'Patient', resourceId: 'patient-1891', patientNhi: 'ZZZ1891', sourceIp: '10.1.2.46', correlationId: 'req-ghi789', outcome: 'success' },
  { id: 'ae-005', timestamp: '2026-06-03T10:15:44Z', actor: 'Nurse Mere Parata', actorRole: 'Nurse Practitioner', action: 'create', resourceType: 'Immunization', resourceId: 'imm-00221', patientNhi: 'ZZZ3390', sourceIp: '10.1.2.47', correlationId: 'req-jkl012', outcome: 'success' },
  { id: 'ae-006', timestamp: '2026-06-03T11:02:07Z', actor: 'Dr. Sione Tuilagi', actorRole: 'Specialist', action: 'read', resourceType: 'Condition', resourceId: 'cond-00114', patientNhi: 'ZZZ4412', sourceIp: '10.1.3.88', correlationId: 'req-mno345', outcome: 'failure' },
  { id: 'ae-007', timestamp: '2026-06-03T11:30:15Z', actor: 'Tama Parata', actorRole: 'Admin', action: 'login', resourceType: 'Session', resourceId: 'sess-admin-001', patientNhi: null, sourceIp: '10.1.4.12', correlationId: 'req-pqr678', outcome: 'success' },
  { id: 'ae-008', timestamp: '2026-06-03T13:45:22Z', actor: 'Aroha Ngata', actorRole: 'Patient', action: 'consent', resourceType: 'Consent', resourceId: 'consent-pharmacy', patientNhi: 'ZZZ0032', sourceIp: '203.88.121.44', correlationId: 'req-stu901', outcome: 'success' },
  { id: 'ae-009', timestamp: '2026-06-03T14:10:08Z', actor: 'Dr. Hemi Walker', actorRole: 'GP', action: 'update', resourceType: 'Encounter', resourceId: 'enc-00882', patientNhi: 'ZZZ2201', sourceIp: '10.1.2.45', correlationId: 'req-vwx234', outcome: 'success' },
  { id: 'ae-010', timestamp: '2026-06-03T14:55:31Z', actor: 'API System', actorRole: 'Service', action: 'create', resourceType: 'AuditEvent', resourceId: 'ae-009', patientNhi: null, sourceIp: '127.0.0.1', correlationId: 'req-vwx234', outcome: 'success' },
];

const resourceTypes = ['All', 'Patient', 'Observation', 'Condition', 'MedicationRequest', 'Immunization', 'Encounter', 'Consent', 'AuditEvent', 'Session'];
const actions: (AuditAction | 'all')[] = ['all', 'read', 'create', 'update', 'delete', 'login', 'consent'];
const practitioners = ['All', 'Dr. Hemi Walker', 'Dr. Piripi Te Aho', 'Nurse Mere Parata', 'Dr. Sione Tuilagi', 'Tama Parata', 'Aroha Ngata', 'API System'];

function actionBadge(action: AuditAction) {
  const map: Record<AuditAction, string> = {
    read: 'bg-blue-100 text-blue-700',
    create: 'bg-green-100 text-green-700',
    update: 'bg-amber-100 text-amber-700',
    delete: 'bg-red-100 text-red-700',
    login: 'bg-gray-100 text-gray-600',
    consent: 'bg-purple-100 text-purple-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${map[action]}`}>
      {action}
    </span>
  );
}

function outcomeDot(outcome: AuditEvent['outcome']) {
  return (
    <span className={`inline-block h-2 w-2 rounded-full ${outcome === 'success' ? 'bg-green-400' : 'bg-red-500'}`} title={outcome} />
  );
}

const PAGE_SIZE = 6;

export function AuditPage() {
  const [dateFrom, setDateFrom] = useState('2026-06-03');
  const [dateTo, setDateTo] = useState('2026-06-03');
  const [actionFilter, setActionFilter] = useState<AuditAction | 'all'>('all');
  const [resourceFilter, setResourceFilter] = useState('All');
  const [practitionerFilter, setPractitionerFilter] = useState('All');
  const [page, setPage] = useState(1);

  const filtered = auditEvents.filter(e => {
    if (actionFilter !== 'all' && e.action !== actionFilter) return false;
    if (resourceFilter !== 'All' && e.resourceType !== resourceFilter) return false;
    if (practitionerFilter !== 'All' && e.actor !== practitionerFilter) return false;
    return true;
  });

  const totalPages = Math.ceil(filtered.length / PAGE_SIZE);
  const paginated = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Audit Log</h1>
        <p className="mt-1 text-sm text-gray-500">
          Immutable record of all reads and writes to health data. Retained for 10 years.
        </p>
      </div>

      {/* Retention note */}
      <div className="bg-amber-50 border border-amber-200 rounded-xl px-4 py-3 mb-6">
        <p className="text-xs text-amber-800">
          <span className="font-semibold">Audit records are immutable.</span> No UPDATE or DELETE operations are
          permitted on the audit_events table. Records must be retained for a minimum of 10 years (Privacy Act 2020 /
          HIPC). Each record includes: timestamp (UTC), actor, patient NHI (encrypted at rest), resource type,
          resource ID, action, source IP, and correlation ID.
        </p>
      </div>

      {/* Filters */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-4 mb-5">
        <div className="grid grid-cols-5 gap-3">
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">From</label>
            <input
              type="date"
              value={dateFrom}
              onChange={e => { setDateFrom(e.target.value); setPage(1); }}
              className="w-full rounded-lg border border-gray-300 px-2.5 py-1.5 text-xs focus:border-brand-500 focus:outline-none"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">To</label>
            <input
              type="date"
              value={dateTo}
              onChange={e => { setDateTo(e.target.value); setPage(1); }}
              className="w-full rounded-lg border border-gray-300 px-2.5 py-1.5 text-xs focus:border-brand-500 focus:outline-none"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Action</label>
            <select
              value={actionFilter}
              onChange={e => { setActionFilter(e.target.value as AuditAction | 'all'); setPage(1); }}
              className="w-full rounded-lg border border-gray-300 px-2.5 py-1.5 text-xs focus:border-brand-500 focus:outline-none capitalize"
            >
              {actions.map(a => <option key={a} value={a} className="capitalize">{a === 'all' ? 'All actions' : a}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Resource Type</label>
            <select
              value={resourceFilter}
              onChange={e => { setResourceFilter(e.target.value); setPage(1); }}
              className="w-full rounded-lg border border-gray-300 px-2.5 py-1.5 text-xs focus:border-brand-500 focus:outline-none"
            >
              {resourceTypes.map(r => <option key={r}>{r}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-600 mb-1">Practitioner / Actor</label>
            <select
              value={practitionerFilter}
              onChange={e => { setPractitionerFilter(e.target.value); setPage(1); }}
              className="w-full rounded-lg border border-gray-300 px-2.5 py-1.5 text-xs focus:border-brand-500 focus:outline-none"
            >
              {practitioners.map(p => <option key={p}>{p}</option>)}
            </select>
          </div>
        </div>
      </div>

      {/* Results */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
        <div className="px-5 py-3 border-b border-gray-100 bg-gray-50 flex items-center justify-between">
          <p className="text-xs text-gray-500">{filtered.length} events matching filters</p>
          <button className="text-xs text-brand-600 hover:underline">Export CSV</button>
        </div>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-100">
              <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Timestamp (UTC)</th>
              <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Actor</th>
              <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Action</th>
              <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Resource</th>
              <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Patient NHI</th>
              <th className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Source IP</th>
              <th className="px-4 py-3 text-center text-xs font-semibold text-gray-500 uppercase tracking-wide">Result</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {paginated.map(evt => (
              <tr key={evt.id} className={`hover:bg-gray-50 ${evt.outcome === 'failure' ? 'bg-red-50' : ''}`}>
                <td className="px-4 py-3 text-xs font-mono text-gray-600 whitespace-nowrap">
                  {formatDateTime(evt.timestamp)}
                </td>
                <td className="px-4 py-3">
                  <p className="text-xs font-medium text-gray-900">{evt.actor}</p>
                  <p className="text-xs text-gray-400">{evt.actorRole}</p>
                </td>
                <td className="px-4 py-3">{actionBadge(evt.action)}</td>
                <td className="px-4 py-3">
                  <p className="text-xs text-gray-700">{evt.resourceType}</p>
                  <code className="text-xs text-gray-400">{evt.resourceId}</code>
                </td>
                <td className="px-4 py-3">
                  {evt.patientNhi ? (
                    <code className="text-xs bg-gray-100 rounded px-1.5 py-0.5">{evt.patientNhi}</code>
                  ) : (
                    <span className="text-xs text-gray-300">—</span>
                  )}
                </td>
                <td className="px-4 py-3">
                  <code className="text-xs text-gray-500">{evt.sourceIp}</code>
                </td>
                <td className="px-4 py-3 text-center">{outcomeDot(evt.outcome)}</td>
              </tr>
            ))}
          </tbody>
        </table>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="px-5 py-3 border-t border-gray-100 flex items-center justify-between">
            <p className="text-xs text-gray-500">
              Page {page} of {totalPages}
            </p>
            <div className="flex gap-2">
              <button
                disabled={page === 1}
                onClick={() => setPage(p => p - 1)}
                className="px-3 py-1.5 rounded-lg border border-gray-300 text-xs text-gray-700 disabled:opacity-40 hover:bg-gray-50 transition-colors"
              >
                Previous
              </button>
              <button
                disabled={page === totalPages}
                onClick={() => setPage(p => p + 1)}
                className="px-3 py-1.5 rounded-lg border border-gray-300 text-xs text-gray-700 disabled:opacity-40 hover:bg-gray-50 transition-colors"
              >
                Next
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
