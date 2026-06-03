import { useState } from 'react';

interface Application {
  id: string;
  practice_name: string;
  hpi_facility_id: string;
  contact_name: string;
  contact_email: string;
  contact_hpi_cpn?: string;
  status: 'pending' | 'approved' | 'rejected';
  submitted_at: string;
  reviewer_notes?: string;
}

interface Tenant {
  id: string;
  name: string;
  hpi_facility_id: string;
  status: 'active' | 'suspended';
  contact_email: string;
  contact_name: string;
  created_at: string;
}

type Tab = 'applications' | 'tenants';

const API_BASE = import.meta.env.VITE_API_URL ?? '';

async function listApplications(status: string): Promise<Application[]> {
  const url = status
    ? `${API_BASE}/api/v1/admin/applications?status=${status}`
    : `${API_BASE}/api/v1/admin/applications`;
  const res = await fetch(url, { credentials: 'include' });
  if (!res.ok) throw new Error('Failed to load applications');
  const data = await res.json();
  return data.applications ?? [];
}

async function listTenants(): Promise<Tenant[]> {
  const res = await fetch(`${API_BASE}/api/v1/admin/tenants`, { credentials: 'include' });
  if (!res.ok) throw new Error('Failed to load tenants');
  const data = await res.json();
  return data.tenants ?? [];
}

async function reviewApplication(
  id: string,
  action: 'approve' | 'reject',
  notes: string,
): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/admin/applications/${id}/${action}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ notes }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Failed to ${action} application`);
  }
}

function StatusBadge({ status }: { status: string }) {
  const colours: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-800',
    approved: 'bg-green-100 text-green-800',
    rejected: 'bg-red-100 text-red-800',
    active: 'bg-green-100 text-green-800',
    suspended: 'bg-gray-100 text-gray-700',
  };
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${colours[status] ?? 'bg-gray-100 text-gray-700'}`}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}

function ReviewModal({
  app,
  action,
  onConfirm,
  onCancel,
}: {
  app: Application;
  action: 'approve' | 'reject';
  onConfirm: (notes: string) => void;
  onCancel: () => void;
}) {
  const [notes, setNotes] = useState('');
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="bg-white rounded-xl shadow-xl w-full max-w-md mx-4 p-6">
        <h3 className="text-lg font-semibold text-gray-900 mb-1">
          {action === 'approve' ? 'Approve Application' : 'Reject Application'}
        </h3>
        <p className="text-sm text-gray-500 mb-4">
          {app.practice_name} — {app.hpi_facility_id}
        </p>
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Notes <span className="text-gray-400 font-normal">(optional)</span>
        </label>
        <textarea
          className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand-500 resize-none"
          rows={3}
          value={notes}
          onChange={e => setNotes(e.target.value)}
          placeholder={action === 'approve' ? 'Welcome message or setup instructions…' : 'Reason for rejection…'}
        />
        <div className="flex gap-3 mt-5 justify-end">
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50"
          >
            Cancel
          </button>
          <button
            onClick={() => onConfirm(notes)}
            className={`px-4 py-2 text-sm font-medium text-white rounded-lg ${
              action === 'approve'
                ? 'bg-green-600 hover:bg-green-700'
                : 'bg-red-600 hover:bg-red-700'
            }`}
          >
            {action === 'approve' ? 'Approve & Provision' : 'Reject'}
          </button>
        </div>
      </div>
    </div>
  );
}

