import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'plans' | 'claims' | 'sessions' | 'dashboard';
type ToastSeverity = 'success' | 'error' | 'info' | 'warning';

interface TreatmentPlan {
  id: string;
  patientNHI: string;
  patientName: string;
  clinician: string;
  profession: 'physiotherapy' | 'occupational_therapy' | 'speech_language_therapy' | 'podiatry';
  diagnosis: string;
  status: 'draft' | 'active' | 'under_review' | 'completed' | 'discontinued' | 'on_hold';
  startDate: string;
  reviewDate: string;
  accNumber?: string;
  sessionsUsed: number;
  sessionsApproved: number;
}

interface ACCClaim {
  id: string;
  patientNHI: string;
  patientName: string;
  claimType: string;
  accNumber: string;
  status: 'draft' | 'submitted' | 'accepted' | 'declined' | 'under_review' | 'closed' | 'expired';
  diagnosis: string;
  bodyRegion: string;
  approvedSessions: number;
  usedSessions: number;
  startDate: string;
  expiryDate: string;
}

interface SessionNote {
  id: string;
  patientNHI: string;
  patientName: string;
  clinician: string;
  profession: string;
  sessionDate: string;
  sessionNumber: number;
  durationMinutes: number;
  chargeCode: string;
  status: 'planned' | 'active' | 'completed' | 'cancelled';
}

const professionClasses: Record<string, string> = {
  physiotherapy: 'bg-blue-50 text-blue-700 border border-blue-200',
  occupational_therapy: 'bg-green-50 text-green-700 border border-green-200',
  speech_language_therapy: 'bg-orange-50 text-orange-700 border border-orange-200',
  podiatry: 'bg-purple-50 text-purple-700 border border-purple-200',
};

const professionProgressColor: Record<string, string> = {
  physiotherapy: 'bg-blue-500',
  occupational_therapy: 'bg-green-500',
  speech_language_therapy: 'bg-orange-500',
  podiatry: 'bg-purple-500',
};

const professionLabels: Record<string, string> = {
  physiotherapy: 'Physiotherapy',
  occupational_therapy: 'Occupational Therapy',
  speech_language_therapy: 'Speech-Language Therapy',
  podiatry: 'Podiatry',
};

const statusClasses: Record<string, string> = {
  draft: 'bg-secondary-100 text-secondary-600',
  active: 'bg-blue-100 text-blue-800',
  under_review: 'bg-amber-100 text-amber-800',
  completed: 'bg-green-100 text-green-800',
  discontinued: 'bg-red-100 text-red-800',
  on_hold: 'bg-sky-100 text-sky-800',
  submitted: 'bg-blue-100 text-blue-800',
  accepted: 'bg-green-100 text-green-800',
  declined: 'bg-red-100 text-red-800',
  closed: 'bg-secondary-100 text-secondary-600',
  expired: 'bg-red-100 text-red-800',
  planned: 'bg-secondary-100 text-secondary-600',
  cancelled: 'bg-red-100 text-red-800',
};

const mockTreatmentPlans: TreatmentPlan[] = [
  {
    id: 'tp-001',
    patientNHI: 'ABC1234',
    patientName: 'John Smith',
    clinician: 'Dr. Sarah Wilson',
    profession: 'physiotherapy',
    diagnosis: 'Lumbar disc herniation L4/L5',
    status: 'active',
    startDate: '2024-01-15',
    reviewDate: '2024-03-15',
    accNumber: 'ACC123456',
    sessionsUsed: 6,
    sessionsApproved: 12,
  },
  {
    id: 'tp-002',
    patientNHI: 'DEF5678',
    patientName: 'Mary Johnson',
    clinician: 'Emma Thompson',
    profession: 'occupational_therapy',
    diagnosis: 'Stroke rehabilitation - right hemiplegia',
    status: 'active',
    startDate: '2024-02-01',
    reviewDate: '2024-04-01',
    accNumber: 'ACC789012',
    sessionsUsed: 8,
    sessionsApproved: 20,
  },
  {
    id: 'tp-003',
    patientNHI: 'GHI9012',
    patientName: 'Robert Brown',
    clinician: 'Dr. James Chen',
    profession: 'speech_language_therapy',
    diagnosis: 'Aphasia post-stroke',
    status: 'under_review',
    startDate: '2024-01-20',
    reviewDate: '2024-03-20',
    accNumber: 'ACC345678',
    sessionsUsed: 10,
    sessionsApproved: 15,
  },
  {
    id: 'tp-004',
    patientNHI: 'JKL3456',
    patientName: 'Susan Davis',
    clinician: 'Lisa Anderson',
    profession: 'podiatry',
    diagnosis: 'Diabetic foot ulcer - plantar forefoot',
    status: 'active',
    startDate: '2024-02-10',
    reviewDate: '2024-03-10',
    accNumber: 'ACC901234',
    sessionsUsed: 4,
    sessionsApproved: 10,
  },
];

