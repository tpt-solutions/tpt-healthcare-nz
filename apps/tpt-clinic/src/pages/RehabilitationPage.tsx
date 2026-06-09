import React, { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface RehabAdmission {
  id: string;
  patientNhi: string;
  clinicianHpi: string;
  ward: string;
  admissionType: string;
  admissionSource: string;
  primaryDiagnosis: string;
  status: string;
  mobilityOnAdmission: string;
  cognitiveStatus: string;
  admittedAt: string;
  dischargedAt?: string;
}

interface RehabGoal {
  id: string;
  admissionId: string;
  discipline: string;
  goalType: string;
  goalText: string;
  targetDate?: string;
  status: string;
  progressNotes?: string;
  achievedAt?: string;
}

interface FIMScore {
  id: string;
  admissionId: string;
  assessedByHpi: string;
  assessmentType: string;
  motorFimTotal: number;
  cognitiveFimTotal: number;
  totalFimScore: number;
  assessedAt: string;
}

interface CommunityEpisode {
  id: string;
  patientNhi: string;
  episodeType: string;
  primaryDiagnosis: string;
  status: string;
  disciplines: string;
  visitsCompleted?: number;
  visitsPlanned?: number;
  referredAt: string;
}

interface ACCPlan {
  id: string;
  patientNhi: string;
  accClaimNumber: string;
  accContractType: string;
  injuryDescription: string;
  status: string;
  fundingApprovedNzd?: number;
  fundingSpentNzd?: number;
  planDate?: string;
}

interface NASCReferral {
  id: string;
  patientNhi: string;
  nascRegion: string;
  referralReason: string;
  urgency: string;
  status: string;
  nascReference?: string;
  createdAt: string;
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------
type TabId = 'admissions' | 'goals' | 'fim' | 'community' | 'acc' | 'nasc';

const TABS: { id: TabId; label: string }[] = [
  { id: 'admissions', label: 'Inpatient' },
  { id: 'goals',      label: 'Goals' },
  { id: 'fim',        label: 'FIM Scores' },
  { id: 'community',  label: 'Community' },
  { id: 'acc',        label: 'ACC Plans' },
  { id: 'nasc',       label: 'NASC' },
];

function TabBar({ active, onChange }: { active: TabId; onChange: (t: TabId) => void }) {
  return (
    <div className="flex gap-1 border-b border-secondary-200 pb-0">
      {TABS.map((t) => (
        <button
          key={t.id}
          onClick={() => onChange(t.id)}
          className={[
            'px-4 py-2 text-sm font-medium rounded-t-md transition-colors',
            active === t.id
              ? 'bg-white border border-b-white border-secondary-200 text-primary-600 -mb-px'
              : 'text-secondary-500 hover:text-secondary-800',
          ].join(' ')}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colours: Record<string, string> = {
    admitted:              'bg-blue-100 text-blue-700',
    active:                'bg-green-100 text-green-700',
    'discharge-planning':  'bg-yellow-100 text-yellow-700',
    discharged:            'bg-secondary-100 text-secondary-500',
    transferred:           'bg-purple-100 text-purple-700',
    referred:              'bg-blue-100 text-blue-700',
    completed:             'bg-green-100 text-green-700',
    withdrawn:             'bg-secondary-100 text-secondary-500',
    declined:              'bg-red-100 text-red-700',
    achieved:              'bg-green-100 text-green-700',
    'not-achieved':        'bg-red-100 text-red-700',
    modified:              'bg-orange-100 text-orange-700',
    discontinued:          'bg-secondary-100 text-secondary-500',
    draft:                 'bg-secondary-100 text-secondary-500',
    submitted:             'bg-blue-100 text-blue-700',
    approved:              'bg-green-100 text-green-700',
    review:                'bg-yellow-100 text-yellow-700',
    acknowledged:          'bg-teal-100 text-teal-700',
    'assessment-scheduled': 'bg-indigo-100 text-indigo-700',
    assessed:              'bg-purple-100 text-purple-700',
  };
  const cls = colours[status] ?? 'bg-secondary-100 text-secondary-600';
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${cls}`}>
      {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Tab panels
// ---------------------------------------------------------------------------
function AdmissionsTab({ items }: { items: RehabAdmission[] }) {
  if (items.length === 0) return <EmptyState message="No inpatient rehabilitation admissions found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Ward', 'Type', 'Status', 'Primary Diagnosis', 'Mobility', 'Admitted'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((a) => (
            <tr key={a.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{a.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-600">{a.ward || '—'}</td>
              <td className="px-4 py-3 text-secondary-600 capitalize">{a.admissionType}</td>
              <td className="px-4 py-3"><StatusBadge status={a.status} /></td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{a.primaryDiagnosis || '—'}</td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{a.mobilityOnAdmission || '—'}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(a.admittedAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function GoalsTab({ items }: { items: RehabGoal[] }) {
  if (items.length === 0) return <EmptyState message="No therapy goals found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['Type', 'Discipline', 'Goal', 'Target Date', 'Status', 'Progress'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((g) => (
            <tr key={g.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3">
                <span className={`inline-flex items-center rounded px-2 py-0.5 text-xs font-bold ${g.goalType === 'LTG' ? 'bg-indigo-100 text-indigo-700' : 'bg-teal-100 text-teal-700'}`}>
                  {g.goalType}
                </span>
              </td>
              <td className="px-4 py-3 text-secondary-600 capitalize">{g.discipline.replace(/-/g, ' ')}</td>
              <td className="px-4 py-3 text-secondary-700 max-w-sm truncate">{g.goalText}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{g.targetDate ?? '—'}</td>
              <td className="px-4 py-3"><StatusBadge status={g.status} /></td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{g.progressNotes || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function FIMTab({ items }: { items: FIMScore[] }) {
  if (items.length === 0) return <EmptyState message="No FIM assessments found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['Assessment', 'Assessed By', 'Motor FIM', 'Cognitive FIM', 'Total FIM', 'Date'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((f) => (
            <tr key={f.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 text-secondary-700 capitalize">{f.assessmentType}</td>
              <td className="px-4 py-3 font-mono text-secondary-500 text-xs">{f.assessedByHpi || '—'}</td>
              <td className="px-4 py-3">
                <span className={f.motorFimTotal < 39 ? 'text-red-600 font-semibold' : f.motorFimTotal < 65 ? 'text-orange-600' : 'text-green-600'}>
                  {f.motorFimTotal}/91
                </span>
              </td>
              <td className="px-4 py-3">
                <span className={f.cognitiveFimTotal < 15 ? 'text-red-600 font-semibold' : f.cognitiveFimTotal < 25 ? 'text-orange-600' : 'text-green-600'}>
                  {f.cognitiveFimTotal}/35
                </span>
              </td>
              <td className="px-4 py-3">
                <span className={`font-semibold ${f.totalFimScore < 54 ? 'text-red-600' : f.totalFimScore < 90 ? 'text-orange-600' : 'text-green-600'}`}>
                  {f.totalFimScore}/126
                </span>
              </td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(f.assessedAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CommunityTab({ items }: { items: CommunityEpisode[] }) {
  if (items.length === 0) return <EmptyState message="No community rehabilitation episodes found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Type', 'Diagnosis', 'Status', 'Disciplines', 'Progress', 'Referred'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((c) => (
            <tr key={c.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{c.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-600 capitalize">{c.episodeType.replace(/-/g, ' ')}</td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{c.primaryDiagnosis || '—'}</td>
              <td className="px-4 py-3"><StatusBadge status={c.status} /></td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{c.disciplines || '—'}</td>
              <td className="px-4 py-3 text-secondary-600">
                {c.visitsPlanned != null
                  ? `${c.visitsCompleted ?? 0} / ${c.visitsPlanned} visits`
                  : '—'}
              </td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(c.referredAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ACCTab({ items }: { items: ACCPlan[] }) {
  if (items.length === 0) return <EmptyState message="No ACC rehabilitation plans found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Claim No.', 'Contract Type', 'Injury', 'Status', 'Funding', 'Spent'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((p) => (
            <tr key={p.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{p.patientNhi}</td>
              <td className="px-4 py-3 font-mono text-secondary-600 text-xs">{p.accClaimNumber || '—'}</td>
              <td className="px-4 py-3 text-secondary-600 capitalize">{p.accContractType.replace(/-/g, ' ')}</td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{p.injuryDescription || '—'}</td>
              <td className="px-4 py-3"><StatusBadge status={p.status} /></td>
              <td className="px-4 py-3 text-secondary-700">
                {p.fundingApprovedNzd != null ? `$${p.fundingApprovedNzd.toLocaleString('en-NZ', { minimumFractionDigits: 2 })}` : '—'}
              </td>
              <td className="px-4 py-3 text-secondary-700">
                {p.fundingSpentNzd != null ? `$${p.fundingSpentNzd.toLocaleString('en-NZ', { minimumFractionDigits: 2 })}` : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function NASCTab({ items }: { items: NASCReferral[] }) {
  if (items.length === 0) return <EmptyState message="No NASC referrals found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Region', 'Reason', 'Urgency', 'Status', 'NASC Ref', 'Created'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((n) => (
            <tr key={n.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{n.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-600">{n.nascRegion || '—'}</td>
              <td className="px-4 py-3 text-secondary-600 capitalize">{n.referralReason.replace(/-/g, ' ')}</td>
              <td className="px-4 py-3">
                <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  n.urgency === 'emergency' ? 'bg-red-100 text-red-700' :
                  n.urgency === 'urgent'    ? 'bg-orange-100 text-orange-700' :
                  'bg-secondary-100 text-secondary-600'
                }`}>
                  {n.urgency}
                </span>
              </td>
              <td className="px-4 py-3"><StatusBadge status={n.status} /></td>
              <td className="px-4 py-3 font-mono text-secondary-500 text-xs">{n.nascReference || '—'}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(n.createdAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-secondary-400">
      <svg className="h-12 w-12 mb-3" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
      <p className="text-sm">{message}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------
export default function RehabilitationPage() {
  const [activeTab, setActiveTab]       = useState<TabId>('admissions');
  const [admissions, setAdmissions]     = useState<RehabAdmission[]>([]);
  const [goals, setGoals]               = useState<RehabGoal[]>([]);
  const [fimScores, setFimScores]       = useState<FIMScore[]>([]);
  const [community, setCommunity]       = useState<CommunityEpisode[]>([]);
  const [accPlans, setAccPlans]         = useState<ACCPlan[]>([]);
  const [nascReferrals, setNascReferrals] = useState<NASCReferral[]>([]);
  const [loading, setLoading]           = useState(true);
  const [error, setError]               = useState<string | null>(null);

  useEffect(() => {
    const base = '/api/v1/rehab';
    setLoading(true);
    setError(null);
    Promise.all([
      fetch(`${base}/admissions`).then((r) => r.json()),
      fetch(`${base}/community`).then((r) => r.json()),
      fetch(`${base}/acc-plans`).then((r) => r.json()),
      fetch(`${base}/nasc`).then((r) => r.json()),
    ])
      .then(([adm, com, acc, nasc]) => {
        setAdmissions(Array.isArray(adm) ? adm : []);
        setCommunity(Array.isArray(com) ? com : []);
        setAccPlans(Array.isArray(acc) ? acc : []);
        setNascReferrals(Array.isArray(nasc) ? nasc : []);
        // Goals and FIM scores are nested under admissions — not fetched at page load.
        setGoals([]);
        setFimScores([]);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Failed to load rehabilitation data'))
      .finally(() => setLoading(false));
  }, []);

  return (
    <AppShell title="Rehabilitation">
      <div className="p-6 space-y-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-secondary-900">Rehabilitation</h1>
            <p className="mt-1 text-sm text-secondary-500">
              Inpatient admissions, FIM scoring, therapy goals, community follow-up, ACC plans, and NASC referrals
            </p>
          </div>
        </div>

        {/* Summary cards */}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          {[
            { label: 'Inpatient',  count: admissions.length,     colour: 'blue' },
            { label: 'Goals',      count: goals.length,          colour: 'teal' },
            { label: 'FIM Scores', count: fimScores.length,      colour: 'indigo' },
            { label: 'Community',  count: community.length,      colour: 'green' },
            { label: 'ACC Plans',  count: accPlans.length,       colour: 'orange' },
            { label: 'NASC',       count: nascReferrals.length,  colour: 'purple' },
          ].map((c) => (
            <div key={c.label} className="rounded-xl border border-secondary-200 bg-white px-4 py-3 shadow-sm">
              <p className="text-xs text-secondary-500">{c.label}</p>
              <p className="text-2xl font-semibold text-secondary-900">{loading ? '—' : c.count}</p>
            </div>
          ))}
        </div>

        {/* Error */}
        {error && (
          <div className="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        )}

        {/* Tab content */}
        <div className="rounded-xl border border-secondary-200 bg-white shadow-sm overflow-hidden">
          <div className="px-4 pt-4">
            <TabBar active={activeTab} onChange={setActiveTab} />
          </div>
          <div className="p-4">
            {loading ? (
              <div className="flex items-center justify-center py-16">
                <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
              </div>
            ) : (
              <>
                {activeTab === 'admissions' && <AdmissionsTab items={admissions} />}
                {activeTab === 'goals'      && <GoalsTab      items={goals} />}
                {activeTab === 'fim'        && <FIMTab        items={fimScores} />}
                {activeTab === 'community'  && <CommunityTab  items={community} />}
                {activeTab === 'acc'        && <ACCTab        items={accPlans} />}
                {activeTab === 'nasc'       && <NASCTab       items={nascReferrals} />}
              </>
            )}
          </div>
        </div>
      </div>
    </AppShell>
  );
}
