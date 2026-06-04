import { Link } from 'react-router-dom';
import AppShell from '@/components/AppShell';

interface StatCard {
  label: string;
  value: string;
  sub: string;
  to: string;
  color: string;
}

const STATS: StatCard[] = [
  {
    label: 'interRAI Assessments',
    value: '—',
    sub: 'pending submission',
    to: '/aged-care/interrai',
    color: 'bg-blue-50 text-blue-700',
  },
  {
    label: 'NASC Referrals',
    value: '—',
    sub: 'awaiting assessment',
    to: '/aged-care/nasc',
    color: 'bg-amber-50 text-amber-700',
  },
  {
    label: 'Funded Hours',
    value: '—',
    sub: 'timesheets pending approval',
    to: '/aged-care/funded-hours',
    color: 'bg-green-50 text-green-700',
  },
  {
    label: 'Care Plans',
    value: '—',
    sub: 'reviews overdue',
    to: '/aged-care/care-plans',
    color: 'bg-purple-50 text-purple-700',
  },
];

const QUICK_LINKS = [
  { label: 'New interRAI Assessment', to: '/aged-care/interrai/new', icon: '📋' },
  { label: 'New NASC Referral', to: '/aged-care/nasc/referrals/new', icon: '📨' },
  { label: 'Record Timesheet', to: '/aged-care/funded-hours/timesheets/new', icon: '🕐' },
  { label: 'New Care Plan', to: '/aged-care/care-plans/new', icon: '📄' },
];

export default function AgedCareDashboard() {
  return (
    <AppShell title="Aged Care">
      <div className="space-y-6">
        {/* Overview cards */}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {STATS.map((s) => (
            <Link
              key={s.to}
              to={s.to}
              className="rounded-xl border border-secondary-200 bg-white p-5 hover:border-primary-300 hover:shadow-sm transition-all"
            >
              <p className="text-sm font-medium text-secondary-500">{s.label}</p>
              <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
              <p className={`mt-1 inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${s.color}`}>
                {s.sub}
              </p>
            </Link>
          ))}
        </div>

        {/* Quick actions */}
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-secondary-500">
            Quick Actions
          </h2>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {QUICK_LINKS.map((l) => (
              <Link
                key={l.to}
                to={l.to}
                className="flex flex-col items-center gap-2 rounded-xl border border-secondary-200 bg-white p-4 text-center text-sm font-medium text-secondary-700 hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 transition-all"
              >
                <span className="text-2xl">{l.icon}</span>
                {l.label}
              </Link>
            ))}
          </div>
        </section>

        {/* Module overview */}
        <section className="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div className="rounded-xl border border-secondary-200 bg-white p-5">
            <h3 className="mb-3 font-semibold text-secondary-900">interRAI Instruments</h3>
            <ul className="space-y-2 text-sm text-secondary-600">
              {[
                ['HC', 'Home Care — community-based recipients'],
                ['LTCF', 'Long-Term Care Facility — residential care'],
                ['CA', 'Contact Assessment — brief screening'],
                ['CHA', 'Community Health Assessment'],
                ['PAC', 'Post-Acute Care'],
              ].map(([code, desc]) => (
                <li key={code} className="flex items-start gap-2">
                  <span className="mt-0.5 rounded bg-blue-100 px-1.5 py-0.5 text-xs font-bold text-blue-700 shrink-0">
                    {code}
                  </span>
                  <span>{desc}</span>
                </li>
              ))}
            </ul>
          </div>

          <div className="rounded-xl border border-secondary-200 bg-white p-5">
            <h3 className="mb-3 font-semibold text-secondary-900">Support Needs Levels</h3>
            <ul className="space-y-2 text-sm">
              {[
                { level: 'Complex', color: 'bg-red-100 text-red-700', desc: 'Highest need, most intensive services' },
                { level: 'High', color: 'bg-orange-100 text-orange-700', desc: 'Significant support requirements' },
                { level: 'Moderate', color: 'bg-amber-100 text-amber-700', desc: 'Regular ongoing support' },
                { level: 'Low', color: 'bg-green-100 text-green-700', desc: 'Occasional or minimal support' },
              ].map(({ level, color, desc }) => (
                <li key={level} className="flex items-start gap-2">
                  <span className={`mt-0.5 rounded px-1.5 py-0.5 text-xs font-bold shrink-0 ${color}`}>
                    {level}
                  </span>
                  <span className="text-secondary-600">{desc}</span>
                </li>
              ))}
            </ul>
          </div>
        </section>
      </div>
    </AppShell>
  );
}
