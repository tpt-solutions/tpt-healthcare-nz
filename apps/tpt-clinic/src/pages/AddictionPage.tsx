import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'programmes' | 'counselling' | 'prescribing' | 'urine';

interface Programme {
  id: string; patientNhi: string; phase: string; currentDoseMg: number;
  takeHomeLevel: number; nextReviewDate: string; substancePrimary: string;
}
interface CounsellingSession {
  id: string; patientNhi: string; modality: string; sessionType: string;
  sessionDate: string; durationMin: number; readinessScore: number;
}
interface OSTPrescription {
  id: string; patientNhi: string; drug: string; doseMg: number; status: string;
  supervised: boolean; takeHomeDays: number;
}
interface UrineScreen {
  id: string; programmeId: string; collectedAt: string; mssaResult: string;
}

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'],
    ['programmes', 'Methadone Programmes'],
    ['counselling', 'Counselling'],
    ['prescribing', 'Prescribing'],
    ['urine', 'Urine Screening'],
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

function PhaseBadge({ phase }: { phase: string }) {
  const map: Record<string, string> = {
    induction: 'bg-amber-100 text-amber-800',
    stabilisation: 'bg-blue-100 text-blue-800',
    maintenance: 'bg-green-100 text-green-800',
    tapering: 'bg-purple-100 text-purple-800',
    discharged: 'bg-secondary-100 text-secondary-700',
  };
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${map[phase] ?? 'bg-secondary-100 text-secondary-700'}`}>
      {phase.replace('_', ' ')}
    </span>
  );
}

function MSSABadge({ result }: { result: string }) {
  const map: Record<string, string> = {
    conforming: 'bg-green-100 text-green-800',
    non_conforming: 'bg-red-100 text-red-800',
    borderline: 'bg-amber-100 text-amber-800',
  };
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${map[result] ?? 'bg-secondary-100 text-secondary-700'}`}>
      {result.replace('_', ' ')}
    </span>
  );
}

export default function AddictionPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [programmes, setProgrammes] = useState<Programme[]>([]);
  const [sessions, setSessions] = useState<CounsellingSession[]>([]);
  const [prescriptions, setPrescriptions] = useState<OSTPrescription[]>([]);
  const [screens, setScreens] = useState<UrineScreen[]>([]);

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/methadone/programmes').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/counselling/sessions').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/ost/prescriptions').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/methadone/programmes//urine-screens').then(r => r.ok ? r.json() : []),
    ])
      .then(([p, s, rx, u]) => {
        setProgrammes(p ?? []);
        setSessions(s ?? []);
        setPrescriptions(rx ?? []);
        setScreens(u ?? []);
      })
      .catch(() => {});
  }, []);

  const activeProgrammes = programmes.filter(pr => pr.phase !== 'discharged');
  const dosesDueToday = activeProgrammes.length; // simplified
  const takeHomeReviews = activeProgrammes.filter(pr => pr.takeHomeLevel > 1).length;

  return (
    <AppShell title="Addiction Services">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Active Programmes', value: activeProgrammes.length, border: 'border-primary-200' },
              { label: 'Doses Due Today', value: dosesDueToday, border: 'border-blue-200' },
              { label: 'Take-Home Reviews', value: takeHomeReviews, border: 'border-purple-200' },
              { label: 'Counselling Sessions', value: sessions.length, border: 'border-secondary-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Opioid Substitution Therapy & Counselling</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Manage methadone and buprenorphine programmes, supervised dosing, take-home approvals,
              urine drug screens, and addiction-specific counselling sessions. All records are
              classified as extra-sensitive under HIPC Rule 11.
            </p>
            <div className="mt-3 rounded-md bg-amber-50 border border-amber-200 px-4 py-3">
              <p className="text-xs font-medium text-amber-800">
                Controlled Drug Notice: Methadone and buprenorphine are Class B3 controlled substances.
                Every dose administration and adjustment must be witnessed or pharmacist-checked and
                recorded in the audit trail.
              </p>
            </div>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('programmes')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                New Programme
              </button>
              <button onClick={() => setActiveTab('counselling')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                New Counselling Session
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'programmes' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Methadone Programmes</h2>
              <p className="text-xs text-secondary-500 mt-0.5">Induction → Stabilisation → Maintenance → Tapering</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Programme
            </button>
          </div>
          {programmes.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No programmes enrolled.</p>
              <p className="text-xs">Select a patient to start an OST programme.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Phase</th>
                  <th className="px-6 py-3 text-left">Dose (mg)</th>
                  <th className="px-6 py-3 text-left">Take-Home</th>
                  <th className="px-6 py-3 text-left">Next Review</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {programmes.map(p => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{p.patientNhi}</td>
                    <td className="px-6 py-3"><PhaseBadge phase={p.phase} /></td>
                    <td className="px-6 py-3">{p.currentDoseMg} mg</td>
                    <td className="px-6 py-3">Level {p.takeHomeLevel}</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(p.nextReviewDate).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'counselling' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Addiction Counselling</h2>
              <p className="text-xs text-amber-700 mt-0.5">Extra-sensitive records — elevated consent required</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Session
            </button>
          </div>
          {sessions.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No counselling sessions.</p>
              <p className="text-xs">Select a patient to record an individual, group, or family session.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Type</th>
                  <th className="px-6 py-3 text-left">Modality</th>
                  <th className="px-6 py-3 text-left">Readiness</th>
                  <th className="px-6 py-3 text-left">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {sessions.map(s => (
                  <tr key={s.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{s.patientNhi}</td>
                    <td className="px-6 py-3 capitalize">{s.sessionType.replace('_', ' ')}</td>
                    <td className="px-6 py-3 capitalize">{s.modality.replace('_', ' ')}</td>
                    <td className="px-6 py-3">{s.readinessScore}/10</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(s.sessionDate).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'prescribing' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">OST Prescribing</h2>
              <p className="text-xs text-secondary-500 mt-0.5">Controlled drug prescriptions — pharmacist check required</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Prescription
            </button>
          </div>
          {prescriptions.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No OST prescriptions.</p>
              <p className="text-xs">Create a prescription for methadone, buprenorphine, or Suboxone.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Drug</th>
                  <th className="px-6 py-3 text-left">Dose</th>
                  <th className="px-6 py-3 text-left">Supervised</th>
                  <th className="px-6 py-3 text-left">Take-Home Days</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {prescriptions.map(rx => (
                  <tr key={rx.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{rx.patientNhi}</td>
                    <td className="px-6 py-3 capitalize">{rx.drug.replace('_', ' ')}</td>
                    <td className="px-6 py-3">{rx.doseMg} mg</td>
                    <td className="px-6 py-3">{rx.supervised ? 'Yes' : 'No'}</td>
                    <td className="px-6 py-3">{rx.takeHomeDays}</td>
                    <td className="px-6 py-3 capitalize">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        rx.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-secondary-100 text-secondary-700'
                      }`}>{rx.status}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'urine' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Urine Drug Screens</h2>
              <p className="text-xs text-secondary-500 mt-0.5">MSSA-compliant urinalysis results</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Record Screen
            </button>
          </div>
          {screens.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No urine screens recorded.</p>
              <p className="text-xs">Record a drug screen result for an active programme.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Programme</th>
                  <th className="px-6 py-3 text-left">Collected</th>
                  <th className="px-6 py-3 text-left">MSSA Result</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {screens.map(u => (
                  <tr key={u.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{u.programmeId.slice(0, 8)}</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(u.collectedAt).toLocaleDateString('en-NZ')}</td>
                    <td className="px-6 py-3"><MSSABadge result={u.mssaResult} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </AppShell>
  );
}
