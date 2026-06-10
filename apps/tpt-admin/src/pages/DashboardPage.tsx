import { useEffect, useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { formatNZD } from '../utils/format';

// Stub data — will be fetched via @tpt/api-client once backend is connected
const stats = {
  practitionerCount: 14,
  patientEnrolmentCount: 4823,
  accClaimsThisMonth: 187450.0,
  capitationCycleDate: '2026-07-01',
  overdueAuditItems: 2,
};

const recentAccClaims = [
  { id: 'ACC-2026-00891', patient: 'M. Tūhoe', amount: 320.0, status: 'approved', date: '2026-06-01' },
  { id: 'ACC-2026-00890', patient: 'J. Ngāpuhi', amount: 185.5, status: 'pending', date: '2026-06-01' },
  { id: 'ACC-2026-00889', patient: 'K. Whanganui', amount: 640.0, status: 'approved', date: '2026-05-31' },
  { id: 'ACC-2026-00888', patient: 'A. Ngāti Porou', amount: 215.0, status: 'declined', date: '2026-05-31' },
];

const practitioners = [
  { name: 'Dr. Hemi Walker', type: 'GP', apcStatus: 'current', patients: 842 },
  { name: 'Dr. Piripi Te Aho', type: 'GP', apcStatus: 'current', patients: 756 },
  { name: 'Nurse Mere Parata', type: 'Nurse Practitioner', apcStatus: 'current', patients: 0 },
  { name: 'Dr. Sione Tuilagi', type: 'Specialist', apcStatus: 'expiring_soon', patients: 312 },
];

function statusBadge(status: string) {
  const map: Record<string, string> = {
    approved: 'bg-green-100 text-green-700',
    pending: 'bg-amber-100 text-amber-700',
    declined: 'bg-red-100 text-red-700',
    current: 'bg-green-100 text-green-700',
    expiring_soon: 'bg-amber-100 text-amber-700',
    expired: 'bg-red-100 text-red-700',
  };
  const labels: Record<string, string> = {
    approved: 'Approved',
    pending: 'Pending',
    declined: 'Declined',
    current: 'APC Current',
    expiring_soon: 'Expiring Soon',
    expired: 'Expired',
  };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${map[status] ?? 'bg-gray-100 text-gray-600'}`}>
      {labels[status] ?? status}
    </span>
  );
}

interface BackupRun {
  id: string;
  label: string;
  started_at: string;
  completed_at?: string;
  status: 'running' | 'success' | 'failed' | 'verified';
  size_bytes: number;
  error_text?: string;
}

function BackupStatusWidget() {
  const [runs, setRuns] = useState<BackupRun[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/v1/practice/system/backup')
      .then(r => r.json())
      .then(data => setRuns(Array.isArray(data) ? data : []))
      .catch(() => setRuns([]))
      .finally(() => setLoading(false));
  }, []);

  const latest = runs[0];
  const statusColor: Record<string, string> = {
    success:  'bg-green-100 text-green-700',
    verified: 'bg-green-100 text-green-700',
    running:  'bg-blue-100 text-blue-700',
    failed:   'bg-red-100 text-red-700',
  };

  function formatBytes(b: number) {
    if (b === 0) return '—';
    if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
    if (b < 1024 * 1024 * 1024) return `${(b / (1024 * 1024)).toFixed(1)} MB`;
    return `${(b / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  }

  if (loading) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5 animate-pulse">
        <div className="h-3 w-24 bg-gray-200 rounded mb-3" />
        <div className="h-5 w-16 bg-gray-100 rounded" />
      </div>
    );
  }

  if (!latest) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
        <p className="text-xs font-medium text-gray-500 uppercase tracking-wide mb-2">Database Backup</p>
        <p className="text-sm text-gray-400">No backups recorded yet.</p>
      </div>
    );
  }

  return (
    <div className={`rounded-xl border shadow-sm p-5 ${latest.status === 'failed' ? 'bg-red-50 border-red-200' : 'bg-white border-gray-200'}`}>
      <div className="flex items-center justify-between mb-3">
        <p className="text-xs font-medium text-gray-500 uppercase tracking-wide">Database Backup</p>
        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${statusColor[latest.status] ?? 'bg-gray-100 text-gray-600'}`}>
          {latest.status}
        </span>
      </div>
      <p className="text-sm font-semibold text-gray-900 truncate">{latest.label}</p>
      <p className="text-xs text-gray-500 mt-0.5">
        {latest.completed_at
          ? new Date(latest.completed_at).toLocaleString('en-NZ')
          : `Started ${new Date(latest.started_at).toLocaleString('en-NZ')}`}
        {latest.size_bytes > 0 && ` — ${formatBytes(latest.size_bytes)}`}
      </p>
      {latest.status === 'failed' && latest.error_text && (
        <p className="mt-2 text-xs text-red-600 truncate">{latest.error_text}</p>
      )}
      {runs.length > 1 && (
        <p className="mt-2 text-xs text-gray-400">{runs.length - 1} earlier run{runs.length > 2 ? 's' : ''} available</p>
      )}
    </div>
  );
}

export function DashboardPage() {
  const { user } = useAuth();

  const daysToCapitation = Math.ceil(
    (new Date(stats.capitationCycleDate).getTime() - Date.now()) / (1000 * 60 * 60 * 24),
  );

  return (
    <div className="p-6 max-w-6xl mx-auto">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">
          {user?.practiceName}
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Admin overview &mdash; {new Date().toLocaleDateString('en-NZ', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}
        </p>
      </div>

      {/* KPI cards */}
      <div className="grid grid-cols-4 gap-4 mb-8">
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <p className="text-xs font-medium text-gray-500 uppercase tracking-wide">Practitioners</p>
          <p className="mt-2 text-3xl font-bold text-gray-900">{stats.practitionerCount}</p>
          <p className="text-xs text-gray-400 mt-1">registered this practice</p>
        </div>

        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <p className="text-xs font-medium text-gray-500 uppercase tracking-wide">Enrolled Patients</p>
          <p className="mt-2 text-3xl font-bold text-gray-900">{stats.patientEnrolmentCount.toLocaleString('en-NZ')}</p>
          <p className="text-xs text-gray-400 mt-1">NES enrolled</p>
        </div>

        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <p className="text-xs font-medium text-gray-500 uppercase tracking-wide">ACC Claims (June)</p>
          <p className="mt-2 text-2xl font-bold text-gray-900">{formatNZD(stats.accClaimsThisMonth)}</p>
          <p className="text-xs text-gray-400 mt-1">month to date</p>
        </div>

        <div className={`rounded-xl border shadow-sm p-5 ${
          stats.overdueAuditItems > 0 ? 'bg-red-50 border-red-200' : 'bg-white border-gray-200'
        }`}>
          <p className={`text-xs font-medium uppercase tracking-wide ${
            stats.overdueAuditItems > 0 ? 'text-red-600' : 'text-gray-500'
          }`}>Overdue Audit Items</p>
          <p className={`mt-2 text-3xl font-bold ${stats.overdueAuditItems > 0 ? 'text-red-700' : 'text-gray-900'}`}>
            {stats.overdueAuditItems}
          </p>
          {stats.overdueAuditItems > 0 ? (
            <a href="/audit" className="text-xs text-red-600 underline mt-1 block">Review now</a>
          ) : (
            <p className="text-xs text-gray-400 mt-1">all clear</p>
          )}
        </div>
      </div>

      {/* Capitation notice */}
      <div className="bg-brand-50 border border-brand-200 rounded-xl px-5 py-4 mb-8 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <svg className="h-5 w-5 text-brand-600 flex-shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />
          </svg>
          <div>
            <p className="text-sm font-semibold text-brand-900">
              Next PHO Capitation Cycle: {new Date(stats.capitationCycleDate).toLocaleDateString('en-NZ', { day: 'numeric', month: 'long', year: 'numeric' })}
            </p>
            <p className="text-xs text-brand-700">
              {daysToCapitation} days away &mdash; ensure all patient enrolments and updates are submitted before the cutoff.
            </p>
          </div>
        </div>
        <a href="/reports" className="text-xs font-medium text-brand-700 hover:underline flex-shrink-0 ml-4">
          View capitation report
        </a>
      </div>

      {/* Backup status + low-stock alert row */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="col-span-1">
          <BackupStatusWidget />
        </div>
        {/* Two quick-stat slots reserved for future M12 widgets */}
        <div className="col-span-2 bg-white rounded-xl border border-gray-200 shadow-sm p-5 flex items-center gap-4">
          <svg className="h-8 w-8 text-gray-300 flex-shrink-0" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 5.625c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" />
          </svg>
          <div>
            <p className="text-sm font-medium text-gray-700">Storage &amp; Integrations</p>
            <p className="text-xs text-gray-400 mt-0.5">Connect accounting, payroll, and cloud storage providers in <a href="/integrations" className="text-brand-600 hover:underline">Integrations</a>.</p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* Recent ACC claims */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm">
          <div className="px-5 py-4 border-b border-gray-100 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-gray-900">Recent ACC Claims</h2>
            <a href="/billing" className="text-xs text-brand-600 hover:underline">View all</a>
          </div>
          <table className="w-full text-sm">
            <tbody className="divide-y divide-gray-100">
              {recentAccClaims.map(claim => (
                <tr key={claim.id}>
                  <td className="px-5 py-3">
                    <p className="text-xs font-mono text-gray-600">{claim.id}</p>
                    <p className="text-xs text-gray-400">{claim.patient}</p>
                  </td>
                  <td className="px-5 py-3 text-right">
                    <p className="text-sm font-medium text-gray-900">{formatNZD(claim.amount)}</p>
                  </td>
                  <td className="px-5 py-3 text-right">{statusBadge(claim.status)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>

        {/* Practitioner APC status */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm">
          <div className="px-5 py-4 border-b border-gray-100 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-gray-900">Practitioner APC Status</h2>
            <a href="/practitioners" className="text-xs text-brand-600 hover:underline">Manage all</a>
          </div>
          <table className="w-full text-sm">
            <tbody className="divide-y divide-gray-100">
              {practitioners.map(p => (
                <tr key={p.name}>
                  <td className="px-5 py-3">
                    <p className="text-sm font-medium text-gray-900">{p.name}</p>
                    <p className="text-xs text-gray-400">{p.type}{p.patients > 0 ? ` — ${p.patients} patients` : ''}</p>
                  </td>
                  <td className="px-5 py-3 text-right">{statusBadge(p.apcStatus)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      </div>
    </div>
  );
}
