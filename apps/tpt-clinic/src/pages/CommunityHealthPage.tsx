import { useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
interface HomeVisit {
  id: string;
  patientNhi: string;
  clinicianHpi: string;
  visitType: string;
  priority: string;
  status: string;
  address: string;
  followUpRequired: boolean;
  scheduledAt: string;
  actualStartAt?: string;
  actualEndAt?: string;
}

interface CarePlan {
  id: string;
  patientNhi: string;
  clinicianHpi: string;
  planName: string;
  planType: string;
  status: string;
  riskLevel: string;
  primaryNeed: string;
  dhbFunded: boolean;
  consentGiven: boolean;
  startedAt: string;
  reviewAt?: string;
}

interface NursingVisit {
  id: string;
  carePlanId: string;
  patientNhi: string;
  clinicianHpi: string;
  visitType: string;
  status: string;
  followUpRequired: boolean;
  scheduledAt: string;
  completedAt?: string;
}

interface OutreachProgramme {
  id: string;
  programmeName: string;
  programmeType: string;
  targetPopulation: string;
  status: string;
  coordinatorHpi: string;
  startDate: string;
  endDate?: string;
}

interface OutreachEvent {
  id: string;
  programmeId: string;
  eventName: string;
  eventType: string;
  location: string;
  targetAttendees?: number;
  actualAttendees: number;
  status: string;
  scheduledAt: string;
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------
type TabId = 'home-visits' | 'district-nursing' | 'outreach';

const TABS: { id: TabId; label: string }[] = [
  { id: 'home-visits',       label: 'Home Visits' },
  { id: 'district-nursing',  label: 'District Nursing' },
  { id: 'outreach',          label: 'Outreach' },
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
    scheduled:        'bg-blue-100 text-blue-700',
    'in-transit':     'bg-indigo-100 text-indigo-700',
    arrived:          'bg-teal-100 text-teal-700',
    'in-progress':    'bg-yellow-100 text-yellow-700',
    completed:        'bg-green-100 text-green-700',
    cancelled:        'bg-secondary-100 text-secondary-500',
    rescheduled:      'bg-orange-100 text-orange-700',
    dna:              'bg-red-100 text-red-700',
    draft:            'bg-secondary-100 text-secondary-500',
    active:           'bg-green-100 text-green-700',
    'under-review':   'bg-yellow-100 text-yellow-700',
    suspended:        'bg-orange-100 text-orange-700',
    planned:          'bg-blue-100 text-blue-700',
    confirmed:        'bg-teal-100 text-teal-700',
    paused:           'bg-orange-100 text-orange-700',
    discontinued:     'bg-secondary-100 text-secondary-500',
    urgent:           'bg-red-100 text-red-700',
    high:             'bg-orange-100 text-orange-700',
    routine:          'bg-secondary-100 text-secondary-600',
    low:              'bg-secondary-50 text-secondary-400',
  };
  const cls = colours[status] ?? 'bg-secondary-100 text-secondary-600';
  return (
    <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${cls}`}>
      {status}
    </span>
  );
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-secondary-400">
      <svg className="h-12 w-12 mb-3" fill="none" stroke="currentColor" strokeWidth={1} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 21v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21m0 0h4.5V3.545M12.75 21h7.5V10.75M2.25 21h1.5m18 0h-18M2.25 9l4.5-1.636M18.75 3l-1.5.545m0 6.205l3 1m-9 .545l-3-1" />
      </svg>
      <p className="text-sm">{message}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab panels
// ---------------------------------------------------------------------------
function HomeVisitsTab({ items }: { items: HomeVisit[] }) {
  if (items.length === 0) return <EmptyState message="No home visits found." />;
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-secondary-200 text-sm">
        <thead className="bg-secondary-50">
          <tr>
            {['NHI', 'Type', 'Priority', 'Status', 'Address', 'Follow-up', 'Scheduled'].map((h) => (
              <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-secondary-100">
          {items.map((v) => (
            <tr key={v.id} className="hover:bg-secondary-50">
              <td className="px-4 py-3 font-mono text-secondary-700">{v.patientNhi || '—'}</td>
              <td className="px-4 py-3 text-secondary-600 capitalize">{v.visitType.replace(/-/g, ' ')}</td>
              <td className="px-4 py-3"><StatusBadge status={v.priority} /></td>
              <td className="px-4 py-3"><StatusBadge status={v.status} /></td>
              <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{v.address || '—'}</td>
              <td className="px-4 py-3 text-center">
                {v.followUpRequired
                  ? <span className="inline-block h-2 w-2 rounded-full bg-orange-400" title="Follow-up required" />
                  : <span className="inline-block h-2 w-2 rounded-full bg-secondary-200" />}
              </td>
              <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(v.scheduledAt).toLocaleDateString('en-NZ')}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function DistrictNursingTab({ carePlans, nursingVisits }: { carePlans: CarePlan[]; nursingVisits: NursingVisit[] }) {
  const [subTab, setSubTab] = useState<'plans' | 'visits'>('plans');
  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {(['plans', 'visits'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setSubTab(t)}
            className={`px-3 py-1.5 rounded text-xs font-medium transition-colors ${
              subTab === t ? 'bg-primary-100 text-primary-700' : 'text-secondary-500 hover:text-secondary-800 hover:bg-secondary-100'
            }`}
          >
            {t === 'plans' ? `Care Plans (${carePlans.length})` : `Visits (${nursingVisits.length})`}
          </button>
        ))}
      </div>

      {subTab === 'plans' && (
        carePlans.length === 0 ? <EmptyState message="No district nursing care plans found." /> : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-secondary-200 text-sm">
              <thead className="bg-secondary-50">
                <tr>
                  {['NHI', 'Plan', 'Type', 'Status', 'Risk', 'Primary Need', 'DHB', 'Consent', 'Started'].map((h) => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-secondary-100">
                {carePlans.map((p) => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-4 py-3 font-mono text-secondary-700">{p.patientNhi || '—'}</td>
                    <td className="px-4 py-3 text-secondary-700 max-w-xs truncate">{p.planName || '—'}</td>
                    <td className="px-4 py-3 text-secondary-600 capitalize">{p.planType.replace(/-/g, ' ')}</td>
                    <td className="px-4 py-3"><StatusBadge status={p.status} /></td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                        p.riskLevel === 'very-high' ? 'bg-red-100 text-red-700' :
                        p.riskLevel === 'high'      ? 'bg-orange-100 text-orange-700' :
                        p.riskLevel === 'moderate'  ? 'bg-yellow-100 text-yellow-700' :
                        'bg-secondary-100 text-secondary-500'
                      }`}>
                        {p.riskLevel}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{p.primaryNeed || '—'}</td>
                    <td className="px-4 py-3 text-center">
                      {p.dhbFunded ? <span className="text-xs text-green-600 font-medium">DHB</span> : <span className="text-xs text-secondary-400">—</span>}
                    </td>
                    <td className="px-4 py-3 text-center">
                      {p.consentGiven ? <span className="text-xs text-green-600 font-medium">Yes</span> : <span className="text-xs text-red-500">No</span>}
                    </td>
                    <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(p.startedAt).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {subTab === 'visits' && (
        nursingVisits.length === 0 ? <EmptyState message="No district nursing visits found." /> : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-secondary-200 text-sm">
              <thead className="bg-secondary-50">
                <tr>
                  {['NHI', 'Visit Type', 'Status', 'Follow-up', 'Scheduled', 'Completed'].map((h) => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-secondary-100">
                {nursingVisits.map((v) => (
                  <tr key={v.id} className="hover:bg-secondary-50">
                    <td className="px-4 py-3 font-mono text-secondary-700">{v.patientNhi || '—'}</td>
                    <td className="px-4 py-3 text-secondary-600 capitalize">{v.visitType}</td>
                    <td className="px-4 py-3"><StatusBadge status={v.status} /></td>
                    <td className="px-4 py-3 text-center">
                      {v.followUpRequired
                        ? <span className="inline-block h-2 w-2 rounded-full bg-orange-400" title="Follow-up required" />
                        : <span className="inline-block h-2 w-2 rounded-full bg-secondary-200" />}
                    </td>
                    <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(v.scheduledAt).toLocaleDateString('en-NZ')}</td>
                    <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{v.completedAt ? new Date(v.completedAt).toLocaleDateString('en-NZ') : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}
    </div>
  );
}

function OutreachTab({ programmes, events }: { programmes: OutreachProgramme[]; events: OutreachEvent[] }) {
  const [subTab, setSubTab] = useState<'programmes' | 'events'>('programmes');
  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {(['programmes', 'events'] as const).map((t) => (
          <button
            key={t}
            onClick={() => setSubTab(t)}
            className={`px-3 py-1.5 rounded text-xs font-medium transition-colors ${
              subTab === t ? 'bg-primary-100 text-primary-700' : 'text-secondary-500 hover:text-secondary-800 hover:bg-secondary-100'
            }`}
          >
            {t === 'programmes' ? `Programmes (${programmes.length})` : `Events (${events.length})`}
          </button>
        ))}
      </div>

      {subTab === 'programmes' && (
        programmes.length === 0 ? <EmptyState message="No outreach programmes found." /> : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-secondary-200 text-sm">
              <thead className="bg-secondary-50">
                <tr>
                  {['Programme', 'Type', 'Target Population', 'Status', 'Coordinator', 'Start Date', 'End Date'].map((h) => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-secondary-100">
                {programmes.map((p) => (
                  <tr key={p.id} className="hover:bg-secondary-50">
                    <td className="px-4 py-3 text-secondary-700 font-medium max-w-xs truncate">{p.programmeName}</td>
                    <td className="px-4 py-3 text-secondary-600 capitalize">{p.programmeType.replace(/-/g, ' ')}</td>
                    <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{p.targetPopulation || '—'}</td>
                    <td className="px-4 py-3"><StatusBadge status={p.status} /></td>
                    <td className="px-4 py-3 font-mono text-secondary-500 text-xs">{p.coordinatorHpi || '—'}</td>
                    <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(p.startDate).toLocaleDateString('en-NZ')}</td>
                    <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{p.endDate ? new Date(p.endDate).toLocaleDateString('en-NZ') : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {subTab === 'events' && (
        events.length === 0 ? <EmptyState message="No outreach events found." /> : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-secondary-200 text-sm">
              <thead className="bg-secondary-50">
                <tr>
                  {['Event', 'Type', 'Location', 'Status', 'Attendees', 'Scheduled'].map((h) => (
                    <th key={h} className="px-4 py-3 text-left text-xs font-semibold text-secondary-500 uppercase tracking-wider">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-secondary-100">
                {events.map((e) => (
                  <tr key={e.id} className="hover:bg-secondary-50">
                    <td className="px-4 py-3 text-secondary-700 font-medium max-w-xs truncate">{e.eventName}</td>
                    <td className="px-4 py-3 text-secondary-600 capitalize">{e.eventType.replace(/-/g, ' ')}</td>
                    <td className="px-4 py-3 text-secondary-600 max-w-xs truncate">{e.location || '—'}</td>
                    <td className="px-4 py-3"><StatusBadge status={e.status} /></td>
                    <td className="px-4 py-3 text-secondary-600">
                      {e.targetAttendees != null
                        ? `${e.actualAttendees} / ${e.targetAttendees}`
                        : e.actualAttendees}
                    </td>
                    <td className="px-4 py-3 text-secondary-500 whitespace-nowrap">{new Date(e.scheduledAt).toLocaleDateString('en-NZ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------
export default function CommunityHealthPage() {
  const [activeTab, setActiveTab]         = useState<TabId>('home-visits');
  const [homeVisits, setHomeVisits]       = useState<HomeVisit[]>([]);
  const [carePlans, setCarePlans]         = useState<CarePlan[]>([]);
  const [nursingVisits, setNursingVisits] = useState<NursingVisit[]>([]);
  const [programmes, setProgrammes]       = useState<OutreachProgramme[]>([]);
  const [events, setEvents]               = useState<OutreachEvent[]>([]);
  const [loading, setLoading]             = useState(true);
  const [error, setError]                 = useState<string | null>(null);

  useEffect(() => {
    const base = '/api/v1/community';
    setLoading(true);
    setError(null);
    Promise.all([
      fetch(`${base}/home-visits`).then((r) => r.json()),
      fetch(`${base}/district-nursing/care-plans`).then((r) => r.json()),
      fetch(`${base}/outreach/programmes`).then((r) => r.json()),
    ])
      .then(([hv, cp, op]) => {
        setHomeVisits(Array.isArray(hv) ? hv : []);
        setCarePlans(Array.isArray(cp) ? cp : []);
        setProgrammes(Array.isArray(op) ? op : []);
        // Nursing visits and events are nested — not fetched at page load.
        setNursingVisits([]);
        setEvents([]);
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Failed to load community health data'))
      .finally(() => setLoading(false));
  }, []);

  return (
    <AppShell title="Community Health">
      <div className="p-6 space-y-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-secondary-900">Community Health</h1>
            <p className="mt-1 text-sm text-secondary-500">
              Home visit scheduling and documentation, district nursing care plans, and community outreach programmes
            </p>
          </div>
        </div>

        {/* Summary cards */}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          {[
            { label: 'Home Visits',    count: homeVisits.length },
            { label: 'Care Plans',     count: carePlans.length },
            { label: 'Nursing Visits', count: nursingVisits.length },
            { label: 'Programmes',     count: programmes.length },
            { label: 'Events',         count: events.length },
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
                {activeTab === 'home-visits'      && <HomeVisitsTab     items={homeVisits} />}
                {activeTab === 'district-nursing' && <DistrictNursingTab carePlans={carePlans} nursingVisits={nursingVisits} />}
                {activeTab === 'outreach'         && <OutreachTab        programmes={programmes} events={events} />}
              </>
            )}
          </div>
        </div>
      </div>
    </AppShell>
  );
}
