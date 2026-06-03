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