export function ClinicsPage() {
  const [tab, setTab] = useState<Tab>('applications');
  const [statusFilter, setStatusFilter] = useState('pending');

  const [applications, setApplications] = useState<Application[] | null>(null);
  const [tenants, setTenants] = useState<Tenant[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [modal, setModal] = useState<{ app: Application; action: 'approve' | 'reject' } | null>(null);

  const loadApplications = async (status: string) => {
    setLoading(true);
    setError(null);
    try {
      setApplications(await listApplications(status));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const loadTenants = async () => {
    setLoading(true);
    setError(null);
    try {
      setTenants(await listTenants());
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const handleTabChange = (next: Tab) => {
    setTab(next);
    setError(null);
    if (next === 'applications') loadApplications(statusFilter);
    else loadTenants();
  };

  const handleFilterChange = (status: string) => {
    setStatusFilter(status);
    loadApplications(status);
  };

  const handleReview = async (notes: string) => {
    if (!modal) return;
    try {
      await reviewApplication(modal.app.id, modal.action, notes);
      setModal(null);
      loadApplications(statusFilter);
    } catch (e) {
      setError((e as Error).message);
      setModal(null);
    }
  };

  // Initial load.
  if (applications === null && tenants === null && !loading && !error) {
    loadApplications(statusFilter);
  }

  return (
    <div className="p-8">
      {modal && (
        <ReviewModal
          app={modal.app}
          action={modal.action}
          onConfirm={handleReview}
          onCancel={() => setModal(null)}
        />
      )}

      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Clinics</h1>
        <p className="text-sm text-gray-500 mt-1">
          Manage clinic applications and active network tenants.
        </p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-gray-200 mb-6">
        {(['applications', 'tenants'] as Tab[]).map(t => (
          <button
            key={t}
            onClick={() => handleTabChange(t)}
            className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors ${
              tab === t
                ? 'border-brand-600 text-brand-700'
                : 'border-transparent text-gray-500 hover:text-gray-700'
            }`}
          >
            {t === 'applications' ? 'Applications' : 'Active Clinics'}
          </button>
        ))}
      </div>

      {tab === 'applications' && (
        <>
          {/* Status filter */}
          <div className="flex gap-2 mb-4">
            {['pending', 'approved', 'rejected', ''].map(s => (
              <button
                key={s || 'all'}
                onClick={() => handleFilterChange(s)}
                className={`px-3 py-1.5 text-xs font-medium rounded-full border transition-colors ${
                  statusFilter === s
                    ? 'bg-brand-600 text-white border-brand-600'
                    : 'bg-white text-gray-600 border-gray-300 hover:border-brand-400'
                }`}
              >
                {s === '' ? 'All' : s.charAt(0).toUpperCase() + s.slice(1)}
              </button>
            ))}
          </div>

          {error && <p className="text-sm text-red-600 mb-4">{error}</p>}

          {loading ? (
            <p className="text-sm text-gray-500">Loading…</p>
          ) : applications && applications.length === 0 ? (
            <p className="text-sm text-gray-400">No applications found.</p>
          ) : (
            <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
              <table className="min-w-full divide-y divide-gray-100">
                <thead className="bg-gray-50">
                  <tr>
                    {['Practice', 'HPI Facility ID', 'Contact', 'Submitted', 'Status', ''].map(h => (
                      <th
                        key={h}
                        className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide"
                      >
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {(applications ?? []).map(app => (
                    <tr key={app.id} className="hover:bg-gray-50">
                      <td className="px-4 py-3">
                        <p className="text-sm font-medium text-gray-900">{app.practice_name}</p>
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-600 font-mono">{app.hpi_facility_id}</td>
                      <td className="px-4 py-3">
                        <p className="text-sm text-gray-900">{app.contact_name}</p>
                        <p className="text-xs text-gray-400">{app.contact_email}</p>
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-500">
                        {new Date(app.submitted_at).toLocaleDateString('en-NZ')}
                      </td>
                      <td className="px-4 py-3">
                        <StatusBadge status={app.status} />
                      </td>
                      <td className="px-4 py-3">
                        {app.status === 'pending' && (
                          <div className="flex gap-2 justify-end">
                            <button
                              onClick={() => setModal({ app, action: 'approve' })}
                              className="px-3 py-1 text-xs font-medium text-white bg-green-600 rounded-lg hover:bg-green-700"
                            >
                              Approve
                            </button>
                            <button
                              onClick={() => setModal({ app, action: 'reject' })}
                              className="px-3 py-1 text-xs font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-red-50 hover:text-red-600 hover:border-red-300"
                            >
                              Reject
                            </button>
                          </div>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {tab === 'tenants' && (
        <>
          {error && <p className="text-sm text-red-600 mb-4">{error}</p>}
          {loading ? (
            <p className="text-sm text-gray-500">Loading…</p>
          ) : tenants && tenants.length === 0 ? (
            <p className="text-sm text-gray-400">No active clinics yet.</p>
          ) : (
            <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
              <table className="min-w-full divide-y divide-gray-100">
                <thead className="bg-gray-50">
                  <tr>
                    {['Clinic', 'HPI Facility ID', 'Tenant ID', 'Contact', 'Joined', 'Status'].map(h => (
                      <th
                        key={h}
                        className="px-4 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide"
                      >
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-50">
                  {(tenants ?? []).map(t => (
                    <tr key={t.id} className="hover:bg-gray-50">
                      <td className="px-4 py-3 text-sm font-medium text-gray-900">{t.name}</td>
                      <td className="px-4 py-3 text-sm text-gray-600 font-mono">{t.hpi_facility_id}</td>
                      <td className="px-4 py-3">
                        <span className="text-xs text-gray-400 font-mono">{t.id}</span>
                      </td>
                      <td className="px-4 py-3">
                        <p className="text-sm text-gray-900">{t.contact_name}</p>
                        <p className="text-xs text-gray-400">{t.contact_email}</p>
                      </td>
                      <td className="px-4 py-3 text-sm text-gray-500">
                        {new Date(t.created_at).toLocaleDateString('en-NZ')}
                      </td>
                      <td className="px-4 py-3">
                        <StatusBadge status={t.status} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}
