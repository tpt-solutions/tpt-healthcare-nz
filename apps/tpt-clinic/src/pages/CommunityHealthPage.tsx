import { useState } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'home-visits' | 'district-nursing' | 'outreach';

interface HomeVisit {
  id: string;
  patientNHI: string;
  patientName: string;
  clinician: string;
  scheduledDate: string;
  visitType: string;
  status: string;
  priority: string;
  address: string;
}

interface CarePlan {
  id: string;
  patientNHI: string;
  patientName: string;
  planName: string;
  planType: string;
  status: string;
  riskLevel: string;
  startDate: string;
  reviewDate: string;
}

interface OutreachProgram {
  id: string;
  programName: string;
  programType: string;
  status: string;
  startDate: string;
  targetPopulation: string;
  fundingSource: string;
}

const sectionClasses = 'rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200';

const statusClasses: Record<string, string> = {
  scheduled: 'bg-blue-100 text-blue-800',
  active: 'bg-green-100 text-green-800',
  completed: 'bg-secondary-100 text-secondary-600',
  cancelled: 'bg-red-100 text-red-800',
  draft: 'bg-amber-100 text-amber-800',
  in_progress: 'bg-sky-100 text-sky-800',
};

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${statusClasses[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

function PriorityBadge({ p }: { p: string }) {
  const map: Record<string, string> = {
    urgent: 'bg-red-100 text-red-800',
    high: 'bg-orange-100 text-orange-800',
    routine: 'bg-blue-100 text-blue-800',
    low: 'bg-green-100 text-green-800',
  };
  return <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${map[p] ?? ''}`}>{p}</span>;
}

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: { id: Tab; label: string }[] = [
    { id: 'home-visits', label: 'Home Visits' },
    { id: 'district-nursing', label: 'District Nursing' },
    { id: 'outreach', label: 'Outreach' },
  ];
  return (
    <div className="mb-6 flex gap-1 overflow-x-auto rounded-lg bg-secondary-100 p-1">
      {tabs.map(t => (
        <button
          key={t.id}
          onClick={() => onSelect(t.id)}
          className={`flex-shrink-0 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${active === t.id ? 'bg-white text-primary-700 shadow-sm' : 'text-secondary-600 hover:text-secondary-900'}`}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

const mockHomeVisits: HomeVisit[] = [
  { id: 'hv-001', patientNHI: 'ABC1234', patientName: 'John Smith', clinician: 'Sarah Wilson', scheduledDate: '2026-06-08', visitType: 'wound_care', status: 'scheduled', priority: 'high', address: '12 Example St, Auckland' },
  { id: 'hv-002', patientNHI: 'DEF5678', patientName: 'Mary Johnson', clinician: 'Emma Thompson', scheduledDate: '2026-06-08', visitType: 'medication_review', status: 'in_progress', priority: 'routine', address: '34 Queen St, Auckland' },
  { id: 'hv-003', patientNHI: 'GHI9012', patientName: 'Robert Brown', clinician: 'James Chen', scheduledDate: '2026-06-09', visitType: 'post_acute', status: 'scheduled', priority: 'urgent', address: '56 King St, Christchurch' },
  { id: 'hv-004', patientNHI: 'JKL3456', patientName: 'Susan Davis', clinician: 'Lisa Anderson', scheduledDate: '2026-06-09', visitType: 'diabetes_care', status: 'completed', priority: 'routine', address: '78 Main Rd, Wellington' },
];

const mockCarePlans: CarePlan[] = [
  { id: 'cp-001', patientNHI: 'ABC1234', patientName: 'John Smith', planName: 'Post-operative wound care', planType: 'wound_care', status: 'active', riskLevel: 'moderate', startDate: '2026-05-01', reviewDate: '2026-07-01' },
  { id: 'cp-002', patientNHI: 'DEF5678', patientName: 'Mary Johnson', planName: 'Diabetes management', planType: 'diabetes', status: 'active', riskLevel: 'high', startDate: '2026-04-15', reviewDate: '2026-06-15' },
  { id: 'cp-003', patientNHI: 'MNO7890', patientName: 'Patricia Lee', planName: 'Palliative care pathway', planType: 'palliative', status: 'active', riskLevel: 'very_high', startDate: '2026-03-01', reviewDate: '2026-06-01' },
];

const mockPrograms: OutreachProgram[] = [
  { id: 'op-001', programName: 'Mobile Diabetes Screening', programType: 'screening', status: 'active', startDate: '2026-01-01', targetPopulation: 'elderly, chronic_disease', fundingSource: 'dhb' },
  { id: 'op-002', programName: 'Aranui Community Clinic', programType: 'mobile_clinic', status: 'active', startDate: '2026-02-01', targetPopulation: 'rural', fundingSource: 'pho' },
  { id: 'op-003', programName: 'Youth Vaccination Drive', programType: 'vaccination', status: 'paused', startDate: '2025-09-01', targetPopulation: 'youth', fundingSource: 'moh' },
];

export default function CommunityHealthPage() {
  const [activeTab, setActiveTab] = useState<Tab>('home-visits');

  return (
    <AppShell title="Community Health">
      {/* Stat cards */}
      <div className="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-4">
        <div className="rounded-xl border border-blue-200 bg-blue-50 p-4">
          <p className="text-2xl font-bold text-blue-800">{mockHomeVisits.length}</p>
          <p className="mt-1 text-xs font-medium text-blue-700">Today's Visits</p>
        </div>
        <div className="rounded-xl border border-green-200 bg-green-50 p-4">
          <p className="text-2xl font-bold text-green-800">{mockCarePlans.length}</p>
          <p className="mt-1 text-xs font-medium text-green-700">Active Care Plans</p>
        </div>
        <div className="rounded-xl border border-purple-200 bg-purple-50 p-4">
          <p className="text-2xl font-bold text-purple-800">{mockPrograms.length}</p>
          <p className="mt-1 text-xs font-medium text-purple-700">Outreach Programs</p>
        </div>
        <div className="rounded-xl border border-amber-200 bg-amber-50 p-4">
          <p className="text-2xl font-bold text-amber-800">1</p>
          <p className="mt-1 text-xs font-medium text-amber-700">Urgent Priority</p>
        </div>
      </div>

      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'home-visits' && (
        <div className={sectionClasses}>
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Home Visits</h2>
            <span className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700 cursor-pointer">+ New Visit</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Clinician</th>
                  <th className="px-6 py-3 text-left">Date</th>
                  <th className="px-6 py-3 text-left">Type</th>
                  <th className="px-6 py-3 text-left">Priority</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">Address</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {mockHomeVisits.map(v => (
                  <tr key={v.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3">
                      <p className="font-medium text-secondary-900">{v.patientName}</p>
                      <p className="font-mono text-xs text-secondary-500">NHI: {v.patientNHI}</p>
                    </td>
                    <td className="px-6 py-3 text-secondary-700">{v.clinician}</td>
                    <td className="px-6 py-3 text-secondary-700">{v.scheduledDate}</td>
                    <td className="px-6 py-3 text-secondary-700">{v.visitType.replace(/_/g, ' ')}</td>
                    <td className="px-6 py-3"><PriorityBadge p={v.priority} /></td>
                    <td className="px-6 py-3"><StatusBadge status={v.status} /></td>
                    <td className="px-6 py-3 text-secondary-500">{v.address}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {activeTab === 'district-nursing' && (
        <div className={sectionClasses}>
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">District Nursing Care Plans</h2>
            <span className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700 cursor-pointer">+ New Plan</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Plan</th>
                  <th className="px-6 py-3 text-left">Type</th>
                  <th className="px-6 py-3 text-left">Risk</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">Review</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {mockCarePlans.map(p => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3">
                      <p className="font-medium text-secondary-900">{p.patientName}</p>
                      <p className="font-mono text-xs text-secondary-500">NHI: {p.patientNHI}</p>
                    </td>
                    <td className="px-6 py-3 text-secondary-700">{p.planName}</td>
                    <td className="px-6 py-3 text-secondary-700">{p.planType.replace(/_/g, ' ')}</td>
                    <td className="px-6 py-3"><PriorityBadge p={p.riskLevel} /></td>
                    <td className="px-6 py-3"><StatusBadge status={p.status} /></td>
                    <td className="px-6 py-3 text-secondary-500">{p.reviewDate}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {activeTab === 'outreach' && (
        <div className={sectionClasses}>
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Outreach Programs</h2>
            <span className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700 cursor-pointer">+ New Program</span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Program</th>
                  <th className="px-6 py-3 text-left">Type</th>
                  <th className="px-6 py-3 text-left">Target</th>
                  <th className="px-6 py-3 text-left">Funding</th>
                  <th className="px-6 py-3 text-left">Start Date</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {mockPrograms.map(p => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3">
                      <p className="font-medium text-secondary-900">{p.programName}</p>
                    </td>
                    <td className="px-6 py-3 text-secondary-700">{p.programType.replace(/_/g, ' ')}</td>
                    <td className="px-6 py-3 text-secondary-700">{p.targetPopulation}</td>
                    <td className="px-6 py-3 text-secondary-700">{p.fundingSource.replace(/_/g, ' ')}</td>
                    <td className="px-6 py-3 text-secondary-500">{p.startDate}</td>
                    <td className="px-6 py-3"><StatusBadge status={p.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </AppShell>
  );
}
