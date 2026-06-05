import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'soap-notes' | 'screening' | 'acc-claims';

interface SOAPNote { id: string; patientNhi: string; createdAt: number }
interface ACCClaim { id: string; patientNhi: string; status: string; bodyRegion: string }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'], ['soap-notes', 'SOAP Notes'], ['screening', 'Contraindication Screening'], ['acc-claims', 'ACC Claims'],
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

export default function MassagePage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [claims, setClaims] = useState<ACCClaim[]>([]);
  const [notes] = useState<SOAPNote[]>([]);

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/acc/claims').then(r => r.ok ? r.json() : []),
    ])
      .then(([c]) => { setClaims(c ?? []); })
      .catch(() => {});
  }, []);

  return (
    <AppShell title="Massage Therapy">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'ACC Claims', value: claims.length, border: 'border-primary-200' },
              { label: 'SOAP Notes', value: notes.length, border: 'border-green-200' },
              { label: 'Pending Claims', value: claims.filter(c => c.status === 'submitted').length, border: 'border-amber-200' },
              { label: 'Accepted', value: claims.filter(c => c.status === 'accepted').length, border: 'border-secondary-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">ACC-Registered Massage Therapy</h2>
            <p className="mt-2 text-sm text-secondary-500">
              ACC covers massage therapy for injury-related conditions. Always complete a contraindication
              screening before treatment. SOAP notes document subjective, objective, assessment and plan.
              Practitioners must hold current ACC Provider status.
            </p>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('screening')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                New Screening
              </button>
              <button onClick={() => setActiveTab('soap-notes')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                New SOAP Note
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'soap-notes' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">SOAP Notes</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Note
            </button>
          </div>
          <div className="p-6">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
              {['Subjective', 'Objective', 'Assessment', 'Plan'].map(section => (
                <div key={section} className="rounded-lg border border-secondary-200 p-4">
                  <p className="text-xs font-semibold uppercase text-secondary-500">{section}</p>
                  <textarea
                    rows={3}
                    placeholder={`Enter ${section.toLowerCase()} findings...`}
                    className="mt-2 w-full resize-none rounded-md border border-secondary-200 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
                  />
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {activeTab === 'screening' && (
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
          <h2 className="mb-4 text-base font-semibold text-secondary-900">Contraindication Screening</h2>
          <p className="mb-4 text-sm text-secondary-500">
            Complete before each massage session. Contraindications include active DVT, acute inflammation,
            open wounds, fever, and certain skin conditions. Consent must be documented.
          </p>
          <div className="space-y-3">
            {[
              'Active DVT or thrombus', 'Acute fracture or sprain', 'Open wounds or skin infection',
              'Fever or acute infection', 'Uncontrolled high blood pressure', 'Recent surgery (within 6 weeks)',
              'Malignancy in treatment area', 'Anticoagulant therapy',
            ].map(item => (
              <label key={item} className="flex items-center gap-3">
                <input type="checkbox" className="h-4 w-4 rounded border-secondary-300 text-primary-600" />
                <span className="text-sm text-secondary-700">{item}</span>
              </label>
            ))}
          </div>
          <button className="mt-6 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
            Save Screening
          </button>
        </div>
      )}

      {activeTab === 'acc-claims' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">ACC Claims</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Claim
            </button>
          </div>
          {claims.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No ACC claims.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Body Region</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {claims.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.patientNhi}</td>
                    <td className="px-6 py-3 capitalize">{c.bodyRegion}</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        c.status === 'accepted' ? 'bg-green-100 text-green-800' :
                        c.status === 'submitted' ? 'bg-blue-100 text-blue-800' :
                        'bg-secondary-100 text-secondary-700'
                      }`}>{c.status}</span>
                    </td>
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
