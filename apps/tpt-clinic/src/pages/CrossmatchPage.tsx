import React, { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Crossmatch {
  id: string;
  patientId: string;
  patientNhi: string;
  patientAbo: string;
  patientRhd: string;
  antibodyScreen: string;
  productUnitIds: string[];
  status: string;
  compatibility: string;
  requestedBy: string;
  issuedBy: string | null;
  transfusedBy: string | null;
  emergencyReason: string | null;
  notes: string;
  requestedAt: string;
  issuedAt: string | null;
  transfusedAt: string | null;
  cancelledAt: string | null;
}

interface CrossmatchListResponse {
  crossmatches: Crossmatch[];
  total: number;
}

// ---------------------------------------------------------------------------
// Status badge
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    pending: 'bg-secondary-100 text-secondary-600',
    matched: 'bg-blue-100 text-blue-700',
    issued: 'bg-amber-100 text-amber-700',
    transfused: 'bg-green-100 text-green-700',
    cancelled: 'bg-red-100 text-red-700',
    incompatible: 'bg-yellow-100 text-yellow-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colors[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Compatibility badge
// ---------------------------------------------------------------------------

function CompatibilityBadge({ compat }: { compat: string }) {
  const colors: Record<string, string> = {
    compatible: 'bg-green-100 text-green-700',
    'caution-antibody-screen-positive': 'bg-yellow-100 text-yellow-700',
    'emergency-release': 'bg-red-100 text-red-700',
    incompatible: 'bg-red-100 text-red-700',
  };
  const labels: Record<string, string> = {
    compatible: 'Compatible',
    'caution-antibody-screen-positive': 'Ab Screen+',
    'emergency-release': 'Emergency',
    incompatible: 'Incompatible',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colors[compat] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {labels[compat] ?? compat}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Create / Emergency modal
// ---------------------------------------------------------------------------

function CreateCrossmatchModal({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const api = useApi();
  const [patientNhi, setPatientNhi] = useState('');
  const [patientAbo, setPatientAbo] = useState('O');
  const [patientRhd, setPatientRhd] = useState('POSITIVE');
  const [antibodyScreen, setAntibodyScreen] = useState('negative');
  const [productUnitIds, setProductUnitIds] = useState('');
  const [requestedBy, setRequestedBy] = useState('');
  const [notes, setNotes] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api.post('/crossmatches', {
        patientNhi,
        patientAbo,
        patientRhd,
        antibodyScreen,
        productUnitIds: productUnitIds.split(',').map((s) => s.trim()).filter(Boolean),
        requestedBy,
        notes,
      });
      onCreated();
    } catch {
      setError('Failed to create crossmatch');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-lg rounded-xl bg-white p-6 shadow-lg">
        <h3 className="mb-4 text-sm font-semibold text-secondary-900">New Cross-match</h3>
        {error && (
          <div className="mb-3 rounded bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>
        )}
        <form onSubmit={handleSubmit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-secondary-700">Patient NHI</label>
              <input
                type="text"
                value={patientNhi}
                onChange={(e) => setPatientNhi(e.target.value.toUpperCase())}
                placeholder="ABC1234"
                required
                className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 font-mono text-sm"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-secondary-700">Requested By</label>
              <input
                type="text"
                value={requestedBy}
                onChange={(e) => setRequestedBy(e.target.value)}
                required
                className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-secondary-700">ABO</label>
              <select value={patientAbo} onChange={(e) => setPatientAbo(e.target.value)} className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm">
                <option value="A">A</option><option value="B">B</option><option value="AB">AB</option><option value="O">O</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-secondary-700">RhD</label>
              <select value={patientRhd} onChange={(e) => setPatientRhd(e.target.value)} className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm">
                <option value="POSITIVE">Positive</option>
                <option value="NEGATIVE">Negative</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary-700">Antibody Screen</label>
            <select value={antibodyScreen} onChange={(e) => setAntibodyScreen(e.target.value)} className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm">
              <option value="negative">Negative</option>
              <option value="positive">Positive</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary-700">Product Unit IDs (comma-separated)</label>
            <input
              type="text"
              value={productUnitIds}
              onChange={(e) => setProductUnitIds(e.target.value)}
              placeholder="uuid-1, uuid-2"
              required
              className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 font-mono text-sm"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary-700">Notes</label>
            <textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={2} className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm" />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <button type="button" onClick={onClose} className="rounded-md bg-white px-4 py-2 text-sm font-medium text-secondary-700 ring-1 ring-secondary-300">
              Cancel
            </button>
            <button type="submit" disabled={submitting} className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50">
              {submitting ? 'Creating…' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Emergency release modal
// ---------------------------------------------------------------------------

function EmergencyReleaseModal({
  crossmatchId,
  onClose,
  onReleased,
}: {
  crossmatchId: string;
  onClose: () => void;
  onReleased: () => void;
}) {
  const api = useApi();
  const [approvedBy, setApprovedBy] = useState('');
  const [clinicalReason, setClinicalReason] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await api.post(`/crossmatches/${crossmatchId}/emergency`, { approvedBy, clinicalReason });
      onReleased();
    } catch {
      setError('Emergency release failed');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-xl bg-white p-6 shadow-lg">
        <h3 className="mb-4 text-sm font-semibold text-red-700">Emergency Release</h3>
        <p className="mb-3 text-xs text-secondary-500">This bypasses full crossmatching for life-threatening situations.</p>
        {error && <div className="mb-3 rounded bg-red-50 px-3 py-2 text-xs text-red-700">{error}</div>}
        <form onSubmit={handleSubmit} className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-secondary-700">Approved By</label>
            <input type="text" value={approvedBy} onChange={(e) => setApprovedBy(e.target.value)} required className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="block text-xs font-medium text-secondary-700">Clinical Reason</label>
            <textarea value={clinicalReason} onChange={(e) => setClinicalReason(e.target.value)} required rows={2} className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm" />
          </div>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="rounded-md bg-white px-4 py-2 text-sm font-medium text-secondary-700 ring-1 ring-secondary-300">
              Cancel
            </button>
            <button type="submit" disabled={submitting} className="rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50">
              {submitting ? 'Releasing…' : 'Emergency Release'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function CrossmatchPage() {
  const api = useApi();

  const [crossmatches, setCrossmatches] = useState<Crossmatch[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] = useState('');

  // Modals
  const [showCreate, setShowCreate] = useState(false);
  const [emergencyId, setEmergencyId] = useState<string | null>(null);

  useEffect(() => {
    loadData();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter]);

  async function loadData() {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = {};
      if (statusFilter) params['status'] = statusFilter;
      const data = await api.get<CrossmatchListResponse>('/crossmatches', { params });
      setCrossmatches(data.crossmatches);
      setTotal(data.total);
    } catch {
      setError('Failed to load crossmatches');
    } finally {
      setLoading(false);
    }
  }

  async function handleIssue(id: string) {
    try {
      const issuedBy = prompt('Issued by:');
      if (!issuedBy) return;
      await api.post(`/crossmatches/${id}/issue`, { issuedBy });
      await loadData();
    } catch {
      setError('Failed to issue');
    }
  }

  async function handleTransfuse(id: string) {
    try {
      const transfusedBy = prompt('Transfused by:');
      if (!transfusedBy) return;
      await api.post(`/crossmatches/${id}/transfuse`, { transfusedBy });
      await loadData();
    } catch {
      setError('Failed to record transfusion');
    }
  }

  async function handleCancel(id: string) {
    try {
      const reason = prompt('Reason for cancellation:') ?? '';
      await api.post(`/crossmatches/${id}/cancel`, { reason });
      await loadData();
    } catch {
      setError('Failed to cancel');
    }
  }

  return (
    <AppShell title="Cross-matching">
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {/* Toolbar */}
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm"
        >
          <option value="">All Statuses</option>
          <option value="matched">Matched</option>
          <option value="issued">Issued</option>
          <option value="transfused">Transfused</option>
          <option value="cancelled">Cancelled</option>
          <option value="incompatible">Incompatible</option>
        </select>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-1.5 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 sm:ml-auto"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
          </svg>
          New Cross-match
        </button>
      </div>

      {/* Create modal */}
      {showCreate && (
        <CreateCrossmatchModal
          onClose={() => setShowCreate(false)}
          onCreated={() => {
            setShowCreate(false);
            void loadData();
          }}
        />
      )}

      {/* Emergency release modal */}
      {emergencyId && (
        <EmergencyReleaseModal
          crossmatchId={emergencyId}
          onClose={() => setEmergencyId(null)}
          onReleased={() => {
            setEmergencyId(null);
            void loadData();
          }}
        />
      )}

      {/* Table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        <div className="border-b border-secondary-100 px-4 py-3">
          <p className="text-sm text-secondary-500">
            {loading ? 'Loading…' : `${total.toLocaleString()} cross-match${total !== 1 ? 'es' : ''}`}
          </p>
        </div>

        {!loading && crossmatches.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-secondary-500">No cross-matches found.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                <tr>
                  <th className="px-4 py-3 text-left">Patient</th>
                  <th className="px-4 py-3 text-left">Type</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-left">Compatibility</th>
                  <th className="px-4 py-3 text-left">Antibody</th>
                  <th className="px-4 py-3 text-left">Units</th>
                  <th className="px-4 py-3 text-left">Requested</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {crossmatches.map((xm) => (
                  <tr key={xm.id} className="hover:bg-secondary-50">
                    <td className="px-4 py-3 font-mono text-secondary-900">{xm.patientNhi || xm.patientId}</td>
                    <td className="px-4 py-3 font-mono text-secondary-600">
                      {xm.patientAbo}{xm.patientRhd === 'POSITIVE' ? '+' : '-'}
                    </td>
                    <td className="px-4 py-3"><StatusBadge status={xm.status} /></td>
                    <td className="px-4 py-3"><CompatibilityBadge compat={xm.compatibility} /></td>
                    <td className="px-4 py-3 text-secondary-600">{xm.antibodyScreen}</td>
                    <td className="px-4 py-3 text-secondary-600">{xm.productUnitIds.length}</td>
                    <td className="px-4 py-3 text-secondary-600">{new Date(xm.requestedAt).toLocaleDateString('en-NZ')}</td>
                    <td className="px-4 py-3">
                      <div className="flex flex-wrap gap-1">
                        {xm.status === 'matched' && (
                          <>
                            <button onClick={() => handleIssue(xm.id)} className="rounded px-2 py-1 text-xs font-medium text-amber-700 hover:bg-amber-50">Issue</button>
                            <button onClick={() => setEmergencyId(xm.id)} className="rounded px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-50">Emergency</button>
                            <button onClick={() => handleCancel(xm.id)} className="rounded px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-50">Cancel</button>
                          </>
                        )}
                        {xm.status === 'issued' && (
                          <button onClick={() => handleTransfuse(xm.id)} className="rounded px-2 py-1 text-xs font-medium text-green-700 hover:bg-green-50">Transfuse</button>
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