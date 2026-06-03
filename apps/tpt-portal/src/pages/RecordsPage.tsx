import { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';

type RecordTab = 'encounters' | 'diagnoses' | 'immunisations';

interface Encounter {
  id: string;
  date: string;
  type: string;
  practitioner: string;
  location: string;
  summary: string;
}

interface Diagnosis {
  id: string;
  code: string;        // ICD-10-AM code
  display: string;
  onsetDate: string;
  status: 'active' | 'resolved' | 'inactive';
  practitioner: string;
}

interface Immunisation {
  id: string;
  vaccine: string;
  vaccineCode: string; // NZMT code
  date: string;
  lot: string;
  site: string;
  practitioner: string;
}

// Stub FHIR R5 data — will be fetched via @tpt/api-client FHIR endpoints
const encounters: Encounter[] = [
  {
    id: 'enc-1',
    date: '2026-05-05',
    type: 'Ambulatory',
    practitioner: 'Dr. Hemi Walker',
    location: 'Auckland City Medical Centre',
    summary: 'HbA1c follow-up. Patient reports good adherence to medication. No new concerns. Annual bloods reviewed.',
  },
  {
    id: 'enc-2',
    date: '2026-04-12',
    type: 'Ambulatory',
    practitioner: 'Dr. Hemi Walker',
    location: 'Auckland City Medical Centre',
    summary: 'Blood pressure well controlled on current regimen. Annual bloods ordered. Lifestyle advice given.',
  },
  {
    id: 'enc-3',
    date: '2025-11-20',
    type: 'Ambulatory',
    practitioner: 'Dr. Hemi Walker',
    location: 'Auckland City Medical Centre',
    summary: 'Acute presentation with upper respiratory tract infection. Symptomatic treatment recommended.',
  },
];

const diagnoses: Diagnosis[] = [
  {
    id: 'cond-1',
    code: 'E11',
    display: 'Type 2 diabetes mellitus',
    onsetDate: '2023-08-15',
    status: 'active',
    practitioner: 'Dr. Hemi Walker',
  },
  {
    id: 'cond-2',
    code: 'I10',
    display: 'Essential (primary) hypertension',
    onsetDate: '2022-03-01',
    status: 'active',
    practitioner: 'Dr. Hemi Walker',
  },
  {
    id: 'cond-3',
    code: 'J06.9',
    display: 'Acute upper respiratory infection, unspecified',
    onsetDate: '2025-11-20',
    status: 'resolved',
    practitioner: 'Dr. Hemi Walker',
  },
];

const immunisations: Immunisation[] = [
  {
    id: 'imm-1',
    vaccine: 'Influenza vaccine (Influvac Tetra)',
    vaccineCode: '44395601000116108',
    date: '2026-04-20',
    lot: 'FLU2026A',
    site: 'Left deltoid',
    practitioner: 'Nurse Mere Parata',
  },
  {
    id: 'imm-2',
    vaccine: 'COVID-19 XBB.1.5 booster (Comirnaty)',
    vaccineCode: '1841241000168102',
    date: '2025-10-05',
    lot: 'CV25B-NZ',
    site: 'Right deltoid',
    practitioner: 'Nurse Mere Parata',
  },
  {
    id: 'imm-3',
    vaccine: 'Tdap (Boostrix)',
    vaccineCode: '34692011000036108',
    date: '2023-03-14',
    lot: 'T23-048',
    site: 'Left deltoid',
    practitioner: 'Dr. Hemi Walker',
  },
];

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-NZ', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

function diagnosisStatusBadge(status: Diagnosis['status']) {
  const map = {
    active: 'bg-red-100 text-red-700',
    resolved: 'bg-green-100 text-green-700',
    inactive: 'bg-gray-100 text-gray-500',
  };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${map[status]}`}>
      {status}
    </span>
  );
}

const tabs: { key: RecordTab; label: string }[] = [
  { key: 'encounters', label: 'Encounters' },
  { key: 'diagnoses', label: 'Diagnoses' },
  { key: 'immunisations', label: 'Immunisations' },
];

export function RecordsPage() {
  const { user } = useAuth();
  const [tab, setTab] = useState<RecordTab>('encounters');

  return (
    <div className="p-6 max-w-4xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">My Health Records</h1>
        <p className="mt-1 text-sm text-gray-500">
          A read-only view of your health records held by your care team.
        </p>
      </div>

      {/* NHI banner */}
      <div className="bg-brand-50 border border-brand-200 rounded-xl px-5 py-4 mb-6 flex items-center gap-4">
        <div className="h-10 w-10 rounded-full bg-brand-600 flex items-center justify-center flex-shrink-0">
          <svg className="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15 9h3.75M15 12h3.75M15 15h3.75M4.5 19.5h15a2.25 2.25 0 0 0 2.25-2.25V6.75A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25v10.5A2.25 2.25 0 0 0 4.5 19.5Zm6-10.125a1.875 1.875 0 1 1-3.75 0 1.875 1.875 0 0 1 3.75 0Zm1.294 6.336a6.721 6.721 0 0 1-3.17.789 6.721 6.721 0 0 1-3.168-.789 3.376 3.376 0 0 1 6.338 0Z" />
          </svg>
        </div>
        <div>
          <p className="text-xs font-medium text-brand-700 uppercase tracking-wide">National Health Index (NHI)</p>
          <p className="text-lg font-bold text-brand-900">{user?.nhi}</p>
          <p className="text-xs text-brand-600">
            {user?.givenName} {user?.familyName} &mdash; DOB {formatDate(user?.dateOfBirth ?? '')}
          </p>
        </div>
      </div>

      {/* Retention note */}
      <div className="bg-amber-50 border border-amber-200 rounded-xl px-4 py-3 mb-6">
        <p className="text-xs text-amber-800">
          <span className="font-semibold">HIPC Rule 6 &mdash; Data Retention:</span> Your health records are retained
          for a minimum of 10 years from the date of last entry, as required by the Health Information Privacy Code
          2020. You may request a copy of your records at any time under the Privacy Act 2020.
        </p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-gray-100 rounded-lg p-1 w-fit mb-5">
        {tabs.map(t => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
              tab === t.key ? 'bg-white shadow-sm text-gray-900' : 'text-gray-500 hover:text-gray-700'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Encounters */}
      {tab === 'encounters' && (
        <div className="space-y-3">
          {encounters.map(enc => (
            <div key={enc.id} className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
              <div className="flex items-start justify-between mb-2">
                <div>
                  <p className="text-sm font-semibold text-gray-900">{enc.practitioner}</p>
                  <p className="text-xs text-gray-500">{enc.type} &mdash; {enc.location}</p>
                </div>
                <p className="text-xs font-medium text-brand-600 flex-shrink-0 ml-4">
                  {formatDate(enc.date)}
                </p>
              </div>
              <p className="text-sm text-gray-600 leading-relaxed">{enc.summary}</p>
            </div>
          ))}
        </div>
      )}

      {/* Diagnoses */}
      {tab === 'diagnoses' && (
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-100 bg-gray-50">
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Condition</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">ICD-10 Code</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Onset</th>
                <th className="px-5 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wide">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {diagnoses.map(diag => (
                <tr key={diag.id}>
                  <td className="px-5 py-4">
                    <p className="font-medium text-gray-900">{diag.display}</p>
                    <p className="text-xs text-gray-400 mt-0.5">Recorded by {diag.practitioner}</p>
                  </td>
                  <td className="px-5 py-4">
                    <code className="text-xs bg-gray-100 rounded px-1.5 py-0.5 text-gray-700">{diag.code}</code>
                  </td>
                  <td className="px-5 py-4 text-gray-600">{formatDate(diag.onsetDate)}</td>
                  <td className="px-5 py-4">{diagnosisStatusBadge(diag.status)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Immunisations */}
      {tab === 'immunisations' && (
        <div className="space-y-3">
          {immunisations.map(imm => (
            <div key={imm.id} className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
              <div className="flex items-start justify-between">
                <div>
                  <p className="text-sm font-semibold text-gray-900">{imm.vaccine}</p>
                  <p className="text-xs text-gray-400 mt-0.5">
                    NZMT: <code className="bg-gray-100 rounded px-1">{imm.vaccineCode}</code>
                  </p>
                  <div className="flex gap-4 mt-2">
                    <p className="text-xs text-gray-500">Site: {imm.site}</p>
                    <p className="text-xs text-gray-500">Lot: {imm.lot}</p>
                    <p className="text-xs text-gray-500">By: {imm.practitioner}</p>
                  </div>
                </div>
                <p className="text-xs font-medium text-brand-600 flex-shrink-0 ml-4">
                  {formatDate(imm.date)}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
