import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'spinal-chart' | 'xray-referrals' | 'acc-claims';

interface ACCClaim { id: string; patientNhi: string; status: string; region: string }
interface XRayReferral { id: string; patientNhi: string; region: string; urgency: string; status: string }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: { id: Tab; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'spinal-chart', label: 'Spinal Chart' },
    { id: 'xray-referrals', label: 'X-Ray Referrals' },
    { id: 'acc-claims', label: 'ACC Claims' },
  ];
  return (
    <div className="mb-6 flex gap-1 rounded-lg bg-secondary-100 p-1">
      {tabs.map(t => (
        <button key={t.id} onClick={() => onSelect(t.id)}
          className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === t.id ? 'bg-white text-primary-700 shadow-sm' : 'text-secondary-600 hover:text-secondary-900'
          }`}>
          {t.label}
        </button>
      ))}
    </div>
  );
}

export default function ChiropracticPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [claims, setClaims] = useState<ACCClaim[]>([]);
  const [referrals, setReferrals] = useState<XRayReferral[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      fetch('/api/v1/acc/claims').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/xray-referrals').then(r => r.ok ? r.json() : []),
    ])
      .then(([c, x]) => { setClaims(c ?? []); setReferrals(x ?? []); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  return (
    <AppShell title="Chiropractic">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'ACC Claims', value: claims.length, border: 'border-primary-200' },
              { label: 'X-Ray Referrals', value: referrals.length, border: 'border-blue-200' },
              { label: 'Pending Referrals', value: referrals.filter(r => r.status === 'ordered').length, border: 'border-amber-200' },
              { label: 'Accepted Claims', value: claims.filter(c => c.status === 'accepted').length, border: 'border-green-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Chiropractic Management</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Document spinal segment assessments, lodge ACC claims for injury treatment, and refer for
              X-ray imaging. Vertebral subluxation and fixation findings are recorded per spinal segment.
            </p>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('spinal-chart')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                Open Spinal Chart
              </button>
              <button onClick={() => setActiveTab('xray-referrals')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                New X-Ray Referral
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'spinal-chart' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Spinal Segment Chart</h2>
            <p className="text-xs text-secondary-400">Select a patient to load their chart</p>
          </div>
          <div className="p-6">
            <div className="mb-4">
              <label className="block text-sm font-medium text-secondary-700">Patient NHI</label>
              <input
                type="text"
                placeholder="e.g. ABC1234"
                className="mt-1 w-64 rounded-md border border-secondary-300 px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
              />
            </div>
            <div className="grid grid-cols-3 gap-3">
              {['C1','C2','C3','C4','C5','C6','C7','T1','T2','T3','T4','T5','T6','T7','T8','T9','T10','T11','T12','L1','L2','L3','L4','L5','S1'].map(seg => (
                <button key={seg}
                  className="rounded-md border border-secondary-200 bg-secondary-50 px-3 py-2 text-center text-xs font-medium text-secondary-700 hover:border-primary-300 hover:bg-primary-50">
                  {seg}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}

      {activeTab === 'xray-referrals' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">X-Ray Referrals</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Referral
            </button>
          </div>
          {referrals.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No X-ray referrals.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Region</th>
                  <th className="px-6 py-3 text-left">Urgency</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {referrals.map(ref => (
                  <tr key={ref.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{ref.patientNhi}</td>
                    <td className="px-6 py-3 capitalize">{ref.region.replace('_', ' ')}</td>
                    <td className="px-6 py-3 capitalize">{ref.urgency}</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        ref.status === 'reported' ? 'bg-green-100 text-green-800' :
                        ref.status === 'completed' ? 'bg-blue-100 text-blue-800' :
                        'bg-amber-100 text-amber-800'
                      }`}>{ref.status}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
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
          {loading ? (
            <div className="flex h-32 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
            </div>
          ) : claims.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No ACC claims.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient</th>
                  <th className="px-6 py-3 text-left">Region</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {claims.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.patientNhi}</td>
                    <td className="px-6 py-3 capitalize">{c.region}</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        c.status === 'accepted' ? 'bg-green-100 text-green-800' :
                        c.status === 'submitted' ? 'bg-blue-100 text-blue-800' :
                        c.status === 'declined' ? 'bg-red-100 text-red-800' :
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
