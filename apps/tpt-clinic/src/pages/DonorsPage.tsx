import React, { FormEvent, useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Donor {
  id: string;
  nhi: string;
  bloodGroup: string;
  rhd: string;
  status: string;
  deferralReason: string | null;
  deferralEndDate: string | null;
  totalDonations: number;
  lastDonationAt: string | null;
  haemoglobinGdl: number | null;
  createdAt: string;
  updatedAt: string;
}

interface DonorListResponse {
  donors: Donor[];
  total: number;
}

// ---------------------------------------------------------------------------
// Donor status badge
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    active: 'bg-green-100 text-green-700',
    deferred: 'bg-yellow-100 text-yellow-700',
    permanent: 'bg-red-100 text-red-700',
    inactive: 'bg-secondary-100 text-secondary-600',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colors[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function DonorsPage() {
  const api = useApi();

  const [donors, setDonors] = useState<Donor[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [statusFilter, setStatusFilter] = useState('active');
  const [bloodGroupFilter, setBloodGroupFilter] = useState('');

  // Create donor form
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newNHI, setNewNHI] = useState('');
  const [newBloodGroup, setNewBloodGroup] = useState('A+');

  // Defer modal
  const [deferDonorId, setDeferDonorId] = useState<string | null>(null);
  const [deferReason, setDeferReason] = useState('low-haemoglobin');
  const [deferDetails, setDeferDetails] = useState('');

  useEffect(() => {
    loadDonors();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter, bloodGroupFilter]);

  async function loadDonors() {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = {};
      if (statusFilter) params['status'] = statusFilter;
      if (bloodGroupFilter) params['bloodGroup'] = bloodGroupFilter;

      const data = await api.get<DonorListResponse>('/donors', { params });
      setDonors(data.donors);
      setTotal(data.total);
    } catch {
      setError('Failed to load donors');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreateDonor(e: FormEvent) {
    e.preventDefault();
    try {
      await api.post('/donors', { nhi: newNHI, bloodGroup: newBloodGroup });
      setShowCreateForm(false);
      setNewNHI('');
      setNewBloodGroup('A+');
      await loadDonors();
    } catch {
      setError('Failed to create donor');
    }
  }

  async function handleDeferDonor(donorId: string) {
    try {
      await api.post(`/donors/${donorId}/defer`, { reason: deferReason, details: deferDetails });
      setDeferDonorId(null);
      setDeferReason('low-haemoglobin');
      setDeferDetails('');
      await loadDonors();
    } catch {
      setError('Failed to defer donor');
    }
  }

  async function handleReinstate(donorId: string) {
    try {
      await api.post(`/donors/${donorId}/reinstate`, {});
      await loadDonors();
    } catch {
      setError('Failed to reinstate donor');
    }
  }

  return (
    <AppShell title="Blood Donors">
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {/* Toolbar */}
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="flex gap-2">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          >
            <option value="">All Statuses</option>
            <option value="active">Active</option>
            <option value="deferred">Deferred</option>
            <option value="permanent">Permanent</option>
            <option value="inactive">Inactive</option>
          </select>
          <select
            value={bloodGroupFilter}
            onChange={(e) => setBloodGroupFilter(e.target.value)}
            className="rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          >
            <option value="">All Blood Groups</option>
            <option value="A+">A+</option>
            <option value="A-">A-</option>
            <option value="B+">B+</option>
            <option value="B-">B-</option>
            <option value="AB+">AB+</option>
            <option value="AB-">AB-</option>
            <option value="O+">O+</option>
            <option value="O-">O-</option>
          </select>
        </div>
        <button
          onClick={() => setShowCreateForm(true)}
          className="flex items-center gap-1.5 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 sm:ml-auto"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
          </svg>
          Register Donor
        </button>
      </div>

      {/* Create donor form */}
      {showCreateForm && (
        <form onSubmit={handleCreateDonor} className="mb-6 rounded-xl border border-primary-200 bg-primary-50 p-4">
          <h3 className="mb-3 text-sm font-semibold text-primary-800">Register New Donor</h3>
          <div className="flex flex-col gap-3 sm:flex-row">
            <div className="flex-1">
              <label className="block text-xs font-medium text-primary-700">NHI</label>
              <input
                type="text"
                value={newNHI}
                onChange={(e) => setNewNHI(e.target.value.toUpperCase())}
                placeholder="ABC1234"
                maxLength={7}
                required
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 font-mono text-sm uppercase shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
              />
            </div>
            <div className="w-32">
              <label className="block text-xs font-medium text-primary-700">Blood Group</label>
              <select
                value={newBloodGroup}
                onChange={(e) => setNewBloodGroup(e.target.value)}
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
              >
                <option value="A+">A+</option>
                <option value="A-">A-</option>
                <option value="B+">B+</option>
                <option value="B-">B-</option>
                <option value="AB+">AB+</option>
                <option value="AB-">AB-</option>
                <option value="O+">O+</option>
                <option value="O-">O-</option>
              </select>
            </div>
            <div className="flex items-end gap-2">
              <button
                type="submit"
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700"
              >
                Save
              </button>
              <button
                type="button"
                onClick={() => setShowCreateForm(false)}
                className="rounded-md bg-white px-4 py-2 text-sm font-medium text-secondary-700 ring-1 ring-secondary-300 hover:bg-secondary-50"
              >
                Cancel
              </button>
            </div>
          </div>
        </form>
      )}

      {/* Deferral modal */}
      {deferDonorId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-lg">
            <h3 className="mb-4 text-sm font-semibold text-secondary-900">Defer Donor</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-secondary-700">Reason</label>
                <select
                  value={deferReason}
                  onChange={(e) => setDeferReason(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm"
                >
                  <option value="low-haemoglobin">Low Haemoglobin</option>
                  <option value="recent-travel">Recent Travel</option>
                  <option value="medical-condition">Medical Condition</option>
                  <option value="medication">Medication</option>
                  <option value="tattoo-piercing">Tattoo / Piercing</option>
                  <option value="underweight">Underweight</option>
                  <option value="behavioural-risk">Behavioural Risk</option>
                  <option value="permanent">Permanent</option>
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-secondary-700">Details (optional)</label>
                <textarea
                  value={deferDetails}
                  onChange={(e) => setDeferDetails(e.target.value)}
                  rows={2}
                  className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm"
                />
              </div>
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setDeferDonorId(null)}
                  className="rounded-md bg-white px-4 py-2 text-sm font-medium text-secondary-700 ring-1 ring-secondary-300"
                >
                  Cancel
                </button>
                <button
                  onClick={() => handleDeferDonor(deferDonorId)}
                  className="rounded-md bg-yellow-600 px-4 py-2 text-sm font-medium text-white hover:bg-yellow-700"
                >
                  Defer
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Donor table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        <div className="border-b border-secondary-100 px-4 py-3">
          <p className="text-sm text-secondary-500">
            {loading ? 'Loading…' : `${total.toLocaleString()} donor${total !== 1 ? 's' : ''}`}
          </p>
        </div>

        {!loading && donors.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-secondary-500">
            No donors found{statusFilter ? ` with status "${statusFilter}"` : ''}.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                <tr>
                  <th className="px-4 py-3 text-left">NHI</th>
                  <th className="px-4 py-3 text-left">Blood Group</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-left">Deferral</th>
                  <th className="px-4 py-3 text-left">Donations</th>
                  <th className="px-4 py-3 text-left">Last Donation</th>
                  <th className="px-4 py-3 text-left">Hb (g/dL)</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {donors.map((d) => (
                  <tr key={d.id} className="hover:bg-secondary-50">
                    <td className="px-4 py-3 font-mono text-secondary-900">{d.nhi}</td>
                    <td className="px-4 py-3 font-mono font-medium text-secondary-900">
                      {d.bloodGroup}
                    </td>
                    <td className="px-4 py-3"><StatusBadge status={d.status} /></td>
                    <td className="px-4 py-3 text-secondary-600">
                      {d.deferralReason ?? '—'}
                      {d.deferralEndDate && (
                        <span className="block text-xs text-secondary-400">
                          until {new Date(d.deferralEndDate).toLocaleDateString('en-NZ')}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-secondary-600">{d.totalDonations}</td>
                    <td className="px-4 py-3 text-secondary-600">
                      {d.lastDonationAt
                        ? new Date(d.lastDonationAt).toLocaleDateString('en-NZ')
                        : '—'}
                    </td>
                    <td className="px-4 py-3 text-secondary-600">{d.haemoglobinGdl ?? '—'}</td>
                    <td className="px-4 py-3">
                      <div className="flex gap-1">
                        {(d.status === 'active') && (
                          <button
                            onClick={() => setDeferDonorId(d.id)}
                            className="rounded px-2 py-1 text-xs font-medium text-yellow-700 hover:bg-yellow-50"
                          >
                            Defer
                          </button>
                        )}
                        {(d.status === 'deferred') && (
                          <button
                            onClick={() => handleReinstate(d.id)}
                            className="rounded px-2 py-1 text-xs font-medium text-green-700 hover:bg-green-50"
                          >
                            Reinstate
                          </button>
                        )}
                      </div>
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