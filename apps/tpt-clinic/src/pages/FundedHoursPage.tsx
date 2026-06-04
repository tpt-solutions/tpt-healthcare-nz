import { useState } from 'react';
import AppShell from '@/components/AppShell';

type AllocStatus = 'active' | 'suspended' | 'expired' | 'closed';
type TSStatus = 'pending' | 'approved' | 'disputed' | 'voided';

interface Allocation {
  id: string;
  patientNhi: string;
  serviceType: string;
  hoursPerWeek: number;
  status: AllocStatus;
  providerName: string;
  startDate: string;
  fundingType: string;
}

interface Timesheet {
  id: string;
  patientNhi: string;
  allocationId: string;
  periodStart: string;
  periodEnd: string;
  totalHours: number;
  status: TSStatus;
}

const ALLOC_STATUS_STYLES: Record<AllocStatus, string> = {
  active: 'bg-green-100 text-green-700',
  suspended: 'bg-amber-100 text-amber-700',
  expired: 'bg-red-100 text-red-700',
  closed: 'bg-secondary-100 text-secondary-600',
};

const TS_STATUS_STYLES: Record<TSStatus, string> = {
  pending: 'bg-amber-100 text-amber-700',
  approved: 'bg-green-100 text-green-700',
  disputed: 'bg-red-100 text-red-700',
  voided: 'bg-secondary-100 text-secondary-600',
};

const STUB_ALLOCATIONS: Allocation[] = [
  {
    id: 'a1',
    patientNhi: 'ZHQ4021',
    serviceType: 'Personal care',
    hoursPerWeek: 14,
    status: 'active',
    providerName: 'CareFirst NZ',
    startDate: '2026-04-01',
    fundingType: 'moh-home-support',
  },
  {
    id: 'a2',
    patientNhi: 'ZHQ4021',
    serviceType: 'Domestic',
    hoursPerWeek: 4,
    status: 'active',
    providerName: 'CareFirst NZ',
    startDate: '2026-04-01',
    fundingType: 'moh-home-support',
  },
  {
    id: 'a3',
    patientNhi: 'ZAB1234',
    serviceType: 'Personal care',
    hoursPerWeek: 21,
    status: 'active',
    providerName: 'Bupa Home Support',
    startDate: '2026-06-01',
    fundingType: 'nasc-allocated',
  },
];

const STUB_TIMESHEETS: Timesheet[] = [
  {
    id: 't1',
    patientNhi: 'ZHQ4021',
    allocationId: 'a1',
    periodStart: '2026-05-26',
    periodEnd: '2026-06-01',
    totalHours: 14,
    status: 'approved',
  },
  {
    id: 't2',
    patientNhi: 'ZHQ4021',
    allocationId: 'a1',
    periodStart: '2026-06-02',
    periodEnd: '2026-06-08',
    totalHours: 12.5,
    status: 'pending',
  },
];

type Tab = 'allocations' | 'timesheets';

export default function FundedHoursPage() {
  const [tab, setTab] = useState<Tab>('allocations');

  // Summary totals from stub data
  const totalAllocated = STUB_ALLOCATIONS.filter((a) => a.status === 'active')
    .reduce((s, a) => s + a.hoursPerWeek, 0);

  return (
    <AppShell title="Funded Hours">
      <div className="space-y-4">
        {/* Summary strip */}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { label: 'Active Allocations', value: STUB_ALLOCATIONS.filter((a) => a.status === 'active').length },
            { label: 'Total hrs/week', value: totalAllocated },
            { label: 'Pending Timesheets', value: STUB_TIMESHEETS.filter((t) => t.status === 'pending').length },
            { label: 'Approved (this month)', value: STUB_TIMESHEETS.filter((t) => t.status === 'approved').length },
          ].map(({ label, value }) => (
            <div key={label} className="rounded-xl border border-secondary-200 bg-white p-4 text-center">
              <p className="text-2xl font-bold text-secondary-900">{value}</p>
              <p className="mt-0.5 text-xs text-secondary-500">{label}</p>
            </div>
          ))}
        </div>

        {/* Tabs */}
        <div className="flex gap-1 rounded-lg bg-secondary-100 p-1 w-fit">
          {(['allocations', 'timesheets'] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={[
                'rounded-md px-4 py-1.5 text-sm font-medium capitalize transition-colors',
                tab === t
                  ? 'bg-white text-secondary-900 shadow-sm'
                  : 'text-secondary-500 hover:text-secondary-700',
              ].join(' ')}
            >
              {t}
            </button>
          ))}
        </div>

        {tab === 'allocations' && (
          <>
            <div className="flex justify-end">
              <button className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                New Allocation
              </button>
            </div>
            <div className="overflow-x-auto rounded-xl border border-secondary-200 bg-white">
              <table className="min-w-full divide-y divide-secondary-200 text-sm">
                <thead className="bg-secondary-50">
                  <tr>
                    {['Patient NHI', 'Service', 'hrs/wk', 'Provider', 'Status', 'Start', 'Funding', ''].map((h) => (
                      <th key={h} className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-secondary-500">
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {STUB_ALLOCATIONS.map((a) => (
                    <tr key={a.id} className="hover:bg-secondary-50">
                      <td className="px-4 py-3 font-mono text-secondary-900">{a.patientNhi}</td>
                      <td className="px-4 py-3 text-secondary-700">{a.serviceType}</td>
                      <td className="px-4 py-3 font-medium text-secondary-900">{a.hoursPerWeek}</td>
                      <td className="px-4 py-3 text-secondary-600">{a.providerName}</td>
                      <td className="px-4 py-3">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${ALLOC_STATUS_STYLES[a.status]}`}>
                          {a.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-secondary-500">{a.startDate}</td>
                      <td className="px-4 py-3 text-xs text-secondary-400">{a.fundingType}</td>
                      <td className="px-4 py-3">
                        <button className="text-xs text-primary-600 hover:underline">Edit</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}

        {tab === 'timesheets' && (
          <>
            <div className="flex justify-end">
              <button className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                Record Timesheet
              </button>
            </div>
            <div className="space-y-3">
              {STUB_TIMESHEETS.map((ts) => (
                <div key={ts.id} className="rounded-xl border border-secondary-200 bg-white p-4">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="flex items-center gap-2">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${TS_STATUS_STYLES[ts.status]}`}>
                          {ts.status}
                        </span>
                        <span className="font-mono text-sm text-secondary-900">{ts.patientNhi}</span>
                      </div>
                      <p className="mt-1 text-sm text-secondary-600">
                        Period: {ts.periodStart} → {ts.periodEnd}
                      </p>
                      <p className="text-sm font-medium text-secondary-900">{ts.totalHours} hours delivered</p>
                    </div>
                    <div className="flex gap-2 shrink-0">
                      {ts.status === 'pending' && (
                        <button className="rounded-md border border-green-300 bg-green-50 px-3 py-1 text-xs font-medium text-green-700 hover:bg-green-100">
                          Approve
                        </button>
                      )}
                      <button className="rounded-md border border-secondary-300 px-3 py-1 text-xs font-medium text-secondary-600 hover:bg-secondary-50">
                        View
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    </AppShell>
  );
}
