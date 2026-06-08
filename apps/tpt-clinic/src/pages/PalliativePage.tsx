import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'patients' | 'acp-plans' | 'pain';

interface PalliativePatient {
  id: string; patientNhi: string; primaryDiagnosis: string; performanceStatus: string;
  careSetting: string; responsibleClinicianId: string; admissionDate: string;
  dnacprInPlace: boolean; preferredPlaceOfDeath?: string;
}
interface ACPPlan {
  id: string; patientNhi: string; status: string; treatmentIntent: string;
  dnacpr: boolean; reviewDate: string;
}
interface PainAssessment {
  id: string; patientNhi: string; painScore: number; severity: string;
  painType: string; assessmentDate: string;
}
interface PainProtocol {
  id: string; patientNhi: string; step: string; nextReviewDate: string;
  prescribedBy: string; outcomeScore?: number;
}

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'],
    ['patients', 'Patients'],
    ['acp-plans', 'Care Plans'],
    ['pain', 'Pain Protocols'],
  ];
  return (
    <div className="mb-6 flex gap-1 rounded-lg bg-secondary-100 p-1">
      {tabs.map(([id, label]) => (
        <button key={id} onClick={() => onSelect(id)}
          className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === id ? 'bg-white text-primary-700 shadow-sm' : 'text-secondary-600 hover:text-secondary-900'
          }`}>
          {label}
        </button>
      ))}
    </div>
  );
}

function PPSBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    '100': 'bg-green-100 text-green-800', '90': 'bg-green-100 text-green-800', '80': 'bg-blue-100 text-blue-800',
    '70': 'bg-blue-100 text-blue-800', '60': 'bg-amber-100 text-amber-800', '50': 'bg-amber-100 text-amber-800',
    '40': 'bg-orange-100 text-orange-800', '30': 'bg-orange-100 text-orange-800', '20': 'bg-red-100 text-red-800',
    '10': 'bg-red-100 text-red-800', '0': 'bg-secondary-100 text-secondary-700',
  };
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${map[status] ?? 'bg-secondary-100 text-secondary-700'}`}>
      PPS {status}
    </span>
  );
}

function SettingBadge({ setting }: { setting: string }) {
  const map: Record<string, string> = {
    home: 'bg-emerald-100 text-emerald-800', inpatient: 'bg-blue-100 text-blue-800',
    residential: 'bg-purple-100 text-purple-800', hospital: 'bg-amber-100 text-amber-800',
  };
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${map[setting] ?? 'bg-secondary-100 text-secondary-700'}`}>
      {setting}
    </span>
  );
}

function StepBadge({ step }: { step: string }) {
  const map: Record<string, string> = {
    step_1_non_opioid: 'bg-blue-100 text-blue-800', step_2_weak_opioid: 'bg-amber-100 text-amber-800',
    step_3_strong_opioid: 'bg-orange-100 text-orange-800', step_4_interventional: 'bg-red-100 text-red-800',
  };
  const label: Record<string, string> = {
    step_1_non_opioid: 'Step 1', step_2_weak_opioid: 'Step 2', step_3_strong_opioid: 'Step 3', step_4_interventional: 'Step 4',
  };
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${map[step] ?? 'bg-secondary-100 text-secondary-700'}`}>
      {label[step] ?? step.replace(/_/g, ' ')}
    </span>
  );
}

