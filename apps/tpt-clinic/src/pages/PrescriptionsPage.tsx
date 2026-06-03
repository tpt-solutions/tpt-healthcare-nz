import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Prescription {
  id: string;
  patientId: string;
  patientName: string;
  patientNhiDisplay: string;
  medicationName: string;
  medicationCode?: string;
  dose: string;
  frequency: string;
  route: string;
  startDate: string;
  endDate?: string;
  prescriber: string;
  status: 'active' | 'stopped' | 'completed' | 'on-hold';
  repeatsRemaining?: number;
  totalRepeats?: number;
  dispensedCount: number;
}

interface PrescriptionFilter {
  status: string;
  search: string;
}

const STATUS_BADGE: Record<Prescription['status'], string> = {
  'active':    'badge-safe',
  'stopped':   'badge-urgent',
  'completed': 'inline-flex items-center rounded-full bg-secondary-100 px-2.5 py-0.5 text-xs font-medium text-secondary-600',
  'on-hold':   'badge-warning',
};

const ALL_STATUSES: Array<{ value: string; label: string }> = [
  { value: '',          label: 'All statuses' },
  { value: 'active',   label: 'Active' },
  { value: 'stopped',  label: 'Stopped' },
  { value: 'completed', label: 'Completed' },
  { value: 'on-hold',  label: 'On Hold' },
];

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function PrescriptionsPage() {
  const api = useApi();

  const [prescriptions, setPrescriptions] = useState<Prescription[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<PrescriptionFilter>({ status: 'active', search: '' });

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const params: Record<string, string> = {};
    if (filter.status) params['status'] = filter.status;
    if (filter.search) params['search'] = filter.search;

    void api.get<{ prescriptions: Prescription[]; total: number }>('/prescriptions', { params })
      .then((d) => {
        if (!cancelled) {
          setPrescriptions(d.prescriptions);
          setTotal(d.total);
        }
      })
      .catch(() => { if (!cancelled) setError('Failed to load prescriptions.'); })
      .finally(() => { if (!cancelled) setLoading(false); });

    return () => { cancelled = true; };
  }, [api, filter]);

  function updateFilter<K extends keyof PrescriptionFilter>(k: K, v: PrescriptionFilter[K]) {
    setFilter((prev) => ({ ...prev, [k]: v }));
  }

  return (
    <AppShell title="Prescriptions">
      {/* Filter bar */}
      <div className="mb-6 flex flex-wrap items-end gap-3">
        <div>
          <label htmlFor="rx-search" className="block text-sm font-medium text-secondary-700">
            Search medication or patient
          </label>
          <input
            id="rx-search"
            type="search"
            value={filter.search}
            onChange={(e) => updateFilter('search', e.target.value)}
            placeholder="Medication name or patient…"
            className="mt-1 rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 w-72"
          />
        </div>

        <div>
          <label htmlFor="rx-status" className="block text-sm font-medium text-secondary-700">
            Status
          </label>
          <select
            id="rx-status"
            value={filter.status}
            onChange={(e) => updateFilter('status', e.target.value)}
            className="mt-1 rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          >
            {ALL_STATUSES.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {/* Results count */}
      <p className="mb-3 text-sm text-secondary-500">
        {loading ? 'Loading…' : `${total.toLocaleString()} prescription${total !== 1 ? 's' : ''}`}
      </p>

      {/* Table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="h-7 w-7 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
          </div>
        ) : prescriptions.length === 0 ? (
          <p className="px-4 py-10 text-center text-sm text-secondary-500">
            No prescriptions found.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                <tr>
                  <th className="px-4 py-3 text-left">Patient</th>
                  <th className="px-4 py-3 text-left">Medication</th>
                  <th className="px-4 py-3 text-left">Dose / Frequency</th>
                  <th className="px-4 py-3 text-left">Route</th>
                  <th className="px-4 py-3 text-left">Start</th>
                  <th className="px-4 py-3 text-left">Repeats</th>
                  <th className="px-4 py-3 text-left">Prescriber</th>
                  <th className="px-4 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {prescriptions.map((rx) => (
                  <tr key={rx.id} className="hover:bg-secondary-50">
                    <td className="px-4 py-3">
                      <Link
                        to={`/patients/${rx.patientId}`}
                        className="font-medium text-secondary-900 hover:text-primary-600 hover:underline"
                      >
                        {rx.patientName}
                      </Link>
                      <div className="font-mono text-xs text-secondary-400">{rx.patientNhiDisplay}</div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="font-medium text-secondary-900">{rx.medicationName}</span>
                      {rx.medicationCode && (
                        <div className="font-mono text-xs text-secondary-400">{rx.medicationCode}</div>
                      )}
                    </td>
                    <td className="px-4 py-3 text-secondary-700">
                      {rx.dose} — {rx.frequency}
                    </td>
                    <td className="px-4 py-3 text-secondary-600">{rx.route}</td>
                    <td className="whitespace-nowrap px-4 py-3 text-secondary-600">
                      {new Date(rx.startDate).toLocaleDateString('en-NZ')}
                    </td>
                    <td className="px-4 py-3 text-secondary-600">
                      {rx.repeatsRemaining !== undefined && rx.totalRepeats !== undefined
                        ? `${rx.repeatsRemaining} / ${rx.totalRepeats}`
                        : '—'}
                    </td>
                    <td className="px-4 py-3 text-secondary-600">{rx.prescriber}</td>
                    <td className="px-4 py-3">
                      <span className={STATUS_BADGE[rx.status]}>{rx.status}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </AppShell>
  );
}
