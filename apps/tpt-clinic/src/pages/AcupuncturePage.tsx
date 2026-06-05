import { useState, useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'needle-sites' | 'treatments' | 'acc-claims';

interface StatCard { label: string; value: number; color: string }
interface ACCClaim { id: string; patientNhi: string; status: string; createdAt: string }
interface NeedleSession { id: string; patientNhi: string; retentionMin: number; createdAt: number }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: { id: Tab; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'needle-sites', label: 'Needle Sites' },
    { id: 'treatments', label: 'Treatments' },
    { id: 'acc-claims', label: 'ACC Claims' },
  ];
  return (
    <div className="mb-6 flex gap-1 rounded-lg bg-secondary-100 p-1">
      {tabs.map(t => (
        <button
          key={t.id}
          onClick={() => onSelect(t.id)}
          className={`flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === t.id
              ? 'bg-white text-primary-700 shadow-sm'
              : 'text-secondary-600 hover:text-secondary-900'
          }`}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

function StatCards({ stats }: { stats: StatCard[] }) {
  const borderColors: Record<string, string> = {
    primary: 'border-primary-200',
    green: 'border-green-200',
    amber: 'border-amber-200',
    secondary: 'border-secondary-200',
  };
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {stats.map(s => (
        <div key={s.label} className={`rounded-xl border ${borderColors[s.color] ?? 'border-secondary-200'} bg-white p-5 shadow-sm`}>
          <p className="text-sm font-medium text-secondary-500">{s.label}</p>
          <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
        </div>
      ))}
    </div>
  );
}

export default function AcupuncturePage() {
  const location = useLocation();
  const initialTab: Tab = (location.hash.replace('#', '') as Tab) || 'overview';
  const [activeTab, setActiveTab] = useState<Tab>(initialTab);
  const [claims, setClaims] = useState<ACCClaim[]>([]);
  const [sessions, setSessions] = useState<NeedleSession[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    Promise.all([
      fetch('/api/v1/acc/claims').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/needle-sites').then(r => r.ok ? r.json() : []),
    ])
      .then(([c, s]) => { setClaims(c ?? []); setSessions(s ?? []); })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const stats: StatCard[] = [
    { label: 'ACC Claims', value: claims.length, color: 'primary' },
    { label: 'Needle Sessions', value: sessions.length, color: 'green' },
    { label: 'Active Claims', value: claims.filter(c => c.status === 'submitted').length, color: 'amber' },
    { label: 'Accepted', value: claims.filter(c => c.status === 'accepted').length, color: 'secondary' },
  ];

  return (
    <AppShell title="Acupuncture">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <StatCards stats={stats} />
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">ACC-Funded Acupuncture</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Acupuncture is funded by ACC for specified injury-related conditions under the treatment
              provider schedule. Use the Needle Sites tab to document sessions and the ACC Claims tab to
              lodge claims with meridian and diagnosis coding.
            </p>
            <div className="mt-4 flex gap-3">
              <button
                onClick={() => setActiveTab('needle-sites')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700"
              >
                New Session
              </button>
              <button
                onClick={() => setActiveTab('acc-claims')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50"
              >
                New ACC Claim
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'needle-sites' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Needle Site Documentation</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Session
            </button>
          </div>
          {loading ? (
            <div className="flex h-32 items-center justify-center">
              <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
            </div>
          ) : sessions.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-2 text-secondary-400">
              <p className="text-sm">No needle sessions recorded.</p>
              <p className="text-xs">Select a patient and start a new session.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Retention (min)</th>
                  <th className="px-6 py-3 text-left">Date</th>
                  <th className="px-6 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {sessions.map(s => (
                  <tr key={s.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{s.patientNhi}</td>
                    <td className="px-6 py-3">{s.retentionMin}</td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(s.createdAt).toLocaleDateString('en-NZ')}</td>
                    <td className="px-6 py-3 text-right">
                      <Link to={`/acupuncture/needle-sites/${s.id}`} className="text-primary-600 hover:underline">View</Link>
                    </td>
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
          <div className="flex h-32 flex-col items-center justify-center gap-2 text-secondary-400">
            <p className="text-sm">No treatment records.</p>
          </div>
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
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">Created</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {claims.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.patientNhi}</td>
                    <td className="px-6 py-3">
                      <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        c.status === 'accepted' ? 'bg-green-100 text-green-800' :
                        c.status === 'submitted' ? 'bg-blue-100 text-blue-800' :
                        c.status === 'declined' ? 'bg-red-100 text-red-800' :
                        'bg-secondary-100 text-secondary-700'
                      }`}>{c.status}</span>
                    </td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(c.createdAt).toLocaleDateString('en-NZ')}</td>
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