function SeverityBadge({ severity }: { severity: string }) {
  const map: Record<string, string> = {
    mild: 'bg-green-100 text-green-800', moderate: 'bg-amber-100 text-amber-800', severe: 'bg-red-100 text-red-800',
  };
  return <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${map[severity] ?? 'bg-secondary-100 text-secondary-700'}`}>{severity}</span>;
}

export default function PalliativePage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [patients, setPatients] = useState<PalliativePatient[]>([]);
  const [plans, setPlans] = useState<ACPPlan[]>([]);
  const [assessments, setAssessments] = useState<PainAssessment[]>([]);
  const [protocols, setProtocols] = useState<PainProtocol[]>([]);

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/palliative/patients').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/palliative/acp-plans').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/palliative/pain-assessments').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/palliative/pain-protocols').then(r => r.ok ? r.json() : []),
    ])
      .then(([p, a, pa, pr]) => {
        setPatients(p ?? []);
        setPlans(a ?? []);
        setAssessments(pa ?? []);
        setProtocols(pr ?? []);
      })
      .catch(() => {});
  }, []);

  const activePatients = patients.filter(pr => !pr.dnacprInPlace);
  const totalActive = patients.length;
  const severePainCount = assessments.filter(a => a.severity === 'severe').length;
  const acpActive = plans.filter(p => ['active', 'draft'].includes(p.status)).length;

  return (
    <AppShell title="Palliative Care">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Active Patients', value: totalActive, border: 'border-primary-200' },
              { label: 'ACP Plans Pending', value: acpActive, border: 'border-purple-200' },
              { label: 'Severe Pain (last 24h)', value: severePainCount, border: 'border-red-200' },
              { label: 'Pain Protocols', value: protocols.length, border: 'border-amber-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Palliative Care & Hospice</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Manage palliative care admissions, advance care planning (ACP), WHO analgesic ladder
              protocols, and end-of-life care coordination. All records are classified as
              extra-sensitive under HIPC.
            </p>
            <div className="mt-3 rounded-md bg-rose-50 border border-rose-200 px-4 py-3">
              <p className="text-xs font-medium text-rose-800">
                End-of-Life Data Warning: Palliative records carry extra sensitive classification.
                DNACPR and ACP decisions must be documented with patient consent.
              </p>
            </div>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('patients')} className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                New Admission
              </button>
              <button onClick={() => setActiveTab('acp-plans')} className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                New ACP Plan
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'patients' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Palliative Patients</h2>
              <p className="text-xs text-secondary-500 mt-0.5">Hospice, home, residential and hospital-based care</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Admission
            </button>
          </div>
          {patients.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No palliative admissions.</p>
              <p className="text-xs">Admit a patient to the palliative care programme.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr><th className="px-6 py-3 text-left">Patient</th><th className="px-6 py-3 text-left">Setting</th><th className="px-6 py-3 text-left">PPS</th><th className="px-6 py-3 text-left">DNACPR</th><th className="px-6 py-3 text-left">Clinician</th></tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {patients.map(p => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{p.patientNhi}</td>
                    <td className="px-6 py-3"><SettingBadge setting={p.careSetting} /></td>
                    <td className="px-6 py-3"><PPSBadge status={p.performanceStatus} /></td>
                    <td className="px-6 py-3">{p.dnacprInPlace ? 'Yes' : 'No'}</td>
                    <td className="px-6 py-3 text-secondary-500">{p.responsibleClinicianId}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'acp-plans' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Advance Care Plans</h2>
              <p className="text-xs text-rose-700 mt-0.5">Extra-sensitive — requires documented consent</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New ACP Plan
            </button>
          </div>
          {plans.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No advance care plans.</p>
              <p className="text-xs">Create an ACP documenting patient wishes and DNACPR status.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr><th className="px-6 py-3 text-left">Patient</th><th className="px-6 py-3 text-left">Status</th><th className="px-6 py-3 text-left">Intent</th><th className="px-6 py-3 text-left">DNACPR</th><th className="px-6 py-3 text-left">Review</th></tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {plans.map(pl => (
                  <tr key={pl.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{pl.patientNhi}</td>
                    <td className="px-6 py-3 capitalize">{pl.status}</td>
                    <td className="px-6 py-3 capitalize">{pl.treatmentIntent.replace(/_/g, ' ')}</td>
                    <td className="px-6 py-3">{pl.dnacpr ? 'Yes' : 'No'}</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(pl.reviewDate).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'pain' && (
        <div className="space-y-6">
          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div>
                <h2 className="text-base font-semibold text-secondary-900">Pain Assessments</h2>
                <p className="text-xs text-secondary-500 mt-0.5">Numeric rating scale (0-10) with impact scoring</p>
              </div>
              <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                + New Assessment
              </button>
            </div>
            {assessments.length === 0 ? (
              <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
                <p className="text-sm">No pain assessments recorded.</p>
                <p className="text-xs">Record a structured pain assessment for a palliative patient.</p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                  <tr><th className="px-6 py-3 text-left">Patient</th><th className="px-6 py-3 text-left">Score</th><th className="px-6 py-3 text-left">Severity</th><th className="px-6 py-3 text-left">Type</th><th className="px-6 py-3 text-left">Date</th></tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {assessments.map(a => (
                    <tr key={a.id} className="hover:bg-secondary-50">
                      <td className="px-6 py-3 font-mono text-xs">{a.patientNhi}</td>
                      <td className="px-6 py-3">{a.painScore}/10</td>
                      <td className="px-6 py-3"><SeverityBadge severity={a.severity} /></td>
                      <td className="px-6 py-3 capitalize">{a.painType.replace(/_/g, ' ')}</td>
                      <td className="px-6 py-3 text-secondary-500">{new Date(a.assessmentDate).toLocaleDateString('en-NZ')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div>
                <h2 className="text-base font-semibold text-secondary-900">Pain Protocols</h2>
                <p className="text-xs text-secondary-500 mt-0.5">WHO analgesic ladder — Step 1 through Step 4</p>
              </div>
              <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                + New Protocol
              </button>
            </div>
            {protocols.length === 0 ? (
              <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
                <p className="text-sm">No pain protocols active.</p>
                <p className="text-xs">Assign a WHO analgesic ladder step and regimen to a patient.</p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                  <tr><th className="px-6 py-3 text-left">Patient</th><th className="px-6 py-3 text-left">Step</th><th className="px-6 py-3 text-left">Next Review</th><th className="px-6 py-3 text-left">Prescribed By</th><th className="px-6 py-3 text-left">Outcome</th></tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {protocols.map(pr => (
                    <tr key={pr.id} className="hover:bg-secondary-50">
                      <td className="px-6 py-3 font-mono text-xs">{pr.patientNhi}</td>
                      <td className="px-6 py-3"><StepBadge step={pr.step} /></td>
                      <td className="px-6 py-3 text-secondary-500">{new Date(pr.nextReviewDate).toLocaleDateString('en-NZ')}</td>
                      <td className="px-6 py-3">{pr.prescribedBy}</td>
                      <td className="px-6 py-3">{pr.outcomeScore !== undefined ? `${pr.outcomeScore}/10` : '-'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}
    </AppShell>
  );
}