const mockACCClaims: ACCClaim[] = [
  {
    id: 'acc-001',
    patientNHI: 'ABC1234',
    patientName: 'John Smith',
    claimType: 'physiotherapy',
    accNumber: 'ACC123456',
    status: 'accepted',
    diagnosis: 'Lumbar disc herniation L4/L5',
    bodyRegion: 'lumbar_spine',
    approvedSessions: 12,
    usedSessions: 6,
    startDate: '2024-01-15',
    expiryDate: '2024-07-15',
  },
  {
    id: 'acc-002',
    patientNHI: 'DEF5678',
    patientName: 'Mary Johnson',
    claimType: 'occupational_therapy',
    accNumber: 'ACC789012',
    status: 'accepted',
    diagnosis: 'Stroke rehabilitation - right hemiplegia',
    bodyRegion: 'upper_limb',
    approvedSessions: 20,
    usedSessions: 8,
    startDate: '2024-02-01',
    expiryDate: '2024-08-01',
  },
  {
    id: 'acc-003',
    patientNHI: 'GHI9012',
    patientName: 'Robert Brown',
    claimType: 'speech_language_therapy',
    accNumber: 'ACC345678',
    status: 'under_review',
    diagnosis: 'Aphasia post-stroke',
    bodyRegion: 'cognitive_communication',
    approvedSessions: 15,
    usedSessions: 10,
    startDate: '2024-01-20',
    expiryDate: '2024-07-20',
  },
  {
    id: 'acc-004',
    patientNHI: 'JKL3456',
    patientName: 'Susan Davis',
    claimType: 'podiatry',
    accNumber: 'ACC901234',
    status: 'accepted',
    diagnosis: 'Diabetic foot ulcer - plantar forefoot',
    bodyRegion: 'foot',
    approvedSessions: 10,
    usedSessions: 4,
    startDate: '2024-02-10',
    expiryDate: '2024-08-10',
  },
];

const mockSessionNotes: SessionNote[] = [
  {
    id: 'sn-001',
    patientNHI: 'ABC1234',
    patientName: 'John Smith',
    clinician: 'Dr. Sarah Wilson',
    profession: 'physiotherapy',
    sessionDate: '2024-02-20',
    sessionNumber: 6,
    durationMinutes: 30,
    chargeCode: 'PHY002',
    status: 'completed',
  },
  {
    id: 'sn-002',
    patientNHI: 'DEF5678',
    patientName: 'Mary Johnson',
    clinician: 'Emma Thompson',
    profession: 'occupational_therapy',
    sessionDate: '2024-02-21',
    sessionNumber: 8,
    durationMinutes: 45,
    chargeCode: 'OT002',
    status: 'completed',
  },
  {
    id: 'sn-003',
    patientNHI: 'GHI9012',
    patientName: 'Robert Brown',
    clinician: 'Dr. James Chen',
    profession: 'speech_language_therapy',
    sessionDate: '2024-02-22',
    sessionNumber: 10,
    durationMinutes: 45,
    chargeCode: 'SLT002',
    status: 'completed',
  },
  {
    id: 'sn-004',
    patientNHI: 'JKL3456',
    patientName: 'Susan Davis',
    clinician: 'Lisa Anderson',
    profession: 'podiatry',
    sessionDate: '2024-02-23',
    sessionNumber: 4,
    durationMinutes: 30,
    chargeCode: 'POD003',
    status: 'completed',
  },
];

