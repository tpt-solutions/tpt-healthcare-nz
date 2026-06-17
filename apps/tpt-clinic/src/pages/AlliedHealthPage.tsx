import { useState } from 'react';
import AppShell from '@/components/AppShell';
import {
  Tab,
  ToastSeverity,
  professionClasses,
  professionLabels,
  mockTreatmentPlans,
  mockACCClaims,
  mockSessionNotes,
} from './alliedHealthTypes';
import { TabBar, Toast } from './AlliedHealthComponents';
import TreatmentPlansPanel from './TreatmentPlansPanel';
import ACCClaimsPanel from './ACCClaimsPanel';
import SessionNotesPanel from './SessionNotesPanel';
import AlliedHealthDashboard from './AlliedHealthDashboard';

export default function AlliedHealthPage() {
  const [activeTab, setActiveTab] = useState<Tab>('plans');
  const [searchTerm, setSearchTerm] = useState('');
  const [professionFilter, setProfessionFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [toast, setToast] = useState<{ message: string; severity: ToastSeverity } | null>(null);

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
      <div className="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
        {(['physiotherapy', 'occupational_therapy', 'speech_language_therapy', 'podiatry'] as const).map(prof => (
          <div key={prof} className={`rounded-xl border p-4 ${professionClasses[prof] ?? ''}`}>
            <p className="text-2xl font-bold">{mockTreatmentPlans.filter(p => p.profession === prof).length}</p>
            <p className="mt-1 text-xs font-medium">{professionLabels[prof]}</p>
          </div>
        ))}
      </div>

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

      {activeTab === 'plans' && (
        <TreatmentPlansPanel
          plans={filteredPlans}
          onNewPlan={() => showToast('Navigate to profession-specific page to create new treatment plan', 'info')}
        />
      )}
      {activeTab === 'claims' && (
        <ACCClaimsPanel
          claims={filteredClaims}
          onNewClaim={() => showToast('Navigate to ACC Claims to create new claim', 'info')}
        />
      )}
      {activeTab === 'sessions' && (
        <SessionNotesPanel
          sessions={filteredSessions}
          onNewSession={() => showToast('Navigate to profession-specific page to create new session note', 'info')}
        />
      )}
      {activeTab === 'dashboard' && (
        <AlliedHealthDashboard plans={mockTreatmentPlans} claims={mockACCClaims} />
      )}

      {toast && <Toast message={toast.message} severity={toast.severity} onClose={() => setToast(null)} />}
    </AppShell>
  );
}
