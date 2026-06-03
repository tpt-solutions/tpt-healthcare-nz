import React, { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface PatientDetail {
  id: string;
  nhi: string;
  nhiDisplay: string;
  name: string;
  dateOfBirth: string;
  gender: string;
  address: string;
  phone?: string;
  email?: string;
  ethnicity?: string;
  enrolledPractice: string;
  gpName?: string;
  allergies: string[];
  alerts: string[];
}

interface Encounter {
  id: string;
  date: string;
  reason: string;
  notes: string;
  practitionerName: string;
  status: string;
}

interface Prescription {
  id: string;
  medicationName: string;
  dose: string;
  frequency: string;
  startDate: string;
  endDate?: string;
  prescriber: string;
  status: 'active' | 'stopped' | 'completed';
}

interface Immunisation {
  id: string;
  vaccineDisplay: string;
  occurrenceDate: string;
  lotNumber?: string;
  practitioner: string;
  status: string;
}

interface AccClaim {
  id: string;
  claimNumber: string;
  injuryDate: string;
  description: string;
  status: 'active' | 'closed' | 'pending';
  lastUpdated: string;
}

interface NesEnrolment {
  id: string;
  practice: string;
  startDate: string;
  endDate?: string;
  status: 'active' | 'inactive';
  funder: string;
}

// ---------------------------------------------------------------------------
// Tab definitions
// ---------------------------------------------------------------------------

type TabId = 'overview' | 'encounters' | 'prescriptions' | 'immunisations' | 'acc' | 'nes';

const TABS: { id: TabId; label: string }[] = [
  { id: 'overview',      label: 'Overview' },
  { id: 'encounters',    label: 'Encounters' },
  { id: 'prescriptions', label: 'Prescriptions' },
  { id: 'immunisations', label: 'Immunisations' },
  { id: 'acc',           label: 'ACC Claims' },
  { id: 'nes',           label: 'NES Enrolment' },
];

// ---------------------------------------------------------------------------
// Patient banner
// ---------------------------------------------------------------------------

function PatientBanner({ patient }: { patient: PatientDetail }) {
  const age = getAge(patient.dateOfBirth);
  return (
    <div className="mb-6 overflow-hidden rounded-xl bg-secondary-800 text-white shadow-md">
      <div className="flex flex-wrap items-start gap-4 px-6 py-5">
        {/* Avatar */}
        <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-full bg-primary-600 text-xl font-bold">
          {patient.name.charAt(0).toUpperCase()}
        </div>

        {/* Core details */}
        <div className="flex-1 min-w-0">
          <h2 className="text-xl font-semibold">{patient.name}</h2>
          <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-sm text-secondary-300">
            <span>
              <span className="font-medium text-secondary-100">NHI</span>{' '}
              <span className="font-mono">{patient.nhiDisplay}</span>
            </span>
            <span>
              {new Date(patient.dateOfBirth).toLocaleDateString('en-NZ')}{' '}
              <span className="text-secondary-400">({age} yrs)</span>
            </span>
            <span>{patient.gender}</span>
            {patient.ethnicity && <span>{patient.ethnicity}</span>}
          </div>
          <div className="mt-1 text-sm text-secondary-300">
            {patient.address}
          </div>
        </div>

        {/* Contact + practice */}
        <div className="text-right text-sm text-secondary-300">
          {patient.phone && <div>{patient.phone}</div>}
          {patient.email && <div>{patient.email}</div>}
          <div className="mt-1 text-secondary-400">{patient.enrolledPractice}</div>
          {patient.gpName && <div className="text-secondary-400">{patient.gpName}</div>}
        </div>
      </div>

      {/* Allergy / alert strip */}
      {(patient.allergies.length > 0 || patient.alerts.length > 0) && (
        <div className="flex flex-wrap items-center gap-2 border-t border-secondary-700 bg-secondary-900/50 px-6 py-2">
          {patient.allergies.map((a) => (
            <span key={a} className="badge-urgent">{a}</span>
          ))}
          {patient.alerts.map((al) => (
            <span key={al} className="badge-warning">{al}</span>
          ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab panels
// ---------------------------------------------------------------------------

function OverviewPanel({ patient }: { patient: PatientDetail }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <InfoCard label="Contact">
        <Row k="Phone" v={patient.phone ?? '—'} />
        <Row k="Email" v={patient.email ?? '—'} />
        <Row k="Address" v={patient.address} />
      </InfoCard>
      <InfoCard label="Practice">
        <Row k="Enrolled Practice" v={patient.enrolledPractice} />
        <Row k="GP" v={patient.gpName ?? '—'} />
      </InfoCard>
      <InfoCard label="Allergies">
        {patient.allergies.length === 0 ? (
          <p className="text-sm text-secondary-500">No known allergies recorded.</p>
        ) : (
          <ul className="list-inside list-disc space-y-1 text-sm text-secondary-700">
            {patient.allergies.map((a) => <li key={a}>{a}</li>)}
          </ul>
        )}
      </InfoCard>
      <InfoCard label="Clinical Alerts">
        {patient.alerts.length === 0 ? (
          <p className="text-sm text-secondary-500">No alerts.</p>
        ) : (
          <ul className="list-inside list-disc space-y-1 text-sm text-secondary-700">
            {patient.alerts.map((al) => <li key={al}>{al}</li>)}
          </ul>
        )}
      </InfoCard>
    </div>
  );
}

function InfoCard({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg bg-white p-4 shadow-sm ring-1 ring-secondary-200">
      <h4 className="mb-3 text-xs font-semibold uppercase tracking-wide text-secondary-500">{label}</h4>
      {children}
    </div>
  );
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex items-baseline justify-between py-0.5 text-sm">
      <span className="text-secondary-500">{k}</span>
      <span className="font-medium text-secondary-800">{v}</span>
    </div>
  );
}

function EncountersPanel({ patientId }: { patientId: string }) {
  const api = useApi();
  const [encounters, setEncounters] = useState<Encounter[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void api.get<{ encounters: Encounter[] }>(`/patients/${patientId}/encounters`)
      .then((d) => { if (!cancelled) setEncounters(d.encounters); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [api, patientId]);

  if (loading) return <Spinner />;
  if (encounters.length === 0) return <EmptyState message="No encounters recorded." />;

  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <table className="w-full text-sm">
        <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
          <tr>
            <th className="px-4 py-3 text-left">Date</th>
            <th className="px-4 py-3 text-left">Reason</th>
            <th className="px-4 py-3 text-left">Practitioner</th>
            <th className="px-4 py-3 text-left">Status</th>
            <th className="px-4 py-3" />
          </tr>
        </thead>
        <tbody className="divide-y divide-secondary-100">
          {encounters.map((enc) => (
            <tr key={enc.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 text-secondary-600">
                {new Date(enc.date).toLocaleDateString('en-NZ')}
              </td>
              <td className="px-4 py-3 font-medium text-secondary-900">{enc.reason}</td>
              <td className="px-4 py-3 text-secondary-600">{enc.practitionerName}</td>
              <td className="px-4 py-3">
                <span className="badge-info">{enc.status}</span>
              </td>
              <td className="px-4 py-3">
                <Link
                  to={`/encounters/${enc.id}`}
                  className="font-medium text-primary-600 hover:underline"
                >
                  Open
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PrescriptionsPanel({ patientId }: { patientId: string }) {
  const api = useApi();
  const [prescriptions, setPrescriptions] = useState<Prescription[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void api.get<{ prescriptions: Prescription[] }>(`/patients/${patientId}/prescriptions`)
      .then((d) => { if (!cancelled) setPrescriptions(d.prescriptions); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [api, patientId]);

  const STATUS_CLASS: Record<Prescription['status'], string> = {
    active:    'badge-safe',
    stopped:   'badge-urgent',
    completed: 'inline-flex items-center rounded-full bg-secondary-100 px-2.5 py-0.5 text-xs font-medium text-secondary-600',
  };

  if (loading) return <Spinner />;
  if (prescriptions.length === 0) return <EmptyState message="No prescriptions recorded." />;

  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <table className="w-full text-sm">
        <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
          <tr>
            <th className="px-4 py-3 text-left">Medication</th>
            <th className="px-4 py-3 text-left">Dose / Frequency</th>
            <th className="px-4 py-3 text-left">Start</th>
            <th className="px-4 py-3 text-left">End</th>
            <th className="px-4 py-3 text-left">Prescriber</th>
            <th className="px-4 py-3 text-left">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-secondary-100">
          {prescriptions.map((rx) => (
            <tr key={rx.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-medium text-secondary-900">{rx.medicationName}</td>
              <td className="px-4 py-3 text-secondary-600">{rx.dose} — {rx.frequency}</td>
              <td className="px-4 py-3 text-secondary-600">
                {new Date(rx.startDate).toLocaleDateString('en-NZ')}
              </td>
              <td className="px-4 py-3 text-secondary-600">
                {rx.endDate ? new Date(rx.endDate).toLocaleDateString('en-NZ') : '—'}
              </td>
              <td className="px-4 py-3 text-secondary-600">{rx.prescriber}</td>
              <td className="px-4 py-3">
                <span className={STATUS_CLASS[rx.status]}>{rx.status}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ImmunisationsPanel({ patientId }: { patientId: string }) {
  const api = useApi();
  const [immunisations, setImmunisations] = useState<Immunisation[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void api.get<{ immunisations: Immunisation[] }>(`/patients/${patientId}/immunisations`)
      .then((d) => { if (!cancelled) setImmunisations(d.immunisations); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [api, patientId]);

  if (loading) return <Spinner />;
  if (immunisations.length === 0) return <EmptyState message="No immunisations recorded." />;

  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <table className="w-full text-sm">
        <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
          <tr>
            <th className="px-4 py-3 text-left">Vaccine</th>
            <th className="px-4 py-3 text-left">Date Given</th>
            <th className="px-4 py-3 text-left">Lot Number</th>
            <th className="px-4 py-3 text-left">Practitioner</th>
            <th className="px-4 py-3 text-left">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-secondary-100">
          {immunisations.map((imm) => (
            <tr key={imm.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-medium text-secondary-900">{imm.vaccineDisplay}</td>
              <td className="px-4 py-3 text-secondary-600">
                {new Date(imm.occurrenceDate).toLocaleDateString('en-NZ')}
              </td>
              <td className="px-4 py-3 font-mono text-secondary-600">{imm.lotNumber ?? '—'}</td>
              <td className="px-4 py-3 text-secondary-600">{imm.practitioner}</td>
              <td className="px-4 py-3">
                <span className="badge-safe">{imm.status}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function AccPanel({ patientId }: { patientId: string }) {
  const api = useApi();
  const [claims, setClaims] = useState<AccClaim[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void api.get<{ claims: AccClaim[] }>(`/patients/${patientId}/acc-claims`)
      .then((d) => { if (!cancelled) setClaims(d.claims); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [api, patientId]);

  const STATUS_CLASS: Record<AccClaim['status'], string> = {
    active:  'badge-safe',
    pending: 'badge-warning',
    closed:  'inline-flex items-center rounded-full bg-secondary-100 px-2.5 py-0.5 text-xs font-medium text-secondary-600',
  };

  if (loading) return <Spinner />;
  if (claims.length === 0) return <EmptyState message="No ACC claims found." />;

  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <table className="w-full text-sm">
        <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
          <tr>
            <th className="px-4 py-3 text-left">Claim Number</th>
            <th className="px-4 py-3 text-left">Injury Date</th>
            <th className="px-4 py-3 text-left">Description</th>
            <th className="px-4 py-3 text-left">Status</th>
            <th className="px-4 py-3 text-left">Last Updated</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-secondary-100">
          {claims.map((claim) => (
            <tr key={claim.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono font-medium text-secondary-900">{claim.claimNumber}</td>
              <td className="px-4 py-3 text-secondary-600">
                {new Date(claim.injuryDate).toLocaleDateString('en-NZ')}
              </td>
              <td className="px-4 py-3 text-secondary-700">{claim.description}</td>
              <td className="px-4 py-3">
                <span className={STATUS_CLASS[claim.status]}>{claim.status}</span>
              </td>
              <td className="px-4 py-3 text-secondary-500">
                {new Date(claim.lastUpdated).toLocaleDateString('en-NZ')}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function NesPanel({ patientId }: { patientId: string }) {
  const api = useApi();
  const [enrolments, setEnrolments] = useState<NesEnrolment[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void api.get<{ enrolments: NesEnrolment[] }>(`/patients/${patientId}/nes-enrolments`)
      .then((d) => { if (!cancelled) setEnrolments(d.enrolments); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [api, patientId]);

  if (loading) return <Spinner />;
  if (enrolments.length === 0) return <EmptyState message="No NES enrolments found." />;

  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <table className="w-full text-sm">
        <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
          <tr>
            <th className="px-4 py-3 text-left">Practice</th>
            <th className="px-4 py-3 text-left">Funder</th>
            <th className="px-4 py-3 text-left">Start Date</th>
            <th className="px-4 py-3 text-left">End Date</th>
            <th className="px-4 py-3 text-left">Status</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-secondary-100">
          {enrolments.map((enr) => (
            <tr key={enr.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-medium text-secondary-900">{enr.practice}</td>
              <td className="px-4 py-3 text-secondary-600">{enr.funder}</td>
              <td className="px-4 py-3 text-secondary-600">
                {new Date(enr.startDate).toLocaleDateString('en-NZ')}
              </td>
              <td className="px-4 py-3 text-secondary-600">
                {enr.endDate ? new Date(enr.endDate).toLocaleDateString('en-NZ') : '—'}
              </td>
              <td className="px-4 py-3">
                <span className={enr.status === 'active' ? 'badge-safe' : 'badge-warning'}>
                  {enr.status}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

function Spinner() {
  return (
    <div className="flex items-center justify-center py-12">
      <div className="h-7 w-7 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="rounded-xl bg-white px-4 py-10 text-center shadow-sm ring-1 ring-secondary-200">
      <p className="text-sm text-secondary-500">{message}</p>
    </div>
  );
}

function getAge(dob: string): number {
  const birth = new Date(dob);
  const today = new Date();
  let age = today.getFullYear() - birth.getFullYear();
  if (
    today.getMonth() < birth.getMonth() ||
    (today.getMonth() === birth.getMonth() && today.getDate() < birth.getDate())
  ) {
    age--;
  }
  return age;
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function PatientDetailPage() {
  const { id } = useParams<{ id: string }>();
  const api = useApi();

  const [patient, setPatient] = useState<PatientDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabId>('overview');

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    setLoading(true);
    void api.get<PatientDetail>(`/patients/${id}`)
      .then((d) => { if (!cancelled) setPatient(d); })
      .catch(() => { if (!cancelled) setError('Patient not found or access denied.'); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [api, id]);

  return (
    <AppShell title={patient?.name ?? 'Patient Detail'}>
      {loading && <Spinner />}

      {error && (
        <div className="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {patient && id && (
        <>
          <PatientBanner patient={patient} />

          {/* Tab bar */}
          <div className="mb-4 border-b border-secondary-200">
            <nav className="-mb-px flex gap-0 overflow-x-auto">
              {TABS.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={[
                    'whitespace-nowrap border-b-2 px-4 py-2.5 text-sm font-medium transition-colors',
                    activeTab === tab.id
                      ? 'border-primary-600 text-primary-700'
                      : 'border-transparent text-secondary-500 hover:border-secondary-300 hover:text-secondary-700',
                  ].join(' ')}
                >
                  {tab.label}
                </button>
              ))}
            </nav>
          </div>

          {/* Tab content */}
          <div>
            {activeTab === 'overview'      && <OverviewPanel patient={patient} />}
            {activeTab === 'encounters'    && <EncountersPanel patientId={id} />}
            {activeTab === 'prescriptions' && <PrescriptionsPanel patientId={id} />}
            {activeTab === 'immunisations' && <ImmunisationsPanel patientId={id} />}
            {activeTab === 'acc'           && <AccPanel patientId={id} />}
            {activeTab === 'nes'           && <NesPanel patientId={id} />}
          </div>
        </>
      )}
    </AppShell>
  );
}
