import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'assessments' | 'treatments';

interface Assessment { id: string; patientNhi: string; diagnosis: string; createdAt: number }
interface Treatment { id: string; patientNhi: string; outcome: string; createdAt: number }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  return (
    <div className="mb-6 flex gap-1 rounded-lg bg-secondary-100 p-1">
      {([['overview','Overview'],['assessments','Assessments'],['treatments','Treatments']] as [Tab,string][]).map(([id, label]) => (
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

function EmptyState({ message }: { message: string }) {
  return <div className="flex h-32 items-center justify-center text-sm text-secondary-400">{message}</div>;
}

export default function OsteopathyPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [assessments, setAssessments] = useState<Assessment[]>([]);
  const [treatments, setTreatments] = useState<Treatment[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      fetch('/api/v1/assessments').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/treatments').then(r => r.ok ? r.json() : []),
    ])
      .then(([a, t]) => { setAssessments(a ?? []); setTreatments(t ?? []); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  return (
    <AppShell title="Osteopathy">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Assessments', value: assessments.length, border: 'border-primary-200' },
              { label: 'Treatments', value: treatments.length, border: 'border-green-200' },
              { label: 'Active Cases', value: treatments.filter(t => t.outcome === '').length, border: 'border-amber-200' },
              { label: 'This Month', value: treatments.filter(t => new Date(t.createdAt).getMonth() === new Date().getMonth()).length, border: 'border-secondary-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Osteopathic Medicine</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Osteopathic assessments include postural analysis, palpation findings, range of motion,
              and orthopedic tests. Treatments document OMT techniques including HVLA, MET,
              counterstrain, craniosacral, fascial, and visceral approaches.
            </p>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('assessments')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                New Assessment
              </button>
              <button onClick={() => setActiveTab('treatments')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                Record Treatment
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'assessments' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Osteopathic Assessments</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Assessment
            </button>
          </div>
          {loading ? (
            <div className="flex h-32 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
            </div>
          ) : assessments.length === 0 ? (
            <EmptyState message="No assessments recorded." />
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Diagnosis</th>
                  <th className="px-6 py-3 text-left">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {assessments.map(a => (
                  <tr key={a.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{a.patientNhi}</td>
                    <td className="px-6 py-3">{a.diagnosis}</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(a.createdAt).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'treatments' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Treatment Records</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Treatment
            </button>
          </div>
          {treatments.length === 0 ? <EmptyState message="No treatment records." /> : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Outcome</th>
                  <th className="px-6 py-3 text-left">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {treatments.map(t => (
                  <tr key={t.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{t.patientNhi}</td>
                    <td className="px-6 py-3">{t.outcome || '—'}</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(t.createdAt).toLocaleDateString('en-NZ')}</td>
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
