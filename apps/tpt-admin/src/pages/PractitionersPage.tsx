import { useState } from 'react';

interface Practitioner {
  id: string;
  name: string;
  hpiCpn: string;        // HPI Common Person Number
  registrationAuthority: string;
  scope: string;
  apcStatus: 'current' | 'expiring_soon' | 'expired';
  apcExpiry: string;
  type: string;
  enrolledPatients: number;
  activeToday: boolean;
}

const practitioners: Practitioner[] = [
  {
    id: 'prac-1',
    name: 'Dr. Hemi Walker',
    hpiCpn: '27ZZWN',
    registrationAuthority: 'Medical Council of New Zealand',
    scope: 'General Practice',
    apcStatus: 'current',
    apcExpiry: '2027-03-31',
    type: 'General Practitioner',
    enrolledPatients: 842,
    activeToday: true,
  },
  {
    id: 'prac-2',
    name: 'Dr. Piripi Te Aho',
    hpiCpn: '27ZZWP',
    registrationAuthority: 'Medical Council of New Zealand',
    scope: 'General Practice',
    apcStatus: 'current',
    apcExpiry: '2027-03-31',
    type: 'General Practitioner',
    enrolledPatients: 756,
    activeToday: true,
  },
  {
    id: 'prac-3',
    name: 'Nurse Mere Parata',
    hpiCpn: '27ZZWQ',
    registrationAuthority: 'Nursing Council of New Zealand',
    scope: 'Nurse Practitioner — Primary Care',
    apcStatus: 'current',
    apcExpiry: '2026-09-30',
    type: 'Nurse Practitioner',
    enrolledPatients: 0,
    activeToday: true,
  },
  {
    id: 'prac-4',
    name: 'Dr. Sione Tuilagi',
    hpiCpn: '27ZZWR',
    registrationAuthority: 'Medical Council of New Zealand',
    scope: 'Internal Medicine',
    apcStatus: 'expiring_soon',
    apcExpiry: '2026-06-30',
    type: 'Specialist',
    enrolledPatients: 312,
    activeToday: false,
  },
];

function apcBadge(status: Practitioner['apcStatus']) {
  const map = {
    current: 'bg-green-100 text-green-700',
    expiring_soon: 'bg-amber-100 text-amber-700',
    expired: 'bg-red-100 text-red-700',
  };
  const labels = { current: 'APC Current', expiring_soon: 'Expiring Soon', expired: 'APC Expired' };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium ${map[status]}`}>
      {labels[status]}
    </span>
  );
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-NZ', { day: 'numeric', month: 'short', year: 'numeric' });
}

export function PractitionersPage() {
  const [search, setSearch] = useState('');

  const filtered = practitioners.filter(p =>
    p.name.toLowerCase().includes(search.toLowerCase()) ||
    p.hpiCpn.toLowerCase().includes(search.toLowerCase()) ||
    p.type.toLowerCase().includes(search.toLowerCase()),
  );

  const expiringSoon = practitioners.filter(p => p.apcStatus === 'expiring_soon' || p.apcStatus === 'expired');

  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Practitioners</h1>
          <p className="mt-1 text-sm text-gray-500">
            Manage HPI-registered practitioners and APC status.
          </p>
        </div>
      </div>

      {/* APC alerts */}
      {expiringSoon.length > 0 && (
        <div className="bg-amber-50 border border-amber-200 rounded-xl px-5 py-4 mb-6">
          <p className="text-sm font-semibold text-amber-800 mb-1">APC Action Required</p>
          {expiringSoon.map(p => (
            <p key={p.id} className="text-xs text-amber-700">
              {p.name} ({p.hpiCpn}) — APC {p.apcStatus === 'expired' ? 'expired' : 'expiring'} {formatDate(p.apcExpiry)}. Clinical actions are {p.apcStatus === 'expired' ? 'blocked' : 'at risk'}.
            </p>
          ))}
        </div>
      )}

      {/* HPCA note */}
      <div className="bg-blue-50 border border-blue-200 rounded-xl px-4 py-3 mb-6">
        <p className="text-xs text-blue-800">
          <span className="font-semibold">HPCA Act 2003:</span> Clinical actions in TPT Healthcare are only available to
          practitioners with a valid APC and appropriate scope of practice as validated against the HPI. APC status is
          cached for 24 hours (cache key: <code className="font-mono bg-blue-100 rounded px-1">hpi:apc:&#123;cpn&#125;</code>).
        </p>
      </div>

      {/* Search */}
      <div className="mb-4">
        <input
          type="search"
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="Search by name, HPI CPN, or type..."
          className="w-full max-w-sm rounded-lg border border-gray-300 px-3.5 py-2 text-sm focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/20"
        />
      </div>

      {/* Table */}
      <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-100 bg-gray-50">
              <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Practitioner</th>
              <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">HPI CPN</th>
              <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Registration Authority</th>
              <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Scope</th>
              <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">APC Expiry</th>
              <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</th>
              <th className="px-5 py-3 text-right text-xs font-semibold text-gray-500 uppercase tracking-wide">Patients</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {filtered.map(p => (
              <tr key={p.id} className="hover:bg-gray-50">
                <td className="px-5 py-4">
                  <div className="flex items-center gap-2">
                    <div className={`h-2 w-2 rounded-full flex-shrink-0 ${p.activeToday ? 'bg-green-400' : 'bg-gray-300'}`} title={p.activeToday ? 'Active today' : 'Not active today'} />
                    <div>
                      <p className="font-medium text-gray-900">{p.name}</p>
                      <p className="text-xs text-gray-400">{p.type}</p>
                    </div>
                  </div>
                </td>
                <td className="px-5 py-4">
                  <code className="text-xs bg-gray-100 rounded px-1.5 py-0.5 text-gray-700">{p.hpiCpn}</code>
                </td>
                <td className="px-5 py-4 text-xs text-gray-600 max-w-[160px] truncate">
                  {p.registrationAuthority}
                </td>
                <td className="px-5 py-4 text-xs text-gray-600">{p.scope}</td>
                <td className="px-5 py-4 text-xs text-gray-600">{formatDate(p.apcExpiry)}</td>
                <td className="px-5 py-4">{apcBadge(p.apcStatus)}</td>
                <td className="px-5 py-4 text-right text-sm text-gray-700">
                  {p.enrolledPatients > 0 ? p.enrolledPatients.toLocaleString('en-NZ') : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
