import React, { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface ImagingStudy {
  id: string;
  patientNhi: string;
  studyInstanceUid: string;
  accessionNumber: string;
  modality: string;
  bodyPart: string;
  studyDate: string | null;
  description: string;
  referringHpi: string;
  performingHpi: string;
  status: string;
  numSeries: number;
  numInstances: number;
  createdAt: string;
}

interface StudyListResponse {
  studies: ImagingStudy[];
  total: number;
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const MODALITIES = ['', 'CT', 'MR', 'XR', 'US', 'NM', 'PT', 'CR', 'DX', 'MG', 'RF'];
const STATUSES = ['', 'registered', 'available', 'cancelled', 'entered-in-error'];

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function ModalityBadge({ modality }: { modality: string }) {
  const colours: Record<string, string> = {
    CT: 'bg-blue-100 text-blue-700',
    MR: 'bg-purple-100 text-purple-700',
    XR: 'bg-green-100 text-green-700',
    US: 'bg-teal-100 text-teal-700',
    NM: 'bg-orange-100 text-orange-700',
    PT: 'bg-yellow-100 text-yellow-700',
    CR: 'bg-slate-100 text-slate-700',
    DX: 'bg-slate-100 text-slate-700',
    MG: 'bg-pink-100 text-pink-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${colours[modality] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {modality}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function ImagingStudiesPage() {
  const api = useApi();

  const [studies, setStudies] = useState<ImagingStudy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [patientNhi, setPatientNhi] = useState('');
  const [modality, setModality] = useState('');
  const [status, setStatus] = useState('');

  // New study form
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({
    patientNhi: '',
    studyInstanceUid: '',
    modality: 'CT',
    bodyPart: '',
    description: '',
    referringHpi: '',
  });
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  async function loadStudies() {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = {};
      if (patientNhi) params.patientNhi = patientNhi;
      if (modality) params.modality = modality;
      if (status) params.status = status;
      const data = await api.get<StudyListResponse>('/imaging-studies', { params });
      setStudies(data.studies ?? []);
    } catch {
      setError('Failed to load imaging studies');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void loadStudies();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleRegisterStudy(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setFormError(null);
    try {
      await api.post('/imaging-studies', form);
      setShowForm(false);
      setForm({ patientNhi: '', studyInstanceUid: '', modality: 'CT', bodyPart: '', description: '', referringHpi: '' });
      await loadStudies();
    } catch {
      setFormError('Failed to register study — check all fields and try again');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AppShell title="Imaging Studies">
      {/* Toolbar */}
      <div className="mb-4 flex flex-wrap items-end gap-3">
        <div className="flex flex-wrap gap-2">
          <input
            type="text"
            placeholder="Patient NHI"
            value={patientNhi}
            onChange={(e) => setPatientNhi(e.target.value.toUpperCase())}
            className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          />
          <select
            value={modality}
            onChange={(e) => setModality(e.target.value)}
            className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
          >
            {MODALITIES.map((m) => (
              <option key={m} value={m}>{m === '' ? 'All modalities' : m}</option>
            ))}
          </select>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
          >
            {STATUSES.map((s) => (
              <option key={s} value={s}>{s === '' ? 'All statuses' : s}</option>
            ))}
          </select>
          <button
            onClick={() => void loadStudies()}
            className="rounded-md bg-secondary-700 px-3 py-1.5 text-sm font-medium text-white hover:bg-secondary-800"
          >
            Search
          </button>
        </div>

        <button
          onClick={() => setShowForm((v) => !v)}
          className="ml-auto rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {showForm ? 'Cancel' : '+ Register Study'}
        </button>
      </div>

      {/* New study form */}
      {showForm && (
        <form
          onSubmit={(e) => void handleRegisterStudy(e)}
          className="mb-6 rounded-xl border border-primary-200 bg-primary-50 p-4"
        >
          <h3 className="mb-3 text-sm font-semibold text-secondary-900">Register Imaging Study</h3>
          {formError && (
            <p className="mb-2 text-xs text-red-600">{formError}</p>
          )}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {[
              { label: 'Patient NHI *', key: 'patientNhi', placeholder: 'AAA1234' },
              { label: 'Study Instance UID *', key: 'studyInstanceUid', placeholder: '1.2.840.10008...' },
              { label: 'Body Part', key: 'bodyPart', placeholder: 'CHEST' },
              { label: 'Description', key: 'description', placeholder: 'CT Chest w/ Contrast' },
              { label: 'Referring HPI', key: 'referringHpi', placeholder: 'HPI-CPN' },
            ].map(({ label, key, placeholder }) => (
              <div key={key}>
                <label className="mb-1 block text-xs font-medium text-secondary-700">{label}</label>
                <input
                  type="text"
                  placeholder={placeholder}
                  value={form[key as keyof typeof form]}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
                />
              </div>
            ))}
            <div>
              <label className="mb-1 block text-xs font-medium text-secondary-700">Modality *</label>
              <select
                value={form.modality}
                onChange={(e) => setForm((f) => ({ ...f, modality: e.target.value }))}
                className="w-full rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-500 focus:outline-none"
              >
                {MODALITIES.filter(Boolean).map((m) => (
                  <option key={m} value={m}>{m}</option>
                ))}
              </select>
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
              {submitting ? 'Registering…' : 'Register'}
            </button>
          </div>
        </form>
      )}

      {/* Error state */}
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">{error}</div>
      )}

      {/* Study table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
          </div>
        ) : studies.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-secondary-500">No imaging studies found</p>
        ) : (
          <table className="w-full text-sm">
            <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
              <tr>
                <th className="px-4 py-3 text-left">Patient NHI</th>
                <th className="px-4 py-3 text-left">Modality</th>
                <th className="px-4 py-3 text-left">Description</th>
                <th className="px-4 py-3 text-left">Series / Instances</th>
                <th className="px-4 py-3 text-left">Status</th>
                <th className="px-4 py-3 text-left">Study Date</th>
                <th className="px-4 py-3 text-left">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-secondary-100">
              {studies.map((s) => (
                <tr key={s.id} className="hover:bg-secondary-50">
                  <td className="px-4 py-3 font-mono text-xs font-semibold text-secondary-900">{s.patientNhi}</td>
                  <td className="px-4 py-3">
                    <ModalityBadge modality={s.modality} />
                  </td>
                  <td className="px-4 py-3 max-w-[200px] truncate text-secondary-700">
                    {s.description || s.bodyPart || '—'}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-secondary-600">
                    {s.numSeries}S / {s.numInstances}I
                  </td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                      s.status === 'available' ? 'bg-green-100 text-green-700' : 'bg-secondary-100 text-secondary-600'
                    }`}>
                      {s.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-secondary-600">
                    {s.studyDate ? new Date(s.studyDate).toLocaleDateString('en-NZ') : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <a
                      href={`/api/v1/dicom-web/studies/${encodeURIComponent(s.studyInstanceUid)}`}
                      target="_blank"
                      rel="noreferrer"
                      className="text-xs text-primary-600 hover:underline"
                    >
                      WADO-RS
                    </a>
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
