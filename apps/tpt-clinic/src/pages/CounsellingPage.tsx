import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'sessions' | 'eap' | 'private';

interface Session {
  id: string; clientNhi: string; modality: string; billingType: string;
  sessionDate: number; durationMin: number;
}
interface EAPClaim { id: string; clientNhi: string; sessions: number; status: string }
interface PrivateClient { id: string; name: string; active: boolean }

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'], ['sessions', 'Session Notes'], ['eap', 'EAP Billing'], ['private', 'Private Practice'],
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

export default function CounsellingPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [sessions] = useState<Session[]>([]);
  const [eapClaims, setEAPClaims] = useState<EAPClaim[]>([]);
  const [clients, setClients] = useState<PrivateClient[]>([]);

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/eap/claims').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/private/clients').then(r => r.ok ? r.json() : []),
    ])
      .then(([e, c]) => { setEAPClaims(e ?? []); setClients(c ?? []); })
      .catch(() => {});
  }, []);

  const billingBadge = (type: string) => {
    const map: Record<string, string> = {
      eap: 'bg-blue-100 text-blue-800',
      private: 'bg-purple-100 text-purple-800',
      acc: 'bg-green-100 text-green-800',
      pro_bono: 'bg-secondary-100 text-secondary-700',
    };
    return map[type] ?? 'bg-secondary-100 text-secondary-700';
  };

  return (
    <AppShell title="Counselling">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Sessions', value: sessions.length, border: 'border-primary-200' },
              { label: 'EAP Claims', value: eapClaims.length, border: 'border-blue-200' },
              { label: 'Private Clients', value: clients.filter(c => c.active).length, border: 'border-purple-200' },
              { label: 'This Month', value: sessions.filter(s => new Date(s.sessionDate).getMonth() === new Date().getMonth()).length, border: 'border-secondary-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="text-lg font-semibold text-secondary-900">Counselling & Psychotherapy</h2>
            <p className="mt-2 text-sm text-secondary-500">
              Session notes are classified as mental health records under HIPC and require elevated
              disclosure consent before access. Supports EAP billing, private practice invoicing,
              and ACC-funded counselling for injury-related mental health.
            </p>
            <div className="mt-3 rounded-md bg-amber-50 border border-amber-200 px-4 py-3">
              <p className="text-xs font-medium text-amber-800">
                Privacy Notice: Session notes are extra-sensitive mental health records.
                Patient disclosure consent is required before viewing or sharing these records (HIPC Rule 11).
              </p>
            </div>
            <div className="mt-4 flex gap-3">
              <button onClick={() => setActiveTab('sessions')}
                className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                New Session Note
              </button>
              <button onClick={() => setActiveTab('eap')}
                className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                New EAP Claim
              </button>
            </div>
          </div>
        </>
      )}

      {activeTab === 'sessions' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Session Notes</h2>
              <p className="text-xs text-amber-700 mt-0.5">Mental health records — elevated consent required</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Session
            </button>
          </div>
          {sessions.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No sessions recorded.</p>
              <p className="text-xs">Select a patient to start a new session note.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Client NHI</th>
                  <th className="px-6 py-3 text-left">Modality</th>
                  <th className="px-6 py-3 text-left">Duration</th>
                  <th className="px-6 py-3 text-left">Billing</th>
                  <th className="px-6 py-3 text-left">Date</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {sessions.map(s => (
                  <tr key={s.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{s.clientNhi}</td>
                    <td className="px-6 py-3 capitalize">{s.modality.replace('_', ' ')}</td>
                    <td className="px-6 py-3">{s.durationMin} min</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${billingBadge(s.billingType)}`}>
                        {s.billingType.replace('_', ' ')}
                      </span>
                    </td>
                    <td className="px-6 py-3 text-secondary-500">{new Date(s.sessionDate).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'eap' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">EAP Claims</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New EAP Claim
            </button>
          </div>
          {eapClaims.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No EAP claims.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Client</th>
                  <th className="px-6 py-3 text-left">Sessions</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {eapClaims.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.clientNhi}</td>
                    <td className="px-6 py-3">{c.sessions}</td>
                    <td className="px-6 py-3 capitalize">
                      <span className="rounded-full bg-secondary-100 px-2.5 py-0.5 text-xs font-medium text-secondary-700">{c.status}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'private' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <h2 className="text-base font-semibold text-secondary-900">Private Practice Clients</h2>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Client
            </button>
          </div>
          {clients.length === 0 ? (
            <div className="flex h-32 items-center justify-center text-sm text-secondary-400">No private clients.</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Name</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {clients.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3">{c.name}</td>
                    <td className="px-6 py-3">
                      <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${c.active ? 'bg-green-100 text-green-800' : 'bg-secondary-100 text-secondary-700'}`}>
                        {c.active ? 'Active' : 'Inactive'}
                      </span>
                    </td>
                    <td className="px-6 py-3 text-right">
                      <button className="text-sm text-primary-600 hover:underline">Invoice</button>
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
