import React, { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface RadiologyReport {
  id: string;
  patientNhi: string;
  imagingStudyId: string | null;
  orderId: string | null;
  radiologistHpi: string;
  status: string;
  findings: string;
  impression: string;
  signedAt: string | null;
  amendedAt: string | null;
  amendmentReason: string;
  createdAt: string;
  updatedAt: string;
}

interface ReportListResponse {
  reports: RadiologyReport[];
  total: number;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const STATUSES = ['', 'draft', 'preliminary', 'final', 'amended', 'corrected', 'cancelled'];

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const colours: Record<string, string> = {
    draft: 'bg-secondary-100 text-secondary-600',
    preliminary: 'bg-blue-100 text-blue-700',
    final: 'bg-green-100 text-green-700',
    amended: 'bg-amber-100 text-amber-700',
    corrected: 'bg-purple-100 text-purple-700',
    cancelled: 'bg-red-100 text-red-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colours[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status}
    </span>
  );
}

// Report detail / editor panel
function ReportPanel({
  report,
  onClose,
  onSigned,
  onAmended,
}: {
  report: RadiologyReport;
  onClose: () => void;
  onSigned: () => void;
  onAmended: () => void;
}) {
  const api = useApi();
  const [findings, setFindings] = useState(report.findings);
  const [impression, setImpression] = useState(report.impression);
  const [saving, setSaving] = useState(false);
  const [signing, setSigning] = useState(false);
  const [amendMode, setAmendMode] = useState(false);
  const [amendReason, setAmendReason] = useState('');
  const [panelError, setPanelError] = useState<string | null>(null);

  const isDraft = report.status === 'draft' || report.status === 'preliminary';
  const canAmend = report.status === 'final' || report.status === 'amended';

  async function handleSave() {
    setSaving(true);
    setPanelError(null);
    try {
      await api.put(`/radiology-reports/${report.id}`, { findings, impression });
    } catch {
      setPanelError('Failed to save report');
    } finally {
      setSaving(false);
    }
  }

  async function handleSign() {
    setSigning(true);
    setPanelError(null);
    try {
      await api.post(`/radiology-reports/${report.id}/sign`, {});
      onSigned();
    } catch {
      setPanelError('Failed to sign report — ensure impression is not empty');
    } finally {
      setSigning(false);
    }
  }

  async function handleAmend() {
    if (!amendReason.trim()) {
      setPanelError('Amendment reason is required');
      return;
    }
    setSaving(true);
    setPanelError(null);
    try {
      await api.post(`/radiology-reports/${report.id}/amend`, {
        findings,
        impression,
        amendmentReason: amendReason,
      });
      onAmended();
    } catch {
      setPanelError('Failed to amend report');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-40 flex items-start justify-center overflow-y-auto bg-black/40 px-4 py-8">
      <div className="w-full max-w-2xl rounded-xl bg-white shadow-2xl">
        <div className="flex items-center justify-between border-b border-secondary-200 px-5 py-4">
          <div>
            <h2 className="text-base font-semibold text-secondary-900">Radiology Report</h2>
            <p className="text-xs text-secondary-500">
              Patient: <span className="font-mono font-semibold">{report.patientNhi}</span>
              {' · '}Radiologist: {report.radiologistHpi}
              {' · '}<StatusBadge status={report.status} />
            </p>
          </div>
          <button
            onClick={onClose}
            className="rounded p-1 text-secondary-400 hover:text-secondary-700"
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="space-y-4 p-5">
          {panelError && (
            <div className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 ring-1 ring-red-200">
              {panelError}
            </div>
          )}

          <div>
            <label className="mb-1 block text-xs font-medium text-secondary-700">Findings</label>
            <textarea
              value={findings}
              onChange={(e) => setFindings(e.target.value)}
              disabled={!isDraft && !amendMode}
              rows={6}
              placeholder="Describe imaging findings…"
              className="w-full rounded-md border border-secondary-300 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none disabled:bg-secondary-50 disabled:text-secondary-600"
            />
          </div>

          <div>
            <label className="mb-1 block text-xs font-medium text-secondary-700">Impression</label>
            <textarea
              value={impression}
              onChange={(e) => setImpression(e.target.value)}
              disabled={!isDraft && !amendMode}
              rows={4}
              placeholder="Clinical impression and recommendations…"
              className="w-full rounded-md border border-secondary-300 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none disabled:bg-secondary-50 disabled:text-secondary-600"
            />
          </div>

          {amendMode && (
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">Amendment Reason *</label>
              <input
                type="text"
                value={amendReason}
                onChange={(e) => setAmendReason(e.target.value)}
                placeholder="Reason for amendment…"
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
          )}

          {report.signedAt && (
            <p className="text-xs text-secondary-500">
              Signed: {new Date(report.signedAt).toLocaleString('en-NZ')}
            </p>
          )}
          {report.amendedAt && (
            <p className="text-xs text-secondary-500">
              Amended: {new Date(report.amendedAt).toLocaleString('en-NZ')} — {report.amendmentReason}
            </p>
          )}
        </div>

        <div className="flex justify-end gap-2 border-t border-secondary-200 px-5 py-3">
          <button
            onClick={onClose}
            className="rounded-md px-3 py-1.5 text-sm text-secondary-600 hover:text-secondary-900"
          >
            Close
          </button>
          {isDraft && (
            <>
              <button
                onClick={() => void handleSave()}
                disabled={saving}
                className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm font-medium text-secondary-700 hover:bg-secondary-50 disabled:opacity-50"
              >
                {saving ? 'Saving…' : 'Save Draft'}
              </button>
              <button
                onClick={() => void handleSign()}
                disabled={signing || !impression}
                className="rounded-md bg-green-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50"
              >
                {signing ? 'Signing…' : 'Sign Report'}
              </button>
            </>
          )}
          {canAmend && !amendMode && (
            <button
              onClick={() => setAmendMode(true)}
              className="rounded-md bg-amber-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-amber-700"
            >
              Amend
            </button>
          )}
          {amendMode && (
            <button
              onClick={() => void handleAmend()}
              disabled={saving}
              className="rounded-md bg-amber-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-amber-700 disabled:opacity-50"
            >
              {saving ? 'Amending…' : 'Submit Amendment'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function RadiologyReportPage() {
  const api = useApi();

  const [reports, setReports] = useState<RadiologyReport[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [statusFilter, setStatusFilter] = useState('');
  const [nhiFilter, setNhiFilter] = useState('');

  // Create form
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ patientNhi: '', radiologistHpi: '', imagingStudyId: '', orderId: '' });
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Detail panel
  const [selectedReport, setSelectedReport] = useState<RadiologyReport | null>(null);

  async function loadReports() {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = {};
      if (statusFilter) params.status = statusFilter;
      if (nhiFilter) params.patientNhi = nhiFilter;
      const data = await api.get<ReportListResponse>('/radiology-reports', { params });
      setReports(data.reports ?? []);
    } catch {
      setError('Failed to load radiology reports');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadReports();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleCreateReport(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setFormError(null);
    try {
      const body: Record<string, string> = {
        patientNhi: form.patientNhi,
        radiologistHpi: form.radiologistHpi,
      };
      if (form.imagingStudyId) body.imagingStudyId = form.imagingStudyId;
      if (form.orderId) body.orderId = form.orderId;
      await api.post('/radiology-reports', body);
      setShowForm(false);
      setForm({ patientNhi: '', radiologistHpi: '', imagingStudyId: '', orderId: '' });
      await loadReports();
    } catch {
      setFormError('Failed to create report draft');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AppShell title="Radiology Reports">
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
            onClick={() => void loadReports()}
            className="rounded-md bg-secondary-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-secondary-800"
          >
            Search
          </button>
        </div>
        <button
          onClick={() => setShowForm((v) => !v)}
          className="ml-auto rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {showForm ? 'Cancel' : '+ New Report'}
        </button>
      </div>

      {/* Create form */}
      {showForm && (
        <form
          onSubmit={(e) => void handleCreateReport(e)}
          className="mb-6 rounded-xl border border-primary-200 bg-primary-50 p-4"
        >
          <h3 className="mb-3 text-sm font-semibold text-secondary-900">New Report Draft</h3>
          {formError && <p className="mb-2 text-xs text-red-600">{formError}</p>}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {[
              { label: 'Patient NHI *', key: 'patientNhi', placeholder: 'AAA1234' },
              { label: 'Radiologist HPI *', key: 'radiologistHpi', placeholder: 'HPI-CPN' },
              { label: 'Imaging Study ID', key: 'imagingStudyId', placeholder: 'UUID (optional)' },
              { label: 'Order ID', key: 'orderId', placeholder: 'UUID (optional)' },
            ].map(({ label, key, placeholder }) => (
              <div key={key}>
                <label className="mb-1 block text-xs font-medium text-secondary-700">{label}</label>
                <input
                  type="text"
                  placeholder={placeholder}
                  value={form[key as keyof typeof form]}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
                />
              </div>
            ))}
          </div>
          <div className="mt-3 flex justify-end gap-2">
            <button type="button" onClick={() => setShowForm(false)} className="rounded-md px-3 py-1.5 text-sm text-secondary-600">
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50"
            >
              {submitting ? 'Creating…' : 'Create Draft'}
            </button>
          </div>
        </form>
      )}

      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">{error}</div>
      )}

      {/* Reports table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
          </div>
        ) : reports.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-secondary-500">No radiology reports found</p>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
              <tr>
                <th className="px-4 py-3 text-left">Patient NHI</th>
                <th className="px-4 py-3 text-left">Radiologist</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-left">Created</th>
                <th className="px-4 py-3 text-left">Signed</th>
                <th className="px-4 py-3 text-left">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-secondary-100">
              {reports.map((rep) => (
                <tr key={rep.id} className="hover:bg-secondary-50">
                  <td className="px-4 py-3 font-mono text-xs font-semibold text-secondary-900">{rep.patientNhi}</td>
                  <td className="px-4 py-3 text-secondary-700">{rep.radiologistHpi}</td>
                  <td className="px-4 py-3"><StatusBadge status={rep.status} /></td>
                  <td className="px-4 py-3 text-secondary-600">
                    {new Date(rep.createdAt).toLocaleDateString('en-NZ')}
                  </td>
                  <td className="px-4 py-3 text-secondary-600">
                    {rep.signedAt ? new Date(rep.signedAt).toLocaleDateString('en-NZ') : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <button
                      onClick={() => setSelectedReport(rep)}
                      className="text-xs text-primary-600 hover:underline"
                    >
                      {rep.status === 'draft' || rep.status === 'preliminary' ? 'Edit / Sign' : 'View'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Detail panel */}
      {selectedReport && (
        <ReportPanel
          report={selectedReport}
          onClose={() => setSelectedReport(null)}
          onSigned={() => { setSelectedReport(null); void loadReports(); }}
          onAmended={() => { setSelectedReport(null); void loadReports(); }}
        />
      )}
    </AppShell>
  );
}
