import { useState } from 'react';
import { formatNZD, formatDate } from '../utils/format';

type ClaimStatus = 'pending' | 'approved' | 'declined' | 'paid';

interface AccClaim {
  id: string;
  claimRef: string;      // ACC45 or ACC6 form reference
  patientNhi: string;    // Masked NHI for display
  practitioner: string;
  serviceDate: string;
  lodgedDate: string;
  amount: number;
  status: ClaimStatus;
  declineReason?: string;
}

interface MonthlyRevenue {
  month: string;
  acc: number;
  pho: number;
  private: number;
  dhb: number;
}

const accClaims: AccClaim[] = [
  { id: '1', claimRef: 'ACC45-2026-00891', patientNhi: 'ZZZ0032', practitioner: 'Dr. Hemi Walker', serviceDate: '2026-06-01', lodgedDate: '2026-06-01', amount: 320.0, status: 'pending' },
  { id: '2', claimRef: 'ACC45-2026-00890', patientNhi: 'ZZZ1891', practitioner: 'Dr. Piripi Te Aho', serviceDate: '2026-06-01', lodgedDate: '2026-06-01', amount: 185.5, status: 'pending' },
  { id: '3', claimRef: 'ACC45-2026-00889', patientNhi: 'ZZZ2201', practitioner: 'Dr. Hemi Walker', serviceDate: '2026-05-31', lodgedDate: '2026-05-31', amount: 640.0, status: 'approved' },
  { id: '4', claimRef: 'ACC45-2026-00888', patientNhi: 'ZZZ3390', practitioner: 'Dr. Sione Tuilagi', serviceDate: '2026-05-31', lodgedDate: '2026-05-31', amount: 215.0, status: 'declined', declineReason: 'Duplicate claim — refer to ACC45-2026-00801' },
  { id: '5', claimRef: 'ACC6-2026-00887', patientNhi: 'ZZZ4412', practitioner: 'Dr. Hemi Walker', serviceDate: '2026-05-30', lodgedDate: '2026-05-30', amount: 1250.0, status: 'paid' },
  { id: '6', claimRef: 'ACC45-2026-00886', patientNhi: 'ZZZ5503', practitioner: 'Dr. Piripi Te Aho', serviceDate: '2026-05-28', lodgedDate: '2026-05-29', amount: 420.0, status: 'paid' },
];

const monthlyRevenue: MonthlyRevenue[] = [
  { month: 'Jan 2026', acc: 142300, pho: 287000, private: 38500, dhb: 52000 },
  { month: 'Feb 2026', acc: 128900, pho: 287000, private: 34200, dhb: 52000 },
  { month: 'Mar 2026', acc: 156700, pho: 287000, private: 41800, dhb: 52000 },
  { month: 'Apr 2026', acc: 163200, pho: 287000, private: 39600, dhb: 52000 },
  { month: 'May 2026', acc: 171450, pho: 291500, private: 43200, dhb: 52000 },
  { month: 'Jun 2026', acc: 187450, pho: 291500, private: 12000, dhb: 52000 },
];

const currentMonth = monthlyRevenue[monthlyRevenue.length - 1];
const currentMonthTotal = currentMonth.acc + currentMonth.pho + currentMonth.private + currentMonth.dhb;

const phoSummary = {
  enrolledPatients: 4823,
  capitationRate: 60.37,
  nextPaymentDate: '2026-07-01',
  nextPaymentAmount: 291500,
};

function claimStatusBadge(status: ClaimStatus) {
  const map: Record<ClaimStatus, string> = {
    pending: 'bg-amber-100 text-amber-700',
    approved: 'bg-blue-100 text-blue-700',
    declined: 'bg-red-100 text-red-700',
    paid: 'bg-green-100 text-green-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${map[status]}`}>
      {status}
    </span>
  );
}

