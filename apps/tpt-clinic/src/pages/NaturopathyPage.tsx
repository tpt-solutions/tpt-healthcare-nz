import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'consultations' | 'remedies' | 'supplements';

interface Consultation { id: string; patientNhi: string; chiefComplaint: string; createdAt: number }
interface Remedy { id: string; patientNhi: string; supplementName: string; createdAt: number }
interface Supplement { id: string; name: string; category: string; active: boolean }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'], ['consultations', 'Consultations'], ['remedies', 'Patient Remedies'], ['supplements', 'Supplement Catalog'],
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

export default function NaturopathyPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [consultations] = useState<Consultation[]>([]);
  const [remedies] = useState<Remedy[]>([]);
  const [supplements, setSupplements] = useState<Supplement[]>([]);

  useEffect(() => {
    fetch('/api/v1/supplements').then(r => r.ok ? r.json() : [])
      .then(s => setSupplements(s ?? []))
      .catch(() => {});
  }, []);

  return (
    <AppShell title="Naturopathy">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Consultations', value: consultations.length, border: 'border-primary-200' },
              { label: 'Patient Remedies', value: remedies.length, border: 'border-green-200' },
              { label: 'Supplement Catalog', value: supplements.length, border: 'border-amber-200' },
              { label: 'Active Patients', value: new Set(remedies.map(r => r.patientNhi)).size, border: 'border-secondary-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Naturopathy — Natural Health &amp; Wellness</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Document naturopathic consultations including full health history, diet assessment,
              and lifestyle review. Prescribe supplements and herbal remedies from the practice
              catalog. Private pay billing — no ACC funding for naturopathy services.
            </p>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('consultations')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                New Consultation
              </button>
              <button onClick={() => setActiveTab('supplements')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                View Catalog
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'consultations' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Naturopathic Consultations</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Consultation
            </button>
          </div>
          {consultations.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No consultations recorded.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Chief Complaint</th>
                  <th className="px-6 py-3 text-left">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {consultations.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.patientNhi}</td>
                    <td className="px-6 py-3">{c.chiefComplaint}</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(c.createdAt).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'remedies' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Patient Remedy Prescriptions</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Prescribe Remedy
            </button>
          </div>
          {remedies.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No remedies prescribed.</div>
          ) : null}
        </div>
      )}

      {activeTab === 'supplements' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Supplement Catalog</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Add Supplement
            </button>
          </div>
          {supplements.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">Supplement catalog is empty.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Name</th>
                  <th className="px-6 py-3 text-left">Category</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {supplements.map(s => (
                  <tr key={s.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-medium">{s.name}</td>
                    <td className="px-6 py-3 capitalize text-secondary-600">{s.category}</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${s.active ? 'bg-green-100 text-green-800' : 'bg-secondary-100 text-secondary-700'}`}>
                        {s.active ? 'Active' : 'Inactive'}
                      </span>
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
