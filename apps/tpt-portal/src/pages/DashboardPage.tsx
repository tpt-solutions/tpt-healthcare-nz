import { useAuth } from '../contexts/AuthContext';

interface UpcomingAppointment {
  id: string;
  date: string;
  time: string;
  practitioner: string;
  type: string;
  location: string;
}

interface TestResult {
  id: string;
  name: string;
  date: string;
  status: 'normal' | 'abnormal' | 'pending';
  summary: string;
}

interface ActiveMedication {
  id: string;
  name: string;
  dose: string;
  frequency: string;
  prescriber: string;
}

// Stub data — replaced by @tpt/api-client calls once backend is connected
const upcomingAppointments: UpcomingAppointment[] = [
  {
    id: 'appt-1',
    date: '2026-06-10',
    time: '09:30',
    practitioner: 'Dr. Hemi Walker',
    type: 'General Practice',
    location: 'Auckland City Medical Centre',
  },
  {
    id: 'appt-2',
    date: '2026-06-18',
    time: '14:00',
    practitioner: 'Dr. Piripi Te Aho',
    type: 'Annual Review',
    location: 'Auckland City Medical Centre',
  },
];

const recentResults: TestResult[] = [
  {
    id: 'res-1',
    name: 'Full Blood Count',
    date: '2026-05-20',
    status: 'normal',
    summary: 'All values within normal range.',
  },
  {
    id: 'res-2',
    name: 'HbA1c',
    date: '2026-05-20',
    status: 'abnormal',
    summary: '52 mmol/mol — slightly elevated. Follow-up recommended.',
  },
  {
    id: 'res-3',
    name: 'Urine Dipstick',
    date: '2026-04-15',
    status: 'normal',
    summary: 'No significant findings.',
  },
];

const activeMedications: ActiveMedication[] = [
  {
    id: 'med-1',
    name: 'Metformin 500 mg',
    dose: '500 mg',
    frequency: 'Twice daily with meals',
    prescriber: 'Dr. Hemi Walker',
  },
  {
    id: 'med-2',
    name: 'Lisinopril 10 mg',
    dose: '10 mg',
    frequency: 'Once daily',
    prescriber: 'Dr. Hemi Walker',
  },
];

function statusBadge(status: TestResult['status']) {
  const styles = {
    normal: 'bg-green-100 text-green-700',
    abnormal: 'bg-amber-100 text-amber-700',
    pending: 'bg-gray-100 text-gray-600',
  };
  const labels = { normal: 'Normal', abnormal: 'Review', pending: 'Pending' };
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${styles[status]}`}>
      {labels[status]}
    </span>
  );
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-NZ', {
    weekday: 'short',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

export function DashboardPage() {
  const { user } = useAuth();

  return (
    <div className="p-6 max-w-5xl mx-auto">
      {/* Greeting */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">
          Kia ora, {user?.givenName}
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Here's your health overview for today, {formatDate(new Date().toISOString().slice(0, 10))}.
        </p>
      </div>

      {/* Quick stats */}
      <div className="grid grid-cols-3 gap-4 mb-8">
        <div className="bg-brand-50 rounded-xl p-4 border border-brand-100">
          <p className="text-xs font-medium text-brand-600 uppercase tracking-wide">Next Appointment</p>
          <p className="mt-1 text-lg font-semibold text-gray-900">
            {formatDate(upcomingAppointments[0].date)}
          </p>
          <p className="text-sm text-gray-500">{upcomingAppointments[0].time} — {upcomingAppointments[0].practitioner}</p>
        </div>
        <div className="bg-green-50 rounded-xl p-4 border border-green-100">
          <p className="text-xs font-medium text-green-600 uppercase tracking-wide">Active Medications</p>
          <p className="mt-1 text-3xl font-bold text-gray-900">{activeMedications.length}</p>
          <p className="text-sm text-gray-500">current prescriptions</p>
        </div>
        <div className="bg-amber-50 rounded-xl p-4 border border-amber-100">
          <p className="text-xs font-medium text-amber-600 uppercase tracking-wide">Results to Review</p>
          <p className="mt-1 text-3xl font-bold text-gray-900">
            {recentResults.filter(r => r.status === 'abnormal').length}
          </p>
          <p className="text-sm text-gray-500">need your attention</p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-6">
        {/* Upcoming appointments */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm">
          <div className="px-5 py-4 border-b border-gray-100 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-gray-900">Upcoming Appointments</h2>
            <a href="/appointments" className="text-xs text-brand-600 hover:underline">View all</a>
          </div>
          <ul className="divide-y divide-gray-100">
            {upcomingAppointments.map(appt => (
              <li key={appt.id} className="px-5 py-4">
                <div className="flex items-start justify-between">
                  <div>
                    <p className="text-sm font-medium text-gray-900">{appt.practitioner}</p>
                    <p className="text-xs text-gray-500 mt-0.5">{appt.type}</p>
                    <p className="text-xs text-gray-400 mt-0.5">{appt.location}</p>
                  </div>
                  <div className="text-right flex-shrink-0 ml-4">
                    <p className="text-xs font-medium text-brand-600">{formatDate(appt.date)}</p>
                    <p className="text-xs text-gray-500">{appt.time}</p>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </section>

        {/* Recent test results */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm">
          <div className="px-5 py-4 border-b border-gray-100 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-gray-900">Recent Test Results</h2>
            <a href="/records" className="text-xs text-brand-600 hover:underline">View all</a>
          </div>
          <ul className="divide-y divide-gray-100">
            {recentResults.map(result => (
              <li key={result.id} className="px-5 py-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-gray-900">{result.name}</p>
                    <p className="text-xs text-gray-500 mt-0.5 line-clamp-1">{result.summary}</p>
                  </div>
                  <div className="flex-shrink-0 flex flex-col items-end gap-1">
                    {statusBadge(result.status)}
                    <p className="text-xs text-gray-400">{formatDate(result.date)}</p>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </section>

        {/* Active medications */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm col-span-2">
          <div className="px-5 py-4 border-b border-gray-100 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-gray-900">Active Medications</h2>
            <a href="/prescriptions" className="text-xs text-brand-600 hover:underline">View all prescriptions</a>
          </div>
          <div className="grid grid-cols-2 gap-px bg-gray-100 rounded-b-xl overflow-hidden">
            {activeMedications.map(med => (
              <div key={med.id} className="bg-white px-5 py-4">
                <p className="text-sm font-semibold text-gray-900">{med.name}</p>
                <p className="text-xs text-gray-500 mt-0.5">{med.frequency}</p>
                <p className="text-xs text-gray-400 mt-0.5">Prescribed by {med.prescriber}</p>
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}