function ProfessionBadge({ profession }: { profession: string }) {
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${professionClasses[profession] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {professionLabels[profession] ?? profession}
    </span>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${statusClasses[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

function ProgressBar({ used, approved, profession }: { used: number; approved: number; profession: string }) {
  const pct = approved > 0 ? Math.min((used / approved) * 100, 100) : 0;
  const barColor = used >= approved ? 'bg-red-500' : (professionProgressColor[profession] ?? 'bg-primary-500');
  return (
    <div>
      <span className="text-sm">{used} / {approved}</span>
      <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-secondary-200">
        <div className={`h-full rounded-full ${barColor}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: 'plans', label: 'Treatment Plans', count: mockTreatmentPlans.length },
    { id: 'claims', label: 'ACC Claims', count: mockACCClaims.length },
    { id: 'sessions', label: 'Session Notes', count: mockSessionNotes.length },
    { id: 'dashboard', label: 'Dashboard', count: 0 },
  ];
  return (
    <div className="mb-6 flex gap-1 overflow-x-auto rounded-lg bg-secondary-100 p-1">
      {tabs.map(t => (
        <button
          key={t.id}
          onClick={() => onSelect(t.id)}
          className={`flex-shrink-0 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === t.id
              ? 'bg-white text-primary-700 shadow-sm'
              : 'text-secondary-600 hover:text-secondary-900'
          }`}
        >
          {t.label}{t.count > 0 ? ` (${t.count})` : ''}
        </button>
      ))}
    </div>
  );
}

const toastBg: Record<ToastSeverity, string> = {
  success: 'bg-green-600',
  error: 'bg-red-600',
  info: 'bg-blue-600',
  warning: 'bg-amber-500',
};

function Toast({ message, severity, onClose }: { message: string; severity: ToastSeverity; onClose: () => void }) {
  useEffect(() => {
    const t = setTimeout(onClose, 5000);
    return () => clearTimeout(t);
  }, [message, onClose]);
  return (
    <div className={`fixed bottom-4 right-4 z-50 flex items-center gap-3 rounded-lg px-4 py-3 text-sm text-white shadow-lg ${toastBg[severity]}`}>
      <span>{message}</span>
      <button onClick={onClose} className="ml-2 font-bold opacity-75 hover:opacity-100">×</button>
    </div>
  );
}

export default function AlliedHealthPage() {
  const [activeTab, setActiveTab] = useState<Tab>('plans');
  const [searchTerm, setSearchTerm] = useState('');
  const [professionFilter, setProfessionFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [toast, setToast] = useState<{ message: string; severity: ToastSeverity } | null>(null);
  const [selectedPlan, setSelectedPlan] = useState<TreatmentPlan | null>(null);

  const showToast = (message: string, severity: ToastSeverity = 'info') => setToast({ message, severity });

  const filteredPlans = mockTreatmentPlans.filter(p => {
    const q = searchTerm.toLowerCase();
    return (
      (p.patientName.toLowerCase().includes(q) || p.patientNHI.toLowerCase().includes(q) || p.diagnosis.toLowerCase().includes(q)) &&
      (professionFilter === 'all' || p.profession === professionFilter) &&
      (statusFilter === 'all' || p.status === statusFilter)
    );
  });

  const filteredClaims = mockACCClaims.filter(c => {
    const q = searchTerm.toLowerCase();
    return (
      (c.patientName.toLowerCase().includes(q) || c.patientNHI.toLowerCase().includes(q) || c.accNumber.toLowerCase().includes(q)) &&
      (professionFilter === 'all' || c.claimType === professionFilter) &&
      (statusFilter === 'all' || c.status === statusFilter)
    );
  });

  const filteredSessions = mockSessionNotes.filter(s => {
    const q = searchTerm.toLowerCase();
    return (
      (s.patientName.toLowerCase().includes(q) || s.patientNHI.toLowerCase().includes(q) || s.clinician.toLowerCase().includes(q)) &&
      (professionFilter === 'all' || s.profession === professionFilter) &&
      (statusFilter === 'all' || s.status === statusFilter)
    );
  });

  return (
    <AppShell title="Allied Health">
      {/* Summary stat cards */}
      <div className="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
        {(['physiotherapy', 'occupational_therapy', 'speech_language_therapy', 'podiatry'] as const).map(prof => (
          <div key={prof} className={`rounded-xl border p-4 ${professionClasses[prof] ?? ''}`}>
            <p className="text-2xl font-bold">{mockTreatmentPlans.filter(p => p.profession === prof).length}</p>
            <p className="mt-1 text-xs font-medium">{professionLabels[prof]}</p>
          </div>
        ))}
      </div>

      {/* Filters */}
      <div className="mb-4 flex flex-wrap gap-3 rounded-xl bg-white p-3 shadow-sm ring-1 ring-secondary-200">
        <div className="relative flex-1 min-w-[200px]">
          <svg className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-secondary-400" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-4.35-4.35M17 11A6 6 0 1 1 5 11a6 6 0 0 1 12 0z" />
          </svg>
          <input
            type="search"
            placeholder="Search patients, NHI, diagnosis…"
            value={searchTerm}
            onChange={e => setSearchTerm(e.target.value)}
            className="w-full rounded-md border border-secondary-300 py-1.5 pl-9 pr-3 text-sm focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400"
          />
        </div>
        <select
          value={professionFilter}
          onChange={e => setProfessionFilter(e.target.value)}
          className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400"
        >
          <option value="all">All Professions</option>
          <option value="physiotherapy">Physiotherapy</option>
          <option value="occupational_therapy">Occupational Therapy</option>
          <option value="speech_language_therapy">Speech-Language Therapy</option>
          <option value="podiatry">Podiatry</option>
        </select>
        <select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
          className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400"
        >
          <option value="all">All Statuses</option>
          <option value="draft">Draft</option>
          <option value="active">Active</option>
          <option value="under_review">Under Review</option>
          <option value="completed">Completed</option>
          <option value="discontinued">Discontinued</option>
          <option value="on_hold">On Hold</option>
          <option value="submitted">Submitted</option>
          <option value="accepted">Accepted</option>
          <option value="declined">Declined</option>
          <option value="closed">Closed</option>
          <option value="expired">Expired</option>
          <option value="planned">Planned</option>
          <option value="cancelled">Cancelled</option>
        </select>
        <button
          onClick={() => showToast('Data refreshed', 'success')}
          className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm font-medium text-secondary-700 hover:bg-secondary-50"
        >
          Refresh
        </button>
      </div>

      <TabBar active={activeTab} onSelect={setActiveTab} />

      {/* Treatment Plans */}
      {activeTab === 'plans' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Treatment Plans</h2>
            <button
              onClick={() => showToast('Navigate to profession-specific page to create new treatment plan', 'info')}
              className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
            >
              + New Plan
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Profession</th>
                  <th className="px-6 py-3 text-left">Clinician</th>
                  <th className="px-6 py-3 text-left">Diagnosis</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">ACC Claim</th>
                  <th className="px-6 py-3 text-left">Sessions</th>
                  <th className="px-6 py-3 text-left">Review</th>
                  <th className="px-6 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {filteredPlans.map(plan => (
                  <tr
                    key={plan.id}
                    className="cursor-pointer hover:bg-secondary-50"
                    onClick={() => setSelectedPlan(plan)}
                  >
                    <td className="px-6 py-3">
                      <p className="font-medium text-secondary-900">{plan.patientName}</p>
                      <p className="font-mono text-xs text-secondary-500">NHI: {plan.patientNHI}</p>
                    </td>
                    <td className="px-6 py-3"><ProfessionBadge profession={plan.profession} /></td>
                    <td className="px-6 py-3 text-secondary-700">{plan.clinician}</td>
                    <td className="max-w-[200px] truncate px-6 py-3 text-secondary-700">{plan.diagnosis}</td>
                    <td className="px-6 py-3"><StatusBadge status={plan.status} /></td>
                    <td className="px-6 py-3 font-mono text-xs text-secondary-600">{plan.accNumber ?? '—'}</td>
                    <td className="px-6 py-3">
                      <ProgressBar used={plan.sessionsUsed} approved={plan.sessionsApproved} profession={plan.profession} />
                    </td>
                    <td className="px-6 py-3 text-secondary-500">{plan.reviewDate}</td>
                    <td className="px-6 py-3 text-right">
                      <button
                        onClick={e => { e.stopPropagation(); setSelectedPlan(plan); }}
                        className="rounded p-1 text-secondary-400 hover:text-primary-600"
                        aria-label="View"
                      >
                        <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                          <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                        </svg>
                      </button>
                    </td>
                  </tr>
                ))}
                {filteredPlans.length === 0 && (
                  <tr>
                    <td colSpan={9} className="px-6 py-8 text-center text-sm text-secondary-400">No treatment plans found</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* ACC Claims */}
      {activeTab === 'claims' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">ACC Claims</h2>
            <button
              onClick={() => showToast('Navigate to ACC Claims to create new claim', 'info')}
              className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
            >
              + New Claim
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Claim Type</th>
                  <th className="px-6 py-3 text-left">ACC Number</th>
                  <th className="px-6 py-3 text-left">Diagnosis</th>
                  <th className="px-6 py-3 text-left">Body Region</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">Sessions</th>
                  <th className="px-6 py-3 text-left">Expiry</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {filteredClaims.map(claim => (
                  <tr key={claim.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3">
                      <p className="font-medium text-secondary-900">{claim.patientName}</p>
                      <p className="font-mono text-xs text-secondary-500">NHI: {claim.patientNHI}</p>
                    </td>
                    <td className="px-6 py-3"><ProfessionBadge profession={claim.claimType} /></td>
                    <td className="px-6 py-3 font-mono text-xs font-medium text-secondary-700">{claim.accNumber}</td>
                    <td className="max-w-[200px] truncate px-6 py-3 text-secondary-700">{claim.diagnosis}</td>
                    <td className="px-6 py-3 text-secondary-600">{claim.bodyRegion.replace(/_/g, ' ')}</td>
                    <td className="px-6 py-3"><StatusBadge status={claim.status} /></td>
                    <td className="px-6 py-3">
                      <ProgressBar used={claim.usedSessions} approved={claim.approvedSessions} profession={claim.claimType} />
                    </td>
                    <td className="px-6 py-3 text-secondary-500">{claim.expiryDate}</td>
                  </tr>
                ))}
                {filteredClaims.length === 0 && (
                  <tr>
                    <td colSpan={8} className="px-6 py-8 text-center text-sm text-secondary-400">No ACC claims found</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Session Notes */}
      {activeTab === 'sessions' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Session Notes</h2>
            <button
              onClick={() => showToast('Navigate to profession-specific page to create new session note', 'info')}
              className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
            >
              + New Session
            </button>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Profession</th>
                  <th className="px-6 py-3 text-left">Clinician</th>
                  <th className="px-6 py-3 text-left">Date</th>
                  <th className="px-6 py-3 text-left">Session #</th>
                  <th className="px-6 py-3 text-left">Duration</th>
                  <th className="px-6 py-3 text-left">Charge Code</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {filteredSessions.map(session => (
                  <tr key={session.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3">
                      <p className="font-medium text-secondary-900">{session.patientName}</p>
                      <p className="font-mono text-xs text-secondary-500">NHI: {session.patientNHI}</p>
                    </td>
                    <td className="px-6 py-3"><ProfessionBadge profession={session.profession} /></td>
                    <td className="px-6 py-3 text-secondary-700">{session.clinician}</td>
                    <td className="px-6 py-3 text-secondary-500">{session.sessionDate}</td>
                    <td className="px-6 py-3 text-secondary-700">{session.sessionNumber}</td>
                    <td className="px-6 py-3 text-secondary-700">{session.durationMinutes} min</td>
                    <td className="px-6 py-3 font-mono text-xs text-secondary-600">{session.chargeCode}</td>
                    <td className="px-6 py-3"><StatusBadge status={session.status} /></td>
                  </tr>
                ))}
                {filteredSessions.length === 0 && (
                  <tr>
                    <td colSpan={8} className="px-6 py-8 text-center text-sm text-secondary-400">No session notes found</td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Dashboard */}
      {activeTab === 'dashboard' && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          {/* Profession distribution */}
          <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="mb-4 text-base font-semibold text-secondary-900">Profession Distribution</h2>
            <div className="grid grid-cols-2 gap-3">
              {(Object.keys(professionLabels) as (keyof typeof professionLabels)[]).map(key => (
                <div key={key} className={`rounded-lg p-3 text-center ${professionClasses[key] ?? ''}`}>
                  <p className="text-2xl font-bold">{mockTreatmentPlans.filter(p => p.profession === key).length}</p>
                  <p className="mt-1 text-xs font-medium">{professionLabels[key]}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Claim status */}
          <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="mb-4 text-base font-semibold text-secondary-900">Claim Status Overview</h2>
            <div className="grid grid-cols-2 gap-3">
              {(['accepted', 'under_review', 'submitted', 'draft'] as const).map(status => (
                <div key={status} className={`rounded-lg p-3 text-center ${statusClasses[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
                  <p className="text-2xl font-bold">{mockACCClaims.filter(c => c.status === status).length}</p>
                  <p className="mt-1 text-xs font-medium">{status.replace(/_/g, ' ')}</p>
                </div>
              ))}
            </div>
          </div>

          {/* Upcoming reviews */}
          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <h2 className="border-b border-secondary-200 px-6 py-4 text-base font-semibold text-secondary-900">Upcoming Reviews</h2>
            <ul className="divide-y divide-secondary-100">
              {mockTreatmentPlans
                .filter(p => p.status === 'active' || p.status === 'under_review')
                .sort((a, b) => a.reviewDate.localeCompare(b.reviewDate))
                .slice(0, 5)
                .map(plan => (
                  <li key={plan.id} className="flex items-center justify-between px-6 py-3">
                    <div>
                      <p className="text-sm font-medium text-secondary-900">{plan.patientName}</p>
                      <p className="text-xs text-secondary-500">{professionLabels[plan.profession] ?? plan.profession} · Review: {plan.reviewDate} · {plan.clinician}</p>
                    </div>
                    <StatusBadge status={plan.status} />
                  </li>
                ))}
            </ul>
          </div>

          {/* Expiring claims */}
          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <h2 className="border-b border-secondary-200 px-6 py-4 text-base font-semibold text-secondary-900">Expiring Claims</h2>
            <ul className="divide-y divide-secondary-100">
              {mockACCClaims
                .filter(c => c.status === 'accepted')
                .sort((a, b) => a.expiryDate.localeCompare(b.expiryDate))
                .slice(0, 5)
                .map(claim => (
                  <li key={claim.id} className="flex items-center justify-between px-6 py-3">
                    <div>
                      <p className="text-sm font-medium text-secondary-900">{claim.patientName}</p>
                      <p className="text-xs text-secondary-500">{professionLabels[claim.claimType] ?? claim.claimType} · Expires: {claim.expiryDate} · {claim.usedSessions}/{claim.approvedSessions} sessions used</p>
                    </div>
                    <StatusBadge status={claim.status} />
                  </li>
                ))}
            </ul>
          </div>
        </div>
      )}

      {/* Plan detail modal */}
      {selectedPlan && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/50" onClick={() => setSelectedPlan(null)} />
          <div className="relative w-full max-w-2xl rounded-xl bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div className="flex items-center gap-3">
                <ProfessionBadge profession={selectedPlan.profession} />
                <h2 className="text-lg font-semibold text-secondary-900">{selectedPlan.patientName}</h2>
              </div>
              <button onClick={() => setSelectedPlan(null)} className="rounded p-1 text-secondary-400 hover:text-secondary-700">
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="grid grid-cols-2 gap-4 p-6">
              <div>
                <p className="text-xs font-medium text-secondary-500">NHI</p>
                <p className="mt-0.5 font-mono text-sm font-semibold text-secondary-900">{selectedPlan.patientNHI}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Clinician</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.clinician}</p>
              </div>
              <div className="col-span-2">
                <p className="text-xs font-medium text-secondary-500">Diagnosis</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.diagnosis}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">ACC Claim</p>
                <p className="mt-0.5 font-mono text-sm text-secondary-900">{selectedPlan.accNumber ?? 'N/A'}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Status</p>
                <div className="mt-0.5"><StatusBadge status={selectedPlan.status} /></div>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Sessions</p>
                <ProgressBar used={selectedPlan.sessionsUsed} approved={selectedPlan.sessionsApproved} profession={selectedPlan.profession} />
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Start Date</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.startDate}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Review Date</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.reviewDate}</p>
              </div>
              <div className="col-span-2 border-t border-secondary-100 pt-4">
                <p className="text-xs font-medium text-secondary-500">Goals &amp; Interventions</p>
                <p className="mt-1 text-sm text-secondary-500">View detailed goals, interventions, and outcome measures in the profession-specific module.</p>
              </div>
            </div>
            <div className="flex justify-end gap-3 border-t border-secondary-200 px-6 py-4">
              <button onClick={() => setSelectedPlan(null)} className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                Close
              </button>
              <button className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                Edit Plan
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Toast */}
      {toast && <Toast message={toast.message} severity={toast.severity} onClose={() => setToast(null)} />}
    </AppShell>
  );
}
