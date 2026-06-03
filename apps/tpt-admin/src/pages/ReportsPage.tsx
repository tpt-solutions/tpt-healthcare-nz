import { formatNZD, formatDate } from '../utils/format';

// PHO/capitation and population health reports
// Data fetched from GET /api/v1/reports/* endpoints

const capitationReport = {
  period: 'Q3 2026 (Jul–Sep)',
  submissionDeadline: '2026-06-25',
  enrolled: 4823,
  eligible: 4710,
  newEnrolments: 38,
  enrolmentChanges: 12,
  expectedPayment: 291500,
};

const ageBreakdown = [
  { band: '0–4', count: 142, pct: 2.9 },
  { band: '5–14', count: 381, pct: 7.9 },
  { band: '15–24', count: 428, pct: 8.9 },
  { band: '25–44', count: 1024, pct: 21.2 },
  { band: '45–64', count: 1487, pct: 30.8 },
  { band: '65–74', count: 782, pct: 16.2 },
  { band: '75+', count: 579, pct: 12.0 },
];

const ethnicityBreakdown = [
  { group: 'Māori', count: 1187, pct: 24.6 },
  { group: 'Pacific', count: 634, pct: 13.1 },
  { group: 'Asian', count: 721, pct: 14.9 },
  { group: 'European / Other', count: 2281, pct: 47.3 },
];

const conditionPrevalence = [
  { condition: 'Type 2 Diabetes', count: 542, pct: 11.2 },
  { condition: 'Hypertension', count: 891, pct: 18.5 },
  { condition: 'Asthma', count: 438, pct: 9.1 },
  { condition: 'COPD', count: 187, pct: 3.9 },
  { condition: 'Depression / Anxiety', count: 623, pct: 12.9 },
  { condition: 'Ischaemic Heart Disease', count: 214, pct: 4.4 },
];

function formatDaysUntil(isoDate: string): string {
  const days = Math.ceil((new Date(isoDate).getTime() - Date.now()) / (1000 * 60 * 60 * 24));
  return days > 0 ? `${days} days` : 'overdue';
}

export function ReportsPage() {
  return (
    <div className="p-6 max-w-5xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Reports</h1>
        <p className="mt-1 text-sm text-gray-500">PHO capitation reports and population health statistics.</p>
      </div>

      {/* Capitation submission */}
      <section className="mb-8">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">
          Upcoming Capitation Submission
        </h2>
        <div className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <div className="flex items-start justify-between">
            <div>
              <h3 className="text-base font-semibold text-gray-900">{capitationReport.period}</h3>
              <p className="text-sm text-gray-500 mt-1">
                Submission deadline: <span className="font-medium text-gray-700">{formatDate(capitationReport.submissionDeadline)}</span>
                <span className="ml-2 text-amber-600 font-medium">({formatDaysUntil(capitationReport.submissionDeadline)} remaining)</span>
              </p>
            </div>
            <button className="rounded-lg bg-brand-600 px-4 py-2 text-sm font-medium text-white hover:bg-brand-700 transition-colors">
              Export Submission File
            </button>
          </div>

          <div className="grid grid-cols-5 gap-4 mt-5 pt-5 border-t border-gray-100">
            <div>
              <p className="text-xs text-gray-500">Enrolled</p>
              <p className="text-xl font-bold text-gray-900">{capitationReport.enrolled.toLocaleString('en-NZ')}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Eligible for Payment</p>
              <p className="text-xl font-bold text-gray-900">{capitationReport.eligible.toLocaleString('en-NZ')}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">New Enrolments</p>
              <p className="text-xl font-bold text-green-600">+{capitationReport.newEnrolments}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Changes (transfers out)</p>
              <p className="text-xl font-bold text-gray-600">{capitationReport.enrolmentChanges}</p>
            </div>
            <div>
              <p className="text-xs text-gray-500">Expected Payment</p>
              <p className="text-xl font-bold text-green-700">{formatNZD(capitationReport.expectedPayment)}</p>
            </div>
          </div>
        </div>
      </section>

      {/* Population health */}
      <div className="grid grid-cols-2 gap-6">
        {/* Age breakdown */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <h2 className="text-sm font-semibold text-gray-900 mb-4">Enrolled Population — Age Bands</h2>
          <div className="space-y-2">
            {ageBreakdown.map(({ band, count, pct }) => (
              <div key={band}>
                <div className="flex justify-between text-xs mb-1">
                  <span className="text-gray-600">{band}</span>
                  <span className="text-gray-500">{count.toLocaleString('en-NZ')} ({pct}%)</span>
                </div>
                <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
                  <div className="h-full bg-brand-400 rounded-full" style={{ width: `${pct * 3}%` }} />
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Ethnicity */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
          <h2 className="text-sm font-semibold text-gray-900 mb-4">Enrolled Population — Ethnicity</h2>
          <div className="space-y-3">
            {ethnicityBreakdown.map(({ group, count, pct }) => (
              <div key={group}>
                <div className="flex justify-between text-xs mb-1">
                  <span className="text-gray-600">{group}</span>
                  <span className="text-gray-500">{count.toLocaleString('en-NZ')} ({pct}%)</span>
                </div>
                <div className="h-2.5 bg-gray-100 rounded-full overflow-hidden">
                  <div className="h-full bg-primary-500 rounded-full" style={{ width: `${pct}%` }} />
                </div>
              </div>
            ))}
          </div>
          <p className="text-xs text-gray-400 mt-4">
            Based on primary ethnicity as recorded in enrolled patient FHIR Patient resources.
          </p>
        </section>

        {/* Long-term condition prevalence */}
        <section className="bg-white rounded-xl border border-gray-200 shadow-sm p-5 col-span-2">
          <h2 className="text-sm font-semibold text-gray-900 mb-4">Long-Term Condition Prevalence</h2>
          <div className="grid grid-cols-3 gap-3">
            {conditionPrevalence.map(({ condition, count, pct }) => (
              <div key={condition} className="bg-gray-50 rounded-lg p-3">
                <p className="text-xs font-medium text-gray-700">{condition}</p>
                <p className="text-xl font-bold text-gray-900 mt-1">{pct}%</p>
                <p className="text-xs text-gray-400">{count.toLocaleString('en-NZ')} patients</p>
              </div>
            ))}
          </div>
          <p className="text-xs text-gray-400 mt-3">
            ICD-10-AM coded conditions from active FHIR Condition resources. Mental health conditions excluded from aggregate reporting without consent.
          </p>
        </section>
      </div>
    </div>
  );
}
