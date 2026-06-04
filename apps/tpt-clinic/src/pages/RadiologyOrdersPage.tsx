import React, { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface RadiologyOrder {
  id: string;
  patientNhi: string;
  imagingStudyId: string | null;
  modality: string;
  bodyPart: string;
  clinicalInfo: string;
  priority: string;
  status: string;
  referringHpi: string;
  requestedAt: string;
  scheduledAt: string | null;
  completedAt: string | null;
  loincCode: string;
  loincDisplay: string;
}

interface OrderListResponse {
  orders: RadiologyOrder[];
  total: number;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const MODALITIES = ['CT', 'MR', 'XR', 'US', 'NM', 'PT', 'CR', 'DX', 'MG', 'RF'];
const PRIORITIES = ['routine', 'urgent', 'stat'];
const STATUSES = ['', 'draft', 'active', 'completed', 'cancelled'];

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function PriorityBadge({ priority }: { priority: string }) {
  const colours: Record<string, string> = {
    stat: 'bg-red-100 text-red-700',
    urgent: 'bg-amber-100 text-amber-700',
    routine: 'bg-secondary-100 text-secondary-600',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${colours[priority] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {priority}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colours: Record<string, string> = {
    draft: 'bg-secondary-100 text-secondary-600',
    active: 'bg-blue-100 text-blue-700',
    completed: 'bg-green-100 text-green-700',
    cancelled: 'bg-red-100 text-red-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colours[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function RadiologyOrdersPage() {
  const api = useApi();

  const [orders, setOrders] = useState<RadiologyOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [statusFilter, setStatusFilter] = useState('');
  const [nhiFilter, setNhiFilter] = useState('');

  // New order form
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({
    patientNhi: '',
    modality: 'CT',
    bodyPart: '',
    clinicalInfo: '',
    priority: 'routine',
    referringHpi: '',
    loincCode: '',
    loincDisplay: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Action state
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  async function loadOrders() {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = {};
      if (statusFilter) params.status = statusFilter;
      if (nhiFilter) params.patientNhi = nhiFilter;
      const data = await api.get<OrderListResponse>('/radiology-orders', { params });
      setOrders(data.orders ?? []);
    } catch {
      setError('Failed to load radiology orders');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadOrders();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleCreateOrder(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setFormError(null);
    try {
      await api.post('/radiology-orders', form);
      setShowForm(false);
      setForm({ patientNhi: '', modality: 'CT', bodyPart: '', clinicalInfo: '', priority: 'routine', referringHpi: '', loincCode: '', loincDisplay: '' });
      await loadOrders();
    } catch {
      setFormError('Failed to create order — check all required fields');
    } finally {
      setSubmitting(false);
    }
  }

  async function handleAction(id: string, action: 'complete' | 'cancel') {
    setActionLoading(id + action);
    try {
      await api.post(`/radiology-orders/${id}/${action}`, {});
      await loadOrders();
    } catch {
      // Silently ignored — a toast system would handle this in production
    } finally {
      setActionLoading(null);
    }
  }

  return (
    <AppShell title="Radiology Orders">
      {/* Toolbar */}
      <div className="mb-4 flex flex-wrap items-end gap-3">
        <div className="flex flex-wrap gap-2">
          <input
            type="text"
            placeholder="Patient NHI"
            value={nhiFilter}
            onChange={(e) => setNhiFilter(e.target.value.toUpperCase())}
            className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
          />
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
          >
            {STATUSES.map((s) => (
              <option key={s} value={s}>{s === '' ? 'All statuses' : s}</option>
            ))}
          </select>
          <button
            onClick={() => void loadOrders()}
            className="rounded-md bg-secondary-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-secondary-800"
          >
            Search
          </button>
        </div>
        <button
          onClick={() => setShowForm((v) => !v)}
          className="ml-auto rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {showForm ? 'Cancel' : '+ New Order'}
        </button>
      </div>

      {/* New order form */}
      {showForm && (
        <form
          onSubmit={(e) => void handleCreateOrder(e)}
          className="mb-6 rounded-xl border border-primary-200 bg-primary-50 p-4"
        >
          <h3 className="mb-3 text-sm font-semibold text-secondary-900">New Radiology Order</h3>
          {formError && <p className="mb-2 text-xs text-red-600">{formError}</p>}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">Patient NHI *</label>
              <input
                type="text"
                placeholder="AAA1234"
                value={form.patientNhi}
                onChange={(e) => setForm((f) => ({ ...f, patientNhi: e.target.value.toUpperCase() }))}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">Referring HPI *</label>
              <input
                type="text"
                placeholder="HPI-CPN"
                value={form.referringHpi}
                onChange={(e) => setForm((f) => ({ ...f, referringHpi: e.target.value }))}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">Modality *</label>
              <select
                value={form.modality}
                onChange={(e) => setForm((f) => ({ ...f, modality: e.target.value }))}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              >
                {MODALITIES.map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">Priority</label>
              <select
                value={form.priority}
                onChange={(e) => setForm((f) => ({ ...f, priority: e.target.value }))}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              >
                {PRIORITIES.map((p) => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">Body Part</label>
              <input
                type="text"
                placeholder="CHEST"
                value={form.bodyPart}
                onChange={(e) => setForm((f) => ({ ...f, bodyPart: e.target.value }))}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">LOINC Code</label>
              <input
                type="text"
                placeholder="24629-2"
                value={form.loincCode}
                onChange={(e) => setForm((f) => ({ ...f, loincCode: e.target.value }))}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
            <div className="sm:col-span-2 lg:col-span-3">
              <label className="mb-1 block text-xs font-medium text-secondary-700">Clinical Information</label>
              <textarea
                placeholder="Clinical indication, relevant history…"
                value={form.clinicalInfo}
                onChange={(e) => setForm((f) => ({ ...f, clinicalInfo: e.target.value }))}
                rows={2}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
          </div>
          <div className="mt-3 flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setShowForm(false)}
              className="rounded-md px-3 py-1.5 text-sm text-secondary-600 hover:text-secondary-900"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50"
            >
              {submitting ? 'Creating…' : 'Create Order'}
            </button>
          </div>
        </form>
      )}

      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">{error}</div>
      )}

      {/* Orders table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
          </div>
        ) : orders.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-secondary-500">No radiology orders found</p>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
              <tr>
                <th className="px-4 py-3 text-left">Patient NHI</th>
                <th className="px-4 py-3 text-left">Modality</th>
                <th className="px-4 py-3 text-left">Body Part</th>
                <th className="px-4 py-3 text-left">Priority</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-left">Requested</th>
                <th className="px-4 py-3 text-left">Scheduled</th>
                <th className="px-4 py-3 text-left">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-secondary-100">
              {orders.map((o) => (
                <tr key={o.id} className="hover:bg-secondary-50">
                  <td className="px-4 py-3 font-mono text-xs font-semibold text-secondary-900">{o.patientNhi}</td>
                  <td className="px-4 py-3 font-medium text-secondary-700">{o.modality}</td>
                  <td className="px-4 py-3 text-secondary-600">{o.bodyPart || '—'}</td>
                  <td className="px-4 py-3"><PriorityBadge priority={o.priority} /></td>
                  <td className="px-4 py-3"><StatusBadge status={o.status} /></td>
                  <td className="px-4 py-3 text-secondary-600">
                    {new Date(o.requestedAt).toLocaleDateString('en-NZ')}
                  </td>
                  <td className="px-4 py-3 text-secondary-600">
                    {o.scheduledAt ? new Date(o.scheduledAt).toLocaleDateString('en-NZ') : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-2">
                      {(o.status === 'draft' || o.status === 'active') && (
                        <button
                          onClick={() => void handleAction(o.id, 'complete')}
                          disabled={actionLoading === o.id + 'complete'}
                          className="text-xs text-green-600 hover:underline disabled:opacity-50"
                        >
                          Complete
                        </button>
                      )}
                      {o.status !== 'completed' && o.status !== 'cancelled' && (
                        <button
                          onClick={() => void handleAction(o.id, 'cancel')}
                          disabled={actionLoading === o.id + 'cancel'}
                          className="text-xs text-red-500 hover:underline disabled:opacity-50"
                        >
                          Cancel
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </AppShell>
  );
}
