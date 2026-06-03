import { useState } from 'react';

interface Appointment {
  id: string;
  date: string;
  time: string;
  practitioner: string;
  type: string;
  location: string;
  status: 'upcoming' | 'past' | 'cancelled';
  notes?: string;
}

// Stub data — replaced by @tpt/api-client FHIR Appointment calls
const allAppointments: Appointment[] = [
  {
    id: 'appt-1',
    date: '2026-06-10',
    time: '09:30',
    practitioner: 'Dr. Hemi Walker',
    type: 'General Practice',
    location: 'Auckland City Medical Centre',
    status: 'upcoming',
  },
  {
    id: 'appt-2',
    date: '2026-06-18',
    time: '14:00',
    practitioner: 'Dr. Piripi Te Aho',
    type: 'Annual Review',
    location: 'Auckland City Medical Centre',
    status: 'upcoming',
  },
  {
    id: 'appt-3',
    date: '2026-05-05',
    time: '10:15',
    practitioner: 'Dr. Hemi Walker',
    type: 'Follow-up',
    location: 'Auckland City Medical Centre',
    status: 'past',
    notes: 'HbA1c review. Metformin dose maintained.',
  },
  {
    id: 'appt-4',
    date: '2026-04-12',
    time: '08:45',
    practitioner: 'Dr. Hemi Walker',
    type: 'General Practice',
    location: 'Auckland City Medical Centre',
    status: 'past',
    notes: 'Blood pressure check. Annual bloods ordered.',
  },
  {
    id: 'appt-5',
    date: '2026-03-28',
    time: '11:00',
    practitioner: 'Dr. Sione Tuilagi',
    type: 'Specialist Referral',
    location: 'Auckland Specialist Clinic',
    status: 'cancelled',
  },
];

// Cancellation window: patients may cancel up to 24 hours before the appointment
const CANCEL_WINDOW_HOURS = 24;

function canCancel(appt: Appointment): boolean {
  if (appt.status !== 'upcoming') return false;
  const apptTime = new Date(`${appt.date}T${appt.time}:00`);
  const now = new Date();
  const hoursUntil = (apptTime.getTime() - now.getTime()) / (1000 * 60 * 60);
  return hoursUntil >= CANCEL_WINDOW_HOURS;
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-NZ', {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

function statusPill(status: Appointment['status']) {
  const map = {
    upcoming: 'bg-brand-100 text-brand-700',
    past: 'bg-gray-100 text-gray-600',
    cancelled: 'bg-red-100 text-red-600',
  };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${map[status]}`}>
      {status}
    </span>
  );
}

export function AppointmentsPage() {
  const [tab, setTab] = useState<'upcoming' | 'past'>('upcoming');
  const [showRequestModal, setShowRequestModal] = useState(false);
  const [cancellingId, setCancellingId] = useState<string | null>(null);
  const [appointments, setAppointments] = useState(allAppointments);

  const visible = appointments.filter(a =>
    tab === 'upcoming' ? a.status === 'upcoming' : a.status === 'past' || a.status === 'cancelled',
  );

  const handleCancel = (id: string) => {
    // TODO: call DELETE /api/v1/fhir/Appointment/{id} via @tpt/api-client
    setAppointments(prev =>
      prev.map(a => (a.id === id ? { ...a, status: 'cancelled' as const } : a)),
    );
    setCancellingId(null);
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">My Appointments</h1>
          <p className="mt-1 text-sm text-gray-500">
            View and manage your appointments at TPT Healthcare.
          </p>
        </div>
        <button
          onClick={() => setShowRequestModal(true)}
          className="inline-flex items-center gap-2 rounded-lg bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-700 transition-colors"
        >
          <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          Request Appointment
        </button>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-gray-100 rounded-lg p-1 w-fit mb-6">
        {(['upcoming', 'past'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors capitalize ${
              tab === t ? 'bg-white shadow-sm text-gray-900' : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {/* Appointments list */}
      {visible.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 py-16 text-center">
          <p className="text-sm text-gray-500">No {tab} appointments.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {visible.map(appt => (
            <div key={appt.id} className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-3 mb-2">
                    <h3 className="text-sm font-semibold text-gray-900">{appt.practitioner}</h3>
                    {statusPill(appt.status)}
                  </div>
                  <p className="text-sm text-gray-600">{appt.type}</p>
                  <p className="text-xs text-gray-400 mt-1">{appt.location}</p>
                  {appt.notes && (
                    <p className="mt-2 text-xs text-gray-500 bg-gray-50 rounded-lg px-3 py-2">
                      {appt.notes}
                    </p>
                  )}
                </div>
                <div className="ml-6 text-right flex-shrink-0">
                  <p className="text-sm font-medium text-brand-700">{formatDate(appt.date)}</p>
                  <p className="text-sm text-gray-500">{appt.time}</p>
                  {canCancel(appt) && (
                    <button
                      onClick={() => setCancellingId(appt.id)}
                      className="mt-3 text-xs text-red-500 hover:text-red-700 underline"
                    >
                      Cancel appointment
                    </button>
                  )}
                  {appt.status === 'upcoming' && !canCancel(appt) && (
                    <p className="mt-3 text-xs text-gray-400">
                      Within {CANCEL_WINDOW_HOURS}h — call to cancel
                    </p>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Confirm cancel modal */}
      {cancellingId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white rounded-2xl shadow-xl p-6 w-full max-w-sm mx-4">
            <h2 className="text-base font-semibold text-gray-900 mb-2">Cancel appointment?</h2>
            <p className="text-sm text-gray-500 mb-6">
              Are you sure you want to cancel this appointment? This cannot be undone.
            </p>
            <div className="flex gap-3">
              <button
                onClick={() => setCancellingId(null)}
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
              >
                Keep appointment
              </button>
              <button
                onClick={() => handleCancel(cancellingId)}
                className="flex-1 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 transition-colors"
              >
                Yes, cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Request appointment modal */}
      {showRequestModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-white rounded-2xl shadow-xl p-6 w-full max-w-md mx-4">
            <h2 className="text-base font-semibold text-gray-900 mb-4">Request New Appointment</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Reason for visit</label>
                <textarea
                  rows={3}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/20"
                  placeholder="Brief description of your concern..."
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Preferred date (optional)</label>
                <input
                  type="date"
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/20"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-gray-700 mb-1">Appointment type</label>
                <select className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/20">
                  <option>In-person</option>
                  <option>Telehealth (phone)</option>
                  <option>Telehealth (video)</option>
                </select>
              </div>
            </div>
            <p className="mt-4 text-xs text-gray-400">
              Our team will contact you within one business day to confirm your appointment time.
            </p>
            <div className="flex gap-3 mt-5">
              <button
                onClick={() => setShowRequestModal(false)}
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => setShowRequestModal(false)}
                className="flex-1 rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 transition-colors"
              >
                Send request
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
