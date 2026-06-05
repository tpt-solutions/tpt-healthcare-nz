import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'diagnosis' | 'prescriptions' | 'herbs';

interface Herb { id: string; pinYin: string; englishName: string; category: string; active: boolean }
interface Prescription { id: string; patientNhi: string; name: string; status: string; createdAt: number }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'], ['diagnosis', 'TCM Diagnosis'], ['prescriptions', 'Herb Prescriptions'], ['herbs', 'Herb Catalog'],
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

export default function TcmPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [herbs, setHerbs] = useState<Herb[]>([]);
  const [prescriptions] = useState<Prescription[]>([]);

  useEffect(() => {
    fetch('/api/v1/herbs').then(r => r.ok ? r.json() : [])
      .then(h => setHerbs(h ?? []))
      .catch(() => {});
  }, []);

  return (
    <AppShell title="Traditional Chinese Medicine">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Herb Catalog', value: herbs.length, border: 'border-primary-200' },
              { label: 'Prescriptions', value: prescriptions.length, border: 'border-green-200' },
              { label: 'Active Rxs', value: prescriptions.filter(p => p.status === 'active').length, border: 'border-amber-200' },
              { label: 'Herb Types', value: [...new Set(herbs.map(h => h.category))].length, border: 'border-secondary-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Traditional Chinese Medicine</h2>
            <p className="mt-2 text-sm text-secondary-500">
              TCM diagnosis uses tongue and pulse assessment to identify patterns of disharmony.
              Herbal prescriptions combine herbs in formulas (jun-chen-zuo-shi) dispensed as
              decoctions, granules, or pills. Private pay — no ACC or public funding.
            </p>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('diagnosis')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                Record Diagnosis
              </button>
              <button onClick={() => setActiveTab('prescriptions')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                New Prescription
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'diagnosis' && (
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
          <h2 className="mb-4 text-base font-semibold text-secondary-900">Tongue &amp; Pulse Diagnosis</h2>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
            <div className="rounded-lg border border-secondary-200 p-4">
              <h3 className="mb-3 text-sm font-semibold text-secondary-800">Tongue Assessment</h3>
              <div className="space-y-3">
                {[
                  { label: 'Body Colour', options: ['Pale', 'Normal', 'Red', 'Dark Red', 'Purple'] },
                  { label: 'Coating', options: ['Thin White', 'Thick White', 'Yellow', 'Grey', 'Black', 'Absent'] },
                  { label: 'Shape', options: ['Normal', 'Swollen', 'Thin', 'Cracked', 'Tooth-marked'] },
                ].map(({ label, options }) => (
                  <div key={label}>
                    <label className="block text-xs font-medium text-secondary-600">{label}</label>
                    <select className="mt-1 w-full rounded-md border border-secondary-300 px-2 py-1.5 text-sm focus:border-primary-500 focus:outline-none">
                      <option value="">Select…</option>
                      {options.map(o => <option key={o} value={o.toLowerCase().replace(' ', '_')}>{o}</option>)}
                    </select>
                  </div>
                ))}
              </div>
            </div>
            <div className="rounded-lg border border-secondary-200 p-4">
              <h3 className="mb-3 text-sm font-semibold text-secondary-800">Pulse Assessment</h3>
              <div className="space-y-3">
                {[
                  { label: 'Depth', options: ['Floating', 'Middle', 'Deep'] },
                  { label: 'Rate', options: ['Slow (<60)', 'Moderate (60–90)', 'Rapid (>90)'] },
                  { label: 'Quality', options: ['Slippery', 'Wiry', 'Thready', 'Choppy', 'Full', 'Hollow', 'Tight'] },
                ].map(({ label, options }) => (
                  <div key={label}>
                    <label className="block text-xs font-medium text-secondary-600">{label}</label>
                    <select className="mt-1 w-full rounded-md border border-secondary-300 px-2 py-1.5 text-sm focus:border-primary-500 focus:outline-none">
                      <option value="">Select…</option>
                      {options.map(o => <option key={o} value={o.toLowerCase()}>{o}</option>)}
                    </select>
                  </div>
                ))}
              </div>
            </div>
          </div>
          <div className="mt-4">
            <label className="block text-sm font-medium text-secondary-700">TCM Pattern Differentiation</label>
            <textarea rows={2} placeholder="e.g. Liver Qi Stagnation with Blood Deficiency…"
              className="mt-1 w-full rounded-md border border-secondary-300 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500" />
          </div>
          <button className="mt-4 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
            Save Diagnosis
          </button>
        </div>
      )}

      {activeTab === 'prescriptions' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Herbal Prescriptions</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Prescription
            </button>
          </div>
          {prescriptions.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No prescriptions.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Formula Name</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {prescriptions.map(p => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{p.patientNhi}</td>
                    <td className="px-6 py-3">{p.name}</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        p.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-secondary-100 text-secondary-700'
                      }`}>{p.status}</span>
                    </td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(p.createdAt).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'herbs' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Herb Catalog</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Add Herb
            </button>
          </div>
          {herbs.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No herbs in catalog.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Pinyin</th>
                  <th className="px-6 py-3 text-left">English Name</th>
                  <th className="px-6 py-3 text-left">Category</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {herbs.map(h => (
                  <tr key={h.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-medium italic">{h.pinYin}</td>
                    <td className="px-6 py-3">{h.englishName}</td>
                    <td className="px-6 py-3 capitalize text-secondary-500">{h.category.replace('_', ' ')}</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${h.active ? 'bg-green-100 text-green-800' : 'bg-secondary-100 text-secondary-700'}`}>
                        {h.active ? 'Active' : 'Inactive'}
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
