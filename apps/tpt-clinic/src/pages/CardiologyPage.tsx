import React, { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface CardiologyAppointment {
  id: string;
  patientNhi: string;
  clinicianHpi: string;
  appointmentType: string;
  status: string;
  indication: string;
  primaryDiagnosis: string;
  scheduledAt: string;
  completedAt?: string;
}

interface ECGStudy {
  id: string;
  patientNhi: string;
  studyType: string;
  status: string;
  indication: string;
  rhythm: string;
  interpretation: string;
  orderedAt: string;
  reportedAt?: string;
}

interface EchoStudy {
  id: string;
  patientNhi: string;
  studyType: string;
  status: string;
  indication: string;
  lvefPercent?: number;
  interpretation: string;
  orderedAt: string;
  reportedAt?: string;
}

interface HolterMonitor {
  id: string;
  patientNhi: string;
  monitorType: string;
  status: string;
  indication: string;
  afBurdenPercent?: number;
  interpretation: string;
  orderedAt: string;
}

interface CathProcedure {
  id: string;
  patientNhi: string;
  procedureType: string;
  status: string;
  indication: string;
  operatorClinicianHpi: string;
  scheduledAt?: string;
  completedAt?: string;
}

interface CardiacRehabProgramme {
  id: string;
  patientNhi: string;
  phase: string;
  status: string;
  indication: string;
  sessionsCompleted?: number;
  sessionsPlanned?: number;
}

interface ImplantableDevice {
  id: string;
  patientNhi: string;
  deviceType: string;
  deviceBrand: string;
  modelName: string;
  status: string;
  estimatedLongevityMonths?: number;
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------
type TabId = 'appointments' | 'ecg' | 'echo' | 'holter' | 'cath' | 'rehab' | 'devices';

const TABS: { id: TabId; label: string }[] = [
  { id: 'appointments', label: 'Clinics' },
  { id: 'ecg',          label: 'ECG' },
  { id: 'echo',         label: 'Echo' },
  { id: 'holter',       label: 'Holter / ABPM' },
  { id: 'cath',         label: 'Cath Lab' },
  { id: 'rehab',        label: 'Rehab' },
  { id: 'devices',      label: 'Devices' },
];

function TabBar({ active, onChange }: { active: TabId; onChange: (t: TabId) => void }) {
  return (
    <div className="flex gap-1 border-b border-secondary-200 pb-0">
      {TABS.map((t) => (
        <button
          key={t.id}
          onClick={() => onChange(t.id)}
          className={[
            'px-4 py-2 text-sm font-medium rounded-t-md transition-colors',
            active === t.id
              ? 'bg-white border border-b-white border-secondary-200 text-primary-600 -mb-px'
              : 'text-secondary-500 hover:text-secondary-800',
          ].join(' ')}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const colours: Record<string, string> = {
    scheduled:    'bg-blue-100 text-blue-700',
    'in-progress': 'bg-yellow-100 text-yellow-700',
    completed:    'bg-green-100 text-green-700',
    cancelled:    'bg-secondary-100 text-secondary-500',
    'did-not-attend': 'bg-red-100 text-red-700',
    ordered:      'bg-blue-100 text-blue-700',
    performed:    'bg-yellow-100 text-yellow-700',
    reported:     'bg-green-100 text-green-700',
    booked:       'bg-blue-100 text-blue-700',
    fitted:       'bg-yellow-100 text-yellow-700',
    referred:     'bg-purple-100 text-purple-700',
    enrolled:     'bg-blue-100 text-blue-700',
    active:       'bg-green-100 text-green-700',
    withdrawn:    'bg-secondary-100 text-secondary-500',
    'battery-replacement-due': 'bg-orange-100 text-orange-700',
    explanted:    'bg-secondary-100 text-secondary-500',
  };
  const cls = colours[status] ?? 'bg-secondary-100 text-secondary-600';
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${cls}`}>
      {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Tab panels
// ---------------------------------------------------------------------------
function AppointmentsTab({ items }: { items: CardiologyAppointment[] }) {
  if (items.length === 0) return <EmptyState message="No cardiology appointments found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Type', 'Status', 'Indication', 'Diagnosis', 'Scheduled'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((a) => (
            <tr key={a.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{a.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-700 capitalize">{a.appointmentType}</td>
              <td className="px-4 py-3"><StatusBadge status={a.status} /></td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{a.indication || '—'}</td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{a.primaryDiagnosis || '—'}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(a.scheduledAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ECGTab({ items }: { items: ECGStudy[] }) {
  if (items.length === 0) return <EmptyState message="No ECG studies found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Type', 'Status', 'Rhythm', 'Indication', 'Ordered'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((e) => (
            <tr key={e.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{e.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-700 capitalize">{e.studyType}</td>
              <td className="px-4 py-3"><StatusBadge status={e.status} /></td>
              <td className="px-4 py-3 text-secondary-600">{e.rhythm || '—'}</td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{e.indication || '—'}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(e.orderedAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EchoTab({ items }: { items: EchoStudy[] }) {
  if (items.length === 0) return <EmptyState message="No echocardiography studies found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Type', 'Status', 'LVEF %', 'Indication', 'Ordered'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((e) => (
            <tr key={e.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{e.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-700">{e.studyType}</td>
              <td className="px-4 py-3"><StatusBadge status={e.status} /></td>
              <td className="px-4 py-3 text-secondary-700">
                {e.lvefPercent != null ? (
                  <span className={e.lvefPercent < 40 ? 'text-red-600 font-semibold' : e.lvefPercent < 55 ? 'text-orange-600' : 'text-green-600'}>
                    {e.lvefPercent}%
                  </span>
                ) : '—'}
              </td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{e.indication || '—'}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(e.orderedAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function HolterTab({ items }: { items: HolterMonitor[] }) {
  if (items.length === 0) return <EmptyState message="No Holter / ABPM studies found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Type', 'Status', 'AF Burden', 'Indication', 'Ordered'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((m) => (
            <tr key={m.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{m.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-700 capitalize">{m.monitorType}</td>
              <td className="px-4 py-3"><StatusBadge status={m.status} /></td>
              <td className="px-4 py-3 text-secondary-700">
                {m.afBurdenPercent != null ? (
                  <span className={m.afBurdenPercent > 5 ? 'text-red-600 font-semibold' : 'text-secondary-700'}>
                    {m.afBurdenPercent.toFixed(1)}%
                  </span>
                ) : '—'}
              </td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{m.indication || '—'}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(m.orderedAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CathTab({ items }: { items: CathProcedure[] }) {
  if (items.length === 0) return <EmptyState message="No cath lab procedures found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Procedure', 'Status', 'Indication', 'Operator', 'Scheduled'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((c) => (
            <tr key={c.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{c.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-700 capitalize">{c.procedureType}</td>
              <td className="px-4 py-3"><StatusBadge status={c.status} /></td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{c.indication || '—'}</td>
              <td className="px-4 py-3 font-mono text-secondary-500 text-xs">{c.operatorClinicianHpi || '—'}</td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">
                {c.scheduledAt ? new Date(c.scheduledAt).toLocaleDateString('en-NZ') : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RehabTab({ items }: { items: CardiacRehabProgramme[] }) {
  if (items.length === 0) return <EmptyState message="No cardiac rehabilitation programmes found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Phase', 'Status', 'Indication', 'Progress'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((p) => (
            <tr key={p.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{p.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-700">Phase {p.phase}</td>
              <td className="px-4 py-3"><StatusBadge status={p.status} /></td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{p.indication || '—'}</td>
              <td className="px-4 py-3 text-secondary-600">
                {p.sessionsPlanned != null
                  ? `${p.sessionsCompleted ?? 0} / ${p.sessionsPlanned} sessions`
                  : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DevicesTab({ items }: { items: ImplantableDevice[] }) {
  if (items.length === 0) return <EmptyState message="No implantable devices found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Type', 'Brand / Model', 'Status', 'Est. Longevity'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((d) => (
            <tr key={d.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{d.patientNhi}</td>
              <td className="px-4 py-3 text-secondary-700 capitalize">{d.deviceType}</td>
              <td className="px-4 py-3 text-secondary-600">{d.deviceBrand} {d.modelName}</td>
              <td className="px-4 py-3"><StatusBadge status={d.status} /></td>
              <td className="px-4 py-3 text-secondary-600">
                {d.estimatedLongevityMonths != null ? (
                  <span className={d.estimatedLongevityMonths < 6 ? 'text-red-600 font-semibold' : d.estimatedLongevityMonths < 12 ? 'text-orange-600' : 'text-secondary-700'}>
                    {d.estimatedLongevityMonths} mo
                  </span>
                ) : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-secondary-400">
      <svg className="h-12 w-12 mb-3" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
      <p className="text-sm">{message}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------
export default function CardiologyPage() {
  const [activeTab, setActiveTab] = useState<TabId>('appointments');
  const [appointments, setAppointments] = useState<CardiologyAppointment[]>([]);
  const [ecgStudies, setEcgStudies]     = useState<ECGStudy[]>([]);
  const [echoStudies, setEchoStudies]   = useState<EchoStudy[]>([]);
  const [holterItems, setHolterItems]   = useState<HolterMonitor[]>([]);
  const [cathProcs, setCathProcs]       = useState<CathProcedure[]>([]);
  const [rehabProgs, setRehabProgs]     = useState<CardiacRehabProgramme[]>([]);
  const [devices, setDevices]           = useState<ImplantableDevice[]>([]);
  const [loading, setLoading]           = useState(true);
  const [error, setError]               = useState<string | null>(null);

  useEffect(() => {
    const base = '/api/v1/cardiology';
    setLoading(true);
    setError(null);
    Promise.all([
      fetch(`${base}/appointments`).then((r) => r.json()),
      fetch(`${base}/ecg`).then((r) => r.json()),
      fetch(`${base}/echo`).then((r) => r.json()),
      fetch(`${base}/holter`).then((r) => r.json()),
      fetch(`${base}/cath-lab`).then((r) => r.json()),
      fetch(`${base}/rehab`).then((r) => r.json()),
      fetch(`${base}/devices`).then((r) => r.json()),
    ])
      .then(([appts, ecg, echo, holter, cath, rehab, devs]) => {
        setAppointments(Array.isArray(appts) ? appts : []);
        setEcgStudies(Array.isArray(ecg) ? ecg : []);
        setEchoStudies(Array.isArray(echo) ? echo : []);
        setHolterItems(Array.isArray(holter) ? holter : []);
        setCathProcs(Array.isArray(cath) ? cath : []);
        setRehabProgs(Array.isArray(rehab) ? rehab : []);
        setDevices(Array.isArray(devs) ? devs : []);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Failed to load cardiology data'))
      .finally(() => setLoading(false));
  }, []);

  return (
    <AppShell title="Cardiology">
      <div className="p-6 space-y-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-secondary-900">Cardiology</h1>
            <p className="mt-1 text-sm text-secondary-500">
              Outpatient clinics, investigations, cath lab, rehabilitation, and device management
            </p>
          </div>
        </div>

        {/* Summary cards */}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
          {[
            { label: 'Appointments', count: appointments.length, colour: 'blue' },
            { label: 'ECG',          count: ecgStudies.length,   colour: 'indigo' },
            { label: 'Echo',         count: echoStudies.length,  colour: 'violet' },
            { label: 'Holter/ABPM',  count: holterItems.length,  colour: 'purple' },
            { label: 'Cath Lab',     count: cathProcs.length,    colour: 'rose' },
            { label: 'Rehab',        count: rehabProgs.length,   colour: 'teal' },
            { label: 'Devices',      count: devices.length,      colour: 'orange' },
          ].map((c) => (
            <div key={c.label} className="rounded-xl border border-secondary-200 bg-white px-4 py-3 shadow-sm">
              <p className="text-xs text-secondary-500">{c.label}</p>
              <p className="text-2xl font-semibold text-secondary-900">{loading ? '—' : c.count}</p>
            </div>
          ))}
        </div>

        {/* Error */}
        {error && (
          <div className="rounded-lg bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        )}

        {/* Tab content */}
        <div className="rounded-xl border border-secondary-200 bg-white shadow-sm overflow-hidden">
          <div className="px-4 pt-4">
            <TabBar active={activeTab} onChange={setActiveTab} />
          </div>
          <div className="p-4">
            {loading ? (
              <div className="flex items-center justify-center py-16">
                <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
              </div>
            ) : (
              <>
                {activeTab === 'appointments' && <AppointmentsTab items={appointments} />}
                {activeTab === 'ecg'          && <ECGTab           items={ecgStudies} />}
                {activeTab === 'echo'         && <EchoTab          items={echoStudies} />}
                {activeTab === 'holter'       && <HolterTab        items={holterItems} />}
                {activeTab === 'cath'         && <CathTab          items={cathProcs} />}
                {activeTab === 'rehab'        && <RehabTab         items={rehabProgs} />}
                {activeTab === 'devices'      && <DevicesTab       items={devices} />}
              </>
            )}
          </div>
        </div>
      </div>
    </AppShell>
  );
}
