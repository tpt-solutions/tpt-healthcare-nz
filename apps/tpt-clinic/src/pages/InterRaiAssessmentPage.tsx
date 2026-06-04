import { useState } from 'react';
import AppShell from '@/components/AppShell';

type Instrument = 'HC' | 'LTCF' | 'CA' | 'CHA' | 'PAC';
type Status = 'draft' | 'submitted' | 'amended';

interface Assessment {
  id: string;
  patientNhi: string;
  instrument: Instrument;
  status: Status;
  assessedAt: string;
  practitionerHpi: string;
  scales: {
    adlHierarchy?: number;
    cps?: number;
    drs?: number;
    pain?: number;
    chess?: number;
  };
}

const INSTRUMENT_LABELS: Record<Instrument, string> = {
  HC: 'Home Care',
  LTCF: 'Long-Term Care Facility',
  CA: 'Contact Assessment',
  CHA: 'Community Health Assessment',
  PAC: 'Post-Acute Care',
};

const STATUS_STYLES: Record<Status, string> = {
  draft: 'bg-amber-100 text-amber-700',
  submitted: 'bg-green-100 text-green-700',
  amended: 'bg-blue-100 text-blue-700',
};

// Stub data — replaced by API calls once the backend is wired to the API client.
const STUB_ASSESSMENTS: Assessment[] = [
  {
    id: '1',
    patientNhi: 'ZHQ4021',
    instrument: 'HC',
    status: 'draft',
    assessedAt: '2026-06-04',
    practitionerHpi: 'HPI-CPN-00123',
    scales: { adlHierarchy: 2, cps: 1, drs: 4, pain: 1 },
  },
  {
    id: '2',
    patientNhi: 'ZAB1234',
    instrument: 'LTCF',
    status: 'submitted',
    assessedAt: '2026-06-01',
    practitionerHpi: 'HPI-CPN-00456',
    scales: { adlHierarchy: 4, cps: 3, drs: 6, chess: 2 },
  },
];

function ScaleBadge({ label, value, max }: { label: string; value?: number; max: number }) {
  if (value === undefined) return null;
  const pct = (value / max) * 100;
  const color = pct >= 66 ? 'bg-red-400' : pct >= 33 ? 'bg-amber-400' : 'bg-green-400';
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="w-10 shrink-0 text-secondary-500">{label}</span>
      <div className="h-1.5 w-20 rounded-full bg-secondary-200">
        <div className={`h-1.5 rounded-full ${color}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="font-medium text-secondary-700">{value}/{max}</span>
    </div>
  );
}

export default function InterRaiAssessmentPage() {
  const [filter, setFilter] = useState<Instrument | ''>('');
  const [statusFilter, setStatusFilter] = useState<Status | ''>('');

  const visible = STUB_ASSESSMENTS.filter(
    (a) =>
      (filter === '' || a.instrument === filter) &&
      (statusFilter === '' || a.status === statusFilter),
  );

  return (
    <AppShell title="interRAI Assessments">
      <div className="space-y-4">
        {/* Toolbar */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex gap-2">
            <select
              value={filter}
              onChange={(e) => setFilter(e.target.value as Instrument | '')}
              className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <option value="">All instruments</option>
              {(Object.keys(INSTRUMENT_LABELS) as Instrument[]).map((k) => (
                <option key={k} value={k}>{k} — {INSTRUMENT_LABELS[k]}</option>
              ))}
            </select>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as Status | '')}
              className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <option value="">All statuses</option>
              <option value="draft">Draft</option>
              <option value="submitted">Submitted</option>
              <option value="amended">Amended</option>
            </select>
          </div>
          <button className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
            New Assessment
          </button>
        </div>

        {/* Assessment cards */}
        <div className="space-y-3">
          {visible.length === 0 && (
            <p className="rounded-xl border border-secondary-200 bg-white py-12 text-center text-sm text-secondary-500">
              No assessments found.
            </p>
          )}
          {visible.map((a) => (
            <div
              key={a.id}
              className="rounded-xl border border-secondary-200 bg-white p-5 hover:border-primary-300 transition-colors"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2">
                    <span className="rounded bg-blue-100 px-2 py-0.5 text-xs font-bold text-blue-700">
                      {a.instrument}
                    </span>
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_STYLES[a.status]}`}>
                      {a.status}
                    </span>
                  </div>
                  <p className="mt-1.5 font-medium text-secondary-900">
                    Patient NHI: <span className="font-mono">{a.patientNhi}</span>
                  </p>
                  <p className="text-xs text-secondary-500 mt-0.5">
                    {INSTRUMENT_LABELS[a.instrument]} &bull; Assessed {a.assessedAt} &bull; {a.practitionerHpi}
                  </p>
                </div>

                <div className="flex gap-2">
                  {a.status === 'draft' && (
                    <button className="rounded-md border border-green-300 bg-green-50 px-3 py-1 text-xs font-medium text-green-700 hover:bg-green-100">
                      Submit
                    </button>
                  )}
                  <button className="rounded-md border border-secondary-300 px-3 py-1 text-xs font-medium text-secondary-600 hover:bg-secondary-50">
                    View CAPs
                  </button>
                  <button className="rounded-md border border-secondary-300 px-3 py-1 text-xs font-medium text-secondary-600 hover:bg-secondary-50">
                    Edit
                  </button>
                </div>
              </div>

              {/* Scale scores */}
              <div className="mt-4 flex flex-wrap gap-4 rounded-lg bg-secondary-50 px-4 py-3">
                <ScaleBadge label="ADL" value={a.scales.adlHierarchy} max={6} />
                <ScaleBadge label="CPS" value={a.scales.cps} max={6} />
                <ScaleBadge label="DRS" value={a.scales.drs} max={14} />
                <ScaleBadge label="Pain" value={a.scales.pain} max={3} />
                <ScaleBadge label="CHESS" value={a.scales.chess} max={18} />
              </div>
            </div>
          ))}
        </div>

        {/* interRAI info banner */}
        <aside className="rounded-xl border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-800">
          <strong>interRAI NZ:</strong> All assessments must be submitted to the MoH interRAI repository after completion.
          Draft assessments are locked on submission. Use the amend workflow for corrections post-submission.
        </aside>
      </div>
    </AppShell>
  );
}
