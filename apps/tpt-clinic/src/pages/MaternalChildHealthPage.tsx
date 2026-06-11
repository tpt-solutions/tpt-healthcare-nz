import { useState, useEffect } from 'react';
import AppShell from '@/components/AppShell';

type Tab = 'overview' | 'maternity' | 'intrapartum' | 'neonatal' | 'paediatrics' | 'wellchild' | 'mmpo' | 'consent';

interface MaternityEpisode {
  id: string; patientNhi: string; lmcHpi: string; status: string;
  edd: string | null; gestationAtBookingWeeks: number | null; riskLevel: string;
}
interface NICUAdmission {
  id: string; patientNhi: string; status: string; gestation_at_birth_weeks: number | null;
  birthWeightGrams: number | null; respiratorySupport: string;
}
interface SCBUAdmission {
  id: string; patientNhi: string; status: string; feedingMethod: string;
  gestationWeeks: number | null; birthWeightGrams: number | null;
}
interface PaediatricAdmission {
  id: string; patientNhi: string; status: string; admissionType: string;
  admissionReason: string; ward: string; ageYears: number | null; ageMonths: number | null;
}
interface PICUAdmission {
  id: string; patientNhi: string; status: string; respiratorySupport: string;
  tpnActive: boolean; inotropesActive: boolean;
}
interface WellChildCheck {
  id: string; patientNhi: string; checkType: string; status: string;
  ageAtCheckWeeks: number | null; immunisationsUpToDate: boolean; developmentalConcerns: boolean;
}
interface MMPOClaim {
  id: string; lmcHpi: string; claimType: string; serviceDate: string;
  serviceCode: string; amountNzd: number | null; status: string;
}
interface ConsentForm {
  id: string; resourceType: string; resourceId: string; patientNhi: string;
  consentType: string; consentGiven: boolean; givenByName: string;
  givenByRelationship: string; clinicianHpi: string; description: string;
  signedAt: string | null; withdrawnAt: string | null; status: string;
}

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: [Tab, string][] = [
    ['overview', 'Overview'],
    ['maternity', 'Maternity'],
    ['intrapartum', 'Intrapartum'],
    ['neonatal', 'Neonatal'],
    ['paediatrics', 'Paediatrics'],
    ['wellchild', 'Well Child'],
    ['mmpo', 'MMPO Claiming'],
    ['consent', 'Consent & Assent'],
  ];
  return (
    <div className="mb-6 flex flex-wrap gap-1 rounded-lg bg-secondary-100 p-1">
      {tabs.map(([id, label]) => (
        <button key={id} onClick={() => onSelect(id)}
          className={`rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === id ? 'bg-white text-primary-700 shadow-sm' : 'text-secondary-600 hover:text-secondary-900'
          }`}>
          {label}
        </button>
      ))}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    booking: 'bg-blue-100 text-blue-800',
    antenatal: 'bg-sky-100 text-sky-800',
    intrapartum: 'bg-amber-100 text-amber-800',
    postnatal: 'bg-green-100 text-green-800',
    completed: 'bg-secondary-100 text-secondary-700',
    admitted: 'bg-amber-100 text-amber-800',
    stable: 'bg-green-100 text-green-800',
    critical: 'bg-red-100 text-red-800',
    discharged: 'bg-secondary-100 text-secondary-700',
    draft: 'bg-secondary-100 text-secondary-700',
    submitted: 'bg-blue-100 text-blue-800',
    accepted: 'bg-green-100 text-green-800',
    paid: 'bg-emerald-100 text-emerald-800',
    rejected: 'bg-red-100 text-red-800',
    withdrawn: 'bg-secondary-100 text-secondary-700',
    scheduled: 'bg-blue-100 text-blue-800',
    completed_check: 'bg-green-100 text-green-800',
    missed: 'bg-red-100 text-red-800',
    declined: 'bg-secondary-100 text-secondary-700',
  };
  const key = status === 'completed' && status === 'completed' ? status : status;
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${map[key] ?? 'bg-secondary-100 text-secondary-700'}`}>
      {status.replace(/-/g, ' ')}
    </span>
  );
}

export default function MaternalChildHealthPage() {
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [episodes, setEpisodes] = useState<MaternityEpisode[]>([]);
  const [nicuAdmissions, setNicuAdmissions] = useState<NICUAdmission[]>([]);
  const [scbuAdmissions, setScbuAdmissions] = useState<SCBUAdmission[]>([]);
  const [paedAdmissions, setPaedAdmissions] = useState<PaediatricAdmission[]>([]);
  const [picuAdmissions, setPicuAdmissions] = useState<PICUAdmission[]>([]);
  const [wellChildChecks, setWellChildChecks] = useState<WellChildCheck[]>([]);
  const [mmpoClaims, setMmpoClaims] = useState<MMPOClaim[]>([]);
  const [consentForms, setConsentForms] = useState<ConsentForm[]>([]);

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/maternity/episodes').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/maternity/nicu').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/maternity/scbu').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/paediatrics/admissions').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/paediatrics/picu').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/well-child').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/maternity/mmpo/claims').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/consent').then(r => r.ok ? r.json() : []),
    ])
      .then(([ep, nicu, scbu, paed, picu, wc, mmpo, consent]) => {
        setEpisodes(ep ?? []);
        setNicuAdmissions(nicu ?? []);
        setScbuAdmissions(scbu ?? []);
        setPaedAdmissions(paed ?? []);
        setPicuAdmissions(picu ?? []);
        setWellChildChecks(wc ?? []);
        setMmpoClaims(mmpo ?? []);
        setConsentForms(consent ?? []);
      })
      .catch(() => {});
  }, []);

  const activeEpisodes = episodes.filter(e => e.status !== 'completed');
  const activeNicu = nicuAdmissions.filter(n => n.status !== 'discharged');
  const activeScbu = scbuAdmissions.filter(s => s.status !== 'discharged' && s.status !== 'transferred-nicu');
  const activePaed = paedAdmissions.filter(p => p.status !== 'discharged');
  const activePicu = picuAdmissions.filter(p => p.status !== 'discharged');
  const pendingClaims = mmpoClaims.filter(c => c.status === 'draft' || c.status === 'submitted');

  return (
    <AppShell title="Maternal & Child Health">
      <TabBar active={activeTab} onSelect={setActiveTab} />

      {activeTab === 'overview' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {[
              { label: 'Active Maternity Episodes', value: activeEpisodes.length, border: 'border-pink-200' },
              { label: 'NICU / SCBU', value: activeNicu.length + activeScbu.length, border: 'border-amber-200' },
              { label: 'Paediatric Inpatients', value: activePaed.length + activePicu.length, border: 'border-blue-200' },
              { label: 'Pending MMPO Claims', value: pendingClaims.length, border: 'border-green-200' },
            ].map(s => (
              <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-5 shadow-sm`}>
                <p className="text-sm font-medium text-secondary-500">{s.label}</p>
                <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              </div>
            ))}
          </div>
          <div className="mt-6 grid gap-4 lg:grid-cols-2">
            <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
              <h2 className="text-base font-semibold text-secondary-900">Maternity Services</h2>
              <p className="mt-2 text-sm text-secondary-500">
                LMC registration, antenatal care, intrapartum, postnatal, and NBRS birth notifications.
                MMPO claiming integrated for LMC funding.
              </p>
              <button onClick={() => setActiveTab('maternity')}
                className="mt-4 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                View Maternity Episodes
              </button>
            </div>
            <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
              <h2 className="text-base font-semibold text-secondary-900">Paediatric Services</h2>
              <p className="mt-2 text-sm text-secondary-500">
                NICU (≤28 days / &lt;44 weeks corrected), SCBU (32–36 weeks), paediatric inpatient,
                PICU, growth tracking, developmental milestones, and Well Child Tamariki Ora.
              </p>
              <button onClick={() => setActiveTab('paediatrics')}
                className="mt-4 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                View Paediatric Admissions
              </button>
            </div>
          </div>
          <div className="mt-4 rounded-md bg-blue-50 border border-blue-200 px-4 py-3">
            <p className="text-xs font-medium text-blue-800">
              Child protection flags (Children's Act 2014) are recorded on paediatric admissions.
              Well Child Tamariki Ora B4 School Checks include the SDQ (Strengths and Difficulties Questionnaire).
              Consent and assent documentation for parent/guardian proxy applies to all records under this module.
            </p>
          </div>
        </>
      )}

      {activeTab === 'maternity' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Maternity Episodes</h2>
              <p className="text-xs text-secondary-500 mt-0.5">Booking → Antenatal → Intrapartum → Postnatal</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Episode
            </button>
          </div>
          {episodes.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No maternity episodes.</p>
              <p className="text-xs">Create a new episode at booking to begin LMC registration.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">LMC HPI</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">EDD</th>
                  <th className="px-6 py-3 text-left">Gestation</th>
                  <th className="px-6 py-3 text-left">Risk</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {episodes.map(e => (
                  <tr key={e.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{e.patientNhi}</td>
                    <td className="px-6 py-3 font-mono text-xs">{e.lmcHpi}</td>
                    <td className="px-6 py-3"><StatusBadge status={e.status} /></td>
                    <td className="px-6 py-3 text-secondary-500">
                      {e.edd ? new Date(e.edd).toLocaleDateString('en-NZ') : '—'}
                    </td>
                    <td className="px-6 py-3">
                      {e.gestationAtBookingWeeks != null ? `${e.gestationAtBookingWeeks}w` : '—'}
                    </td>
                    <td className="px-6 py-3 capitalize">{e.riskLevel}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'intrapartum' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Intrapartum Care</h2>
              <p className="text-xs text-secondary-500 mt-0.5">Birthing suite, partogram, CTG recording</p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Start Labour Episode
            </button>
          </div>
          <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
            <p className="text-sm">Select a maternity episode to view intrapartum records.</p>
            <p className="text-xs">Partogram entries and CTG recordings are accessible per episode.</p>
          </div>
        </div>
      )}

      {activeTab === 'neonatal' && (
        <div className="space-y-4">
          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div>
                <h2 className="text-base font-semibold text-secondary-900">NICU Admissions</h2>
                <p className="text-xs text-secondary-500 mt-0.5">≤28 days / &lt;44 weeks corrected gestation</p>
              </div>
              <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                + Admit to NICU
              </button>
            </div>
            {nicuAdmissions.length === 0 ? (
              <div className="flex h-28 flex-col items-center justify-center gap-1 text-secondary-400">
                <p className="text-sm">No NICU admissions.</p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                  <tr>
                    <th className="px-6 py-3 text-left">Patient NHI</th>
                    <th className="px-6 py-3 text-left">Status</th>
                    <th className="px-6 py-3 text-left">Gestation</th>
                    <th className="px-6 py-3 text-left">Birth Weight</th>
                    <th className="px-6 py-3 text-left">Resp Support</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {nicuAdmissions.map(n => (
                    <tr key={n.id} className="hover:bg-secondary-50">
                      <td className="px-6 py-3 font-mono text-xs">{n.patientNhi}</td>
                      <td className="px-6 py-3"><StatusBadge status={n.status} /></td>
                      <td className="px-6 py-3">{n.gestation_at_birth_weeks != null ? `${n.gestation_at_birth_weeks}w` : '—'}</td>
                      <td className="px-6 py-3">{n.birthWeightGrams != null ? `${n.birthWeightGrams} g` : '—'}</td>
                      <td className="px-6 py-3 capitalize">{n.respiratorySupport.replace(/-/g, ' ')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div>
                <h2 className="text-base font-semibold text-secondary-900">SCBU Admissions</h2>
                <p className="text-xs text-secondary-500 mt-0.5">Special Care Baby Unit — step-down from NICU (~32–36 weeks)</p>
              </div>
              <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                + Admit to SCBU
              </button>
            </div>
            {scbuAdmissions.length === 0 ? (
              <div className="flex h-28 flex-col items-center justify-center gap-1 text-secondary-400">
                <p className="text-sm">No SCBU admissions.</p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                  <tr>
                    <th className="px-6 py-3 text-left">Patient NHI</th>
                    <th className="px-6 py-3 text-left">Status</th>
                    <th className="px-6 py-3 text-left">Gestation</th>
                    <th className="px-6 py-3 text-left">Birth Weight</th>
                    <th className="px-6 py-3 text-left">Feeding</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {scbuAdmissions.map(s => (
                    <tr key={s.id} className="hover:bg-secondary-50">
                      <td className="px-6 py-3 font-mono text-xs">{s.patientNhi}</td>
                      <td className="px-6 py-3"><StatusBadge status={s.status} /></td>
                      <td className="px-6 py-3">{s.gestationWeeks != null ? `${s.gestationWeeks}w` : '—'}</td>
                      <td className="px-6 py-3">{s.birthWeightGrams != null ? `${s.birthWeightGrams} g` : '—'}</td>
                      <td className="px-6 py-3 capitalize">{s.feedingMethod}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {activeTab === 'paediatrics' && (
        <div className="space-y-4">
          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div>
                <h2 className="text-base font-semibold text-secondary-900">Paediatric Inpatient Admissions</h2>
                <p className="text-xs text-secondary-500 mt-0.5">Age/weight-adjusted clinical ranges applied</p>
              </div>
              <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                + Admit Patient
              </button>
            </div>
            {paedAdmissions.length === 0 ? (
              <div className="flex h-28 flex-col items-center justify-center gap-1 text-secondary-400">
                <p className="text-sm">No paediatric admissions.</p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                  <tr>
                    <th className="px-6 py-3 text-left">Patient NHI</th>
                    <th className="px-6 py-3 text-left">Status</th>
                    <th className="px-6 py-3 text-left">Type</th>
                    <th className="px-6 py-3 text-left">Reason</th>
                    <th className="px-6 py-3 text-left">Age</th>
                    <th className="px-6 py-3 text-left">Ward</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {paedAdmissions.map(p => (
                    <tr key={p.id} className="hover:bg-secondary-50">
                      <td className="px-6 py-3 font-mono text-xs">{p.patientNhi}</td>
                      <td className="px-6 py-3"><StatusBadge status={p.status} /></td>
                      <td className="px-6 py-3 capitalize">{p.admissionType}</td>
                      <td className="px-6 py-3 max-w-[200px] truncate">{p.admissionReason}</td>
                      <td className="px-6 py-3">
                        {p.ageYears != null ? `${p.ageYears}y` : ''}
                        {p.ageMonths != null ? ` ${p.ageMonths}m` : ''}
                        {p.ageYears == null && p.ageMonths == null ? '—' : ''}
                      </td>
                      <td className="px-6 py-3">{p.ward || '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div>
                <h2 className="text-base font-semibold text-secondary-900">PICU Admissions</h2>
                <p className="text-xs text-secondary-500 mt-0.5">Paediatric ICU — children &gt;28 days requiring intensive care</p>
              </div>
              <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                + Admit to PICU
              </button>
            </div>
            {picuAdmissions.length === 0 ? (
              <div className="flex h-28 flex-col items-center justify-center gap-1 text-secondary-400">
                <p className="text-sm">No PICU admissions.</p>
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                  <tr>
                    <th className="px-6 py-3 text-left">Patient NHI</th>
                    <th className="px-6 py-3 text-left">Status</th>
                    <th className="px-6 py-3 text-left">Resp Support</th>
                    <th className="px-6 py-3 text-left">TPN</th>
                    <th className="px-6 py-3 text-left">Inotropes</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {picuAdmissions.map(p => (
                    <tr key={p.id} className="hover:bg-secondary-50">
                      <td className="px-6 py-3 font-mono text-xs">{p.patientNhi}</td>
                      <td className="px-6 py-3"><StatusBadge status={p.status} /></td>
                      <td className="px-6 py-3 capitalize">{p.respiratorySupport.replace(/-/g, ' ')}</td>
                      <td className="px-6 py-3">{p.tpnActive ? 'Yes' : 'No'}</td>
                      <td className="px-6 py-3">{p.inotropesActive ? 'Yes' : 'No'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {activeTab === 'wellchild' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Well Child Tamariki Ora</h2>
              <p className="text-xs text-secondary-500 mt-0.5">
                Neonatal → 6wk → 3mo → 5mo → 9mo → 12mo → 15mo → 2yr → B4 School Check
              </p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Record Check
            </button>
          </div>
          {wellChildChecks.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No Well Child checks recorded.</p>
              <p className="text-xs">Record checks from birth through to the B4 School Check at age ~4.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Check Type</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">Age (weeks)</th>
                  <th className="px-6 py-3 text-left">Immunisations</th>
                  <th className="px-6 py-3 text-left">Dev Concerns</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {wellChildChecks.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.patientNhi}</td>
                    <td className="px-6 py-3 font-medium">{c.checkType}</td>
                    <td className="px-6 py-3"><StatusBadge status={c.status} /></td>
                    <td className="px-6 py-3">{c.ageAtCheckWeeks != null ? `${c.ageAtCheckWeeks}w` : '—'}</td>
                    <td className="px-6 py-3">
                      <span className={`text-xs font-medium ${c.immunisationsUpToDate ? 'text-green-700' : 'text-red-700'}`}>
                        {c.immunisationsUpToDate ? 'Up to date' : 'Overdue'}
                      </span>
                    </td>
                    <td className="px-6 py-3">
                      {c.developmentalConcerns ? (
                        <span className="rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-800">Yes</span>
                      ) : (
                        <span className="text-xs text-secondary-400">None</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {activeTab === 'mmpo' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">MMPO LMC Funding Claims</h2>
              <p className="text-xs text-secondary-500 mt-0.5">
                Midwifery and Maternity Providers Organisation — LMC Schedule of Payments
              </p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + New Claim
            </button>
          </div>
          {mmpoClaims.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No MMPO claims.</p>
              <p className="text-xs">Create a claim for booking, antenatal, intrapartum, or postnatal services.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">LMC HPI</th>
                  <th className="px-6 py-3 text-left">Type</th>
                  <th className="px-6 py-3 text-left">Service Date</th>
                  <th className="px-6 py-3 text-left">Code</th>
                  <th className="px-6 py-3 text-left">Amount</th>
                  <th className="px-6 py-3 text-left">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {mmpoClaims.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.lmcHpi}</td>
                    <td className="px-6 py-3 capitalize">{c.claimType.replace(/-/g, ' ')}</td>
                    <td className="px-6 py-3 text-secondary-500">
                      {new Date(c.serviceDate).toLocaleDateString('en-NZ')}
                    </td>
                    <td className="px-6 py-3 font-mono text-xs">{c.serviceCode}</td>
                    <td className="px-6 py-3">
                      {c.amountNzd != null ? `$${c.amountNzd.toFixed(2)}` : '—'}
                    </td>
                    <td className="px-6 py-3"><StatusBadge status={c.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
      {activeTab === 'consent' && (
        <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
            <div>
              <h2 className="text-base font-semibold text-secondary-900">Consent &amp; Assent Documentation</h2>
              <p className="text-xs text-secondary-500 mt-0.5">
                Parent/guardian proxy consent and child assent across all MCH records
              </p>
            </div>
            <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
              + Record Consent
            </button>
          </div>
          <div className="border-b border-secondary-100 bg-amber-50 px-6 py-2.5">
            <p className="text-xs text-amber-800">
              Consent withdrawal is permanent for a given form. To re-obtain consent, create a new form.
              Child assent should be recorded separately from parent/guardian consent for children aged ~7 and over.
            </p>
          </div>
          {consentForms.length === 0 ? (
            <div className="flex h-32 flex-col items-center justify-center gap-1 text-secondary-400">
              <p className="text-sm">No consent forms recorded.</p>
              <p className="text-xs">Record consent forms for procedures, treatment, information sharing, or research.</p>
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
                <tr>
                  <th className="px-6 py-3 text-left">Patient NHI</th>
                  <th className="px-6 py-3 text-left">Type</th>
                  <th className="px-6 py-3 text-left">Resource</th>
                  <th className="px-6 py-3 text-left">Given By</th>
                  <th className="px-6 py-3 text-left">Relationship</th>
                  <th className="px-6 py-3 text-left">Given</th>
                  <th className="px-6 py-3 text-left">Status</th>
                  <th className="px-6 py-3 text-left">Signed</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {consentForms.map(c => (
                  <tr key={c.id} className="hover:bg-secondary-50">
                    <td className="px-6 py-3 font-mono text-xs">{c.patientNhi}</td>
                    <td className="px-6 py-3 capitalize">{c.consentType.replace(/-/g, ' ')}</td>
                    <td className="px-6 py-3 text-secondary-500 text-xs capitalize">
                      {c.resourceType.replace(/_/g, ' ')}
                    </td>
                    <td className="px-6 py-3">{c.givenByName}</td>
                    <td className="px-6 py-3 capitalize">{c.givenByRelationship}</td>
                    <td className="px-6 py-3">
                      <span className={`text-xs font-medium ${c.consentGiven ? 'text-green-700' : 'text-red-700'}`}>
                        {c.consentGiven ? 'Yes' : 'No'}
                      </span>
                    </td>
                    <td className="px-6 py-3"><StatusBadge status={c.status} /></td>
                    <td className="px-6 py-3 text-secondary-500 text-xs">
                      {c.signedAt ? new Date(c.signedAt).toLocaleDateString('en-NZ') : '—'}
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
