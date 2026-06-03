import React, { FormEvent, useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Provider {
  id: string;
  name: string;
  role: string;
}

interface Appointment {
  id: string;
  patientId: string;
  patientName: string;
  patientNhiDisplay: string;
  providerId: string;
  providerName: string;
  date: string;
  startTime: string;
  endTime: string;
  type: string;
  status: 'booked' | 'arrived' | 'fulfilled' | 'cancelled' | 'noshow';
  notes?: string;
}

interface NewAppointmentForm {
  patientId: string;
  providerId: string;
  date: string;
  startTime: string;
  endTime: string;
  type: string;
  notes: string;
}

const APPOINTMENT_TYPES = [
  'General Consultation',
  'Follow-up',
  'Annual Check-up',
  'Urgent',
  'Mental Health',
  'Cervical Smear',
  'Immunisation',
  'ACC Review',
  'Chronic Condition Review',
  'Telehealth',
  'Other',
];

const STATUS_LABELS: Record<Appointment['status'], string> = {
  booked:    'Booked',
  arrived:   'Arrived',
  fulfilled: 'Completed',
  cancelled: 'Cancelled',
  noshow:    'No Show',
};

const STATUS_BADGE: Record<Appointment['status'], string> = {
  booked:    'badge-info',
  arrived:   'badge-warning',
  fulfilled: 'badge-safe',
  cancelled: 'inline-flex items-center rounded-full bg-secondary-100 px-2.5 py-0.5 text-xs font-medium text-secondary-600',
  noshow:    'badge-urgent',
};

// ---------------------------------------------------------------------------
// Create appointment modal
// ---------------------------------------------------------------------------

interface CreateModalProps {
  providers: Provider[];
  onClose: () => void;
  onCreated: (appt: Appointment) => void;
}

function CreateAppointmentModal({ providers, onClose, onCreated }: CreateModalProps) {
  const api = useApi();
  const [form, setForm] = useState<NewAppointmentForm>({
    patientId: '',
    providerId: providers[0]?.id ?? '',
    date: new Date().toISOString().slice(0, 10),
    startTime: '09:00',
    endTime: '09:15',
    type: 'General Consultation',
    notes: '',
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function update<K extends keyof NewAppointmentForm>(k: K, v: NewAppointmentForm[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const appt = await api.post<Appointment>('/appointments', form);
      onCreated(appt);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create appointment.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="fixed inset-0 bg-black/40" onClick={onClose} />

      {/* Dialog */}
      <div className="relative w-full max-w-lg rounded-xl bg-white shadow-xl ring-1 ring-secondary-200">
        <div className="flex items-center justify-between border-b border-secondary-100 px-6 py-4">
          <h2 className="text-base font-semibold text-secondary-900">New Appointment</h2>
          <button
            onClick={onClose}
            className="rounded p-1 text-secondary-400 hover:text-secondary-700"
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4 p-6">
          {error && (
            <div className="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
              {error}
            </div>
          )}

          <Field label="Patient ID" htmlFor="appt-patient">
            <input
              id="appt-patient"
              type="text"
              required
              value={form.patientId}
              onChange={(e) => update('patientId', e.target.value)}
              placeholder="FHIR Patient resource ID"
              className={inputClass}
            />
          </Field>

          <Field label="Provider" htmlFor="appt-provider">
            <select
              id="appt-provider"
              required
              value={form.providerId}
              onChange={(e) => update('providerId', e.target.value)}
              className={inputClass}
            >
              {providers.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name} — {p.role}
                </option>
              ))}
            </select>
          </Field>

          <Field label="Appointment type" htmlFor="appt-type">
            <select
              id="appt-type"
              value={form.type}
              onChange={(e) => update('type', e.target.value)}
              className={inputClass}
            >
              {APPOINTMENT_TYPES.map((t) => (
                <option key={t}>{t}</option>
              ))}
            </select>
          </Field>

          <div className="grid grid-cols-3 gap-3">
            <Field label="Date" htmlFor="appt-date">
              <input
                id="appt-date"
                type="date"
                required
                value={form.date}
                onChange={(e) => update('date', e.target.value)}
                className={inputClass}
              />
            </Field>
            <Field label="Start" htmlFor="appt-start">
              <input
                id="appt-start"
                type="time"
                required
                value={form.startTime}
                onChange={(e) => update('startTime', e.target.value)}
                className={inputClass}
              />
            </Field>
            <Field label="End" htmlFor="appt-end">
              <input
                id="appt-end"
                type="time"
                required
                value={form.endTime}
                onChange={(e) => update('endTime', e.target.value)}
                className={inputClass}
              />
            </Field>
          </div>

          <Field label="Notes (optional)" htmlFor="appt-notes">
            <textarea
              id="appt-notes"
              rows={3}
              value={form.notes}
              onChange={(e) => update('notes', e.target.value)}
              placeholder="Reason for visit, special requirements…"
              className={inputClass}
            />
          </Field>

          <div className="flex justify-end gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-100"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="rounded-md bg-primary-600 px-4 py-2 text-sm font-semibold text-white hover:bg-primary-700 disabled:opacity-50"
            >
              {saving ? 'Saving…' : 'Create Appointment'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

const inputClass =
  'mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500';

function Field({ label, htmlFor, children }: { label: string; htmlFor: string; children: React.ReactNode }) {
  return (
    <div>
      <label htmlFor={htmlFor} className="block text-sm font-medium text-secondary-700">
        {label}
      </label>
      {children}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function AppointmentsPage() {
  const api = useApi();

  const [selectedDate, setSelectedDate] = useState<string>(
    new Date().toISOString().slice(0, 10),
  );
  const [selectedProvider, setSelectedProvider] = useState<string>('');
  const [providers, setProviders] = useState<Provider[]>([]);
  const [appointments, setAppointments] = useState<Appointment[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showModal, setShowModal] = useState(false);

  // Load providers on mount
  useEffect(() => {
    void api.get<{ providers: Provider[] }>('/practitioners').then((d) => {
      setProviders(d.providers);
    }).catch(() => undefined);
  }, [api]);

  // Load appointments whenever filter changes
  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const params: Record<string, string> = { date: selectedDate };
    if (selectedProvider) params['providerId'] = selectedProvider;

    void api.get<{ appointments: Appointment[] }>('/appointments', { params })
      .then((d) => { if (!cancelled) setAppointments(d.appointments); })
      .catch(() => { if (!cancelled) setError('Failed to load appointments.'); })
      .finally(() => { if (!cancelled) setLoading(false); });

    return () => { cancelled = true; };
  }, [api, selectedDate, selectedProvider]);

  function handleCreated(appt: Appointment) {
    setAppointments((prev) => [...prev, appt].sort((a, b) => a.startTime.localeCompare(b.startTime)));
  }

  const displayDate = new Date(selectedDate + 'T00:00:00').toLocaleDateString('en-NZ', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });

  return (
    <AppShell title="Appointments">
      {/* Filter bar */}
      <div className="mb-6 flex flex-wrap items-end gap-3">
        <div>
          <label htmlFor="appt-date-filter" className="block text-sm font-medium text-secondary-700">
            Date
          </label>
          <input
            id="appt-date-filter"
            type="date"
            value={selectedDate}
            onChange={(e) => setSelectedDate(e.target.value)}
            className="mt-1 rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          />
        </div>

        <div>
          <label htmlFor="provider-filter" className="block text-sm font-medium text-secondary-700">
            Provider
          </label>
          <select
            id="provider-filter"
            value={selectedProvider}
            onChange={(e) => setSelectedProvider(e.target.value)}
            className="mt-1 rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          >
            <option value="">All providers</option>
            {providers.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} — {p.role}
              </option>
            ))}
          </select>
        </div>

        <button
          onClick={() => setShowModal(true)}
          className="ml-auto flex items-center gap-2 rounded-md bg-primary-600 px-4 py-2 text-sm font-semibold text-white hover:bg-primary-700"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
          </svg>
          New Appointment
        </button>
      </div>

      {/* Heading */}
      <h2 className="mb-4 text-base font-semibold text-secondary-700">{displayDate}</h2>

      {/* Error */}
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {/* Appointment table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="h-7 w-7 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
          </div>
        ) : appointments.length === 0 ? (
          <p className="px-4 py-10 text-center text-sm text-secondary-500">
            No appointments for this date and provider.
          </p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                <tr>
                  <th className="px-4 py-3 text-left">Time</th>
                  <th className="px-4 py-3 text-left">Patient</th>
                  <th className="px-4 py-3 text-left">Provider</th>
                  <th className="px-4 py-3 text-left">Type</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {appointments.map((appt) => (
                  <tr key={appt.id} className="hover:bg-secondary-50">
                    <td className="whitespace-nowrap px-4 py-3 font-mono text-secondary-700">
                      {appt.startTime} – {appt.endTime}
                    </td>
                    <td className="px-4 py-3">
                      <Link
                        to={`/patients/${appt.patientId}`}
                        className="font-medium text-secondary-900 hover:text-primary-600 hover:underline"
                      >
                        {appt.patientName}
                      </Link>
                      <div className="font-mono text-xs text-secondary-400">{appt.patientNhiDisplay}</div>
                    </td>
                    <td className="px-4 py-3 text-secondary-600">{appt.providerName}</td>
                    <td className="px-4 py-3 text-secondary-700">{appt.type}</td>
                    <td className="px-4 py-3">
                      <span className={STATUS_BADGE[appt.status]}>
                        {STATUS_LABELS[appt.status]}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <Link
                        to={`/patients/${appt.patientId}`}
                        className="font-medium text-primary-600 hover:underline"
                      >
                        Patient
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Modal */}
      {showModal && (
        <CreateAppointmentModal
          providers={providers}
          onClose={() => setShowModal(false)}
          onCreated={handleCreated}
        />
      )}
    </AppShell>
  );
}