export function BillingPage() {
  const [claimFilter, setClaimFilter] = useState<ClaimStatus | 'all'>('all');

  const filteredClaims = claimFilter === 'all'
    ? accClaims
    : accClaims.filter(c => c.status === claimFilter);

  const pendingTotal = accClaims.filter(c => c.status === 'pending').reduce((s, c) => s + c.amount, 0);
  const approvedTotal = accClaims.filter(c => c.status === 'approved').reduce((s, c) => s + c.amount, 0);
  const paidTotal = accClaims.filter(c => c.status === 'paid').reduce((s, c) => s + c.amount, 0);
  const declinedTotal = accClaims.filter(c => c.status === 'declined').reduce((s, c) => s + c.amount, 0);

  const maxRevenue = Math.max(...monthlyRevenue.map(m => m.acc + m.pho + m.private + m.dhb));

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Billing Dashboard</h1>
        <p className="mt-1 text-sm text-gray-500">ACC claims, PHO capitation, and revenue by funding type.</p>
      </div>

      {/* ACC claims summary */}
      <section className="mb-8">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">ACC Claims Status</h2>
        <div className="grid grid-cols-4 gap-3 mb-5">
          {[
            { label: 'Pending', value: pendingTotal, count: accClaims.filter(c => c.status === 'pending').length, color: 'amber' },
            { label: 'Approved', value: approvedTotal, count: accClaims.filter(c => c.status === 'approved').length, color: 'blue' },
            { label: 'Paid', value: paidTotal, count: accClaims.filter(c => c.status === 'paid').length, color: 'green' },
            { label: 'Declined', value: declinedTotal, count: accClaims.filter(c => c.status === 'declined').length, color: 'red' },
          ].map(({ label, value, count, color }) => (
            <button
              key={label}
              onClick={() => setClaimFilter(label.toLowerCase() as ClaimStatus)}
              className={`bg-white rounded-xl border border-gray-200 p-4 text-left hover:border-${color}-300 transition-colors ${
                claimFilter === label.toLowerCase() ? `border-${color}-400 ring-1 ring-${color}-300` : ''
              }`}
            >
              <p className="text-xs font-medium text-gray-500">{label}</p>
              <p className="mt-1 text-lg font-bold text-gray-900">{formatNZD(value)}</p>
              <p className="text-xs text-gray-400">{count} claim{count !== 1 ? 's' : ''}</p>
            </button>
          ))}
        </div>

        {/* Filter controls */}
        <div className="flex gap-2 mb-3">
          {(['all', 'pending', 'approved', 'declined', 'paid'] as const).map(f => (
            <button
              key={f}
              onClick={() => setClaimFilter(f)}
              className={`px-3 py-1 rounded-full text-xs font-medium transition-colors capitalize ${
                claimFilter === f
                  ? 'bg-brand-600 text-white'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
              }`}
            >
              {f === 'all' ? 'All' : f}
            </button>
          ))}
        </div>

        {/* Claims table */}
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 bg-gray-50">
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Claim Ref</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Patient NHI</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Practitioner</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Service Date</th>
                <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Amount</th>
                <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredClaims.map(claim => (
                <tr key={claim.id} className="hover:bg-gray-50">
                  <td className="px-5 py-3">
                    <code className="text-xs text-gray-700">{claim.claimRef}</code>
                  </td>
                  <td className="px-5 py-3">
                    <code className="text-xs bg-gray-100 rounded px-1.5 py-0.5">{claim.patientNhi}</code>
                  </td>
                  <td className="px-5 py-3 text-xs text-gray-600">{claim.practitioner}</td>
                  <td className="px-5 py-3 text-xs text-gray-600">{formatDate(claim.serviceDate)}</td>
                  <td className="px-5 py-3 text-right text-sm font-medium text-gray-900">{formatNZD(claim.amount)}</td>
                  <td className="px-5 py-3 text-right">
                    <div className="flex flex-col items-end gap-1">
                      {claimStatusBadge(claim.status)}
                      {claim.declineReason && (
                        <p className="text-xs text-red-500 max-w-[200px] text-right">{claim.declineReason}</p>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* PHO Capitation */}
      <section className="mb-8">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">PHO Capitation</h2>
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <div className="grid grid-cols-4 gap-4">
            <div>
              <p className="text-xs text-gray-500">Enrolled Patients</p>
              <p className="text-xl font-bold text-gray-900 mt-1">{phoSummary.enrolledPatients.toLocaleString('en-NZ')}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Capitation Rate</p>
              <p className="text-xl font-bold text-gray-900 mt-1">{formatNZD(phoSummary.capitationRate)} / patient</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Next Payment</p>
              <p className="text-xl font-bold text-gray-900 mt-1">{formatDate(phoSummary.nextPaymentDate)}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Expected Amount</p>
              <p className="text-xl font-bold text-green-700 mt-1">{formatNZD(phoSummary.nextPaymentAmount)}</p>
            </div>
          </div>
        </div>
      </section>

      {/* Revenue breakdown */}
      <section>
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">
          Monthly Revenue by Funding Type
        </h2>

        {/* Current month breakdown */}
        <div className="grid grid-cols-5 gap-3 mb-5">
          <div className="bg-white rounded-xl border border-gray-200 p-4 col-span-1">
            <p className="text-xs text-gray-500">June 2026 Total</p>
            <p className="text-xl font-bold text-gray-900 mt-1">{formatNZD(currentMonthTotal)}</p>
          </div>
          {[
            { label: 'ACC', value: currentMonth.acc, color: 'bg-blue-400' },
            { label: 'PHO', value: currentMonth.pho, color: 'bg-green-400' },
            { label: 'Private', value: currentMonth.private, color: 'bg-purple-400' },
            { label: 'DHB', value: currentMonth.dhb, color: 'bg-amber-400' },
          ].map(({ label, value, color }) => (
            <div key={label} className="bg-white rounded-xl border border-gray-200 p-4">
              <div className="flex items-center gap-2 mb-1">
                <span className={`inline-block h-2.5 w-2.5 rounded-full ${color}`} />
                <p className="text-xs text-gray-500">{label}</p>
              </div>
              <p className="text-lg font-bold text-gray-900">{formatNZD(value)}</p>
              <p className="text-xs text-gray-400">{((value / currentMonthTotal) * 100).toFixed(1)}%</p>
            </div>
          ))}
        </div>

        {/* Bar chart (CSS) */}
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <div className="space-y-3">
            {monthlyRevenue.map(m => {
              const total = m.acc + m.pho + m.private + m.dhb;
              const pct = (total / maxRevenue) * 100;
              return (
                <div key={m.month} className="flex items-center gap-3">
                  <p className="text-xs text-gray-500 w-20 flex-shrink-0">{m.month}</p>
                  <div className="flex-1 h-6 bg-gray-100 rounded-full overflow-hidden flex">
                    <div className="bg-green-400 h-full" style={{ width: `${(m.pho / total) * pct}%` }} title={`PHO: ${formatNZD(m.pho)}`} />
                    <div className="bg-blue-400 h-full" style={{ width: `${(m.acc / total) * pct}%` }} title={`ACC: ${formatNZD(m.acc)}`} />
                    <div className="bg-amber-400 h-full" style={{ width: `${(m.dhb / total) * pct}%` }} title={`DHB: ${formatNZD(m.dhb)}`} />
                    <div className="bg-purple-400 h-full" style={{ width: `${(m.private / total) * pct}%` }} title={`Private: ${formatNZD(m.private)}`} />
                  </div>
                  <p className="text-xs font-medium text-gray-700 w-28 text-right flex-shrink-0">{formatNZD(total)}</p>
                </div>
              );
            })}
          </div>
          <div className="flex gap-5 mt-4">
            {[
              { color: 'bg-green-400', label: 'PHO' },
              { color: 'bg-blue-400', label: 'ACC' },
              { color: 'bg-amber-400', label: 'DHB' },
              { color: 'bg-purple-400', label: 'Private' },
            ].map(({ color, label }) => (
              <div key={label} className="flex items-center gap-1.5">
                <span className={`h-2.5 w-2.5 rounded-full ${color}`} />
                <span className="text-xs text-gray-500">{label}</span>
              </div>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
