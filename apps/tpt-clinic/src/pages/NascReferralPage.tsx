import { useState } from 'react';
import AppShell from '@/components/AppShell';

type ReferralStatus = 'pending' | 'accepted' | 'assessing' | 'completed' | 'declined' | 'withdrawn';
type NeedsLevel = 'low' | 'moderate' | 'high' | 'complex';
type PlanStatus = 'active' | 'expiring' | 'expired' | 'suspended' | 'closed';

interface NASCReferral {
  id: string;
  patientNhi: string;
  status: ReferralStatus;
  nascOrgCode: string;
  urgencyFlag: boolean;
  referralReason: string;
  createdAt: string;
}

interface ServicePlan {
  id: string;
  patientNhi: string;
  status: PlanStatus;
  needsLevel: NeedsLevel;
  planStartDate: string;
  nextReviewDate: string;
  totalHoursPerWeek: number;
}

const REFERRAL_STATUS_STYLES: Record<ReferralStatus, string> = {
  pending: 'bg-amber-100 text-amber-700',
  accepted: 'bg-blue-100 text-blue-700',
  assessing: 'bg-indigo-100 text-indigo-700',
  completed: 'bg-green-100 text-green-700',
  declined: 'bg-red-100 text-red-700',
  withdrawn: 'bg-secondary-100 text-secondary-600',
};

const NEEDS_LEVEL_STYLES: Record<NeedsLevel, string> = {
  complex: 'bg-red-100 text-red-700',
  high: 'bg-orange-100 text-orange-700',
  moderate: 'bg-amber-100 text-amber-700',
  low: 'bg-green-100 text-green-700',
};

const PLAN_STATUS_STYLES: Record<PlanStatus, string> = {
  active: 'bg-green-100 text-green-700',
  expiring: 'bg-amber-100 text-amber-700',
  expired: 'bg-red-100 text-red-700',
  suspended: 'bg-orange-100 text-orange-700',
  closed: 'bg-secondary-100 text-secondary-600',
};

const STUB_REFERRALS: NASCReferral[] = [
  {
    id: 'r1',
    patientNhi: 'ZHQ4021',
    status: 'assessing',
    nascOrgCode: 'NASC-AKL',
    urgencyFlag: true,
    referralReason: 'Recent fall with significant reduction in ADL function. Requires urgent needs assessment for home support.',
    createdAt: '2026-05-28',
  },
  {
    id: 'r2',
    patientNhi: 'ZAB1234',
    status: 'completed',
    nascOrgCode: 'NASC-WLG',
    urgencyFlag: false,
    referralReason: 'Progressive dementia — residential care placement assessment.',
    createdAt: '2026-05-15',
  },
];

const STUB_PLANS: ServicePlan[] = [
  {
    id: 'sp1',
    patientNhi: 'ZAB1234',
    status: 'active',
    needsLevel: 'high',
    planStartDate: '2026-06-01',
    nextReviewDate: '2026-12-01',
    totalHoursPerWeek: 21,
  },
];

type Tab = 'referrals' | 'plans';

export default function NascReferralPage() {
  const [tab, setTab] = useState<Tab>('referrals');

  return (
    <AppShell title="NASC — Needs Assessment & Service Coordination">
      <div className="space-y-4">
        {/* Tab bar */}
        <div className="flex gap-1 rounded-lg bg-secondary-100 p-1 w-fit">
          {(['referrals', 'plans'] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={[
                'rounded-md px-4 py-1.5 text-sm font-medium transition-colors capitalize',
                tab === t
                  ? 'bg-white text-secondary-900 shadow-sm'
                  : 'text-secondary-500 hover:text-secondary-700',
              ].join(' ')}
            >
              {t === 'referrals' ? 'Referrals' : 'Service Plans'}
            </button>
          ))}
        </div>

        {tab === 'referrals' && (
          <>
            <div className="flex justify-end">
              <button className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                New Referral
              </button>
            </div>
            <div className="space-y-3">
              {STUB_REFERRALS.map((ref) => (
                <div key={ref.id} className="rounded-xl border border-secondary-200 bg-white p-5">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${REFERRAL_STATUS_STYLES[ref.status]}`}>
                          {ref.status}
                        </span>
                        {ref.urgencyFlag && (
                          <span className="rounded-full bg-red-600 px-2 py-0.5 text-xs font-bold text-white">
                            URGENT
                          </span>
                        )}
                        <span className="text-xs text-secondary-500">{ref.nascOrgCode}</span>
                      </div>
                      <p className="mt-1.5 font-medium text-secondary-900">
                        Patient NHI: <span className="font-mono">{ref.patientNhi}</span>
                      </p>
                      <p className="mt-1 text-sm text-secondary-600 max-w-xl">{ref.referralReason}</p>
                      <p className="mt-1 text-xs text-secondary-400">Referred {ref.createdAt}</p>
                    </div>
                    <div className="flex gap-2 shrink-0">
                      {ref.status === 'completed' && (
                        <button className="rounded-md border border-primary-300 bg-primary-50 px-3 py-1 text-xs font-medium text-primary-700 hover:bg-primary-100">
                          Create Plan
                        </button>
                      )}
                      <button className="rounded-md border border-secondary-300 px-3 py-1 text-xs font-medium text-secondary-600 hover:bg-secondary-50">
                        View
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </>
        )}

        {tab === 'plans' && (
          <>
            <div className="flex justify-end">
              <button className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
                New Service Plan
              </button>
            </div>
            <div className="space-y-3">
              {STUB_PLANS.map((plan) => (
                <div key={plan.id} className="rounded-xl border border-secondary-200 bg-white p-5">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <div className="flex items-center gap-2">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${PLAN_STATUS_STYLES[plan.status]}`}>
                          {plan.status}
                        </span>
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${NEEDS_LEVEL_STYLES[plan.needsLevel]}`}>
                          {plan.needsLevel} needs
                        </span>
                      </div>
                      <p className="mt-1.5 font-medium text-secondary-900">
                        Patient NHI: <span className="font-mono">{plan.patientNhi}</span>
                      </p>
                      <div className="mt-2 flex gap-4 text-sm text-secondary-600">
                        <span>Start: {plan.planStartDate}</span>
                        <span>Next review: {plan.nextReviewDate}</span>
                        <span className="font-medium">{plan.totalHoursPerWeek} hrs/week</span>
                      </div>
                    </div>
                    <div className="flex gap-2 shrink-0">
                      <button className="rounded-md border border-amber-300 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700 hover:bg-amber-100">
                        Record Review
                      </button>
                      <button className="rounded-md border border-secondary-300 px-3 py-1 text-xs font-medium text-secondary-600 hover:bg-secondary-50">
                        View
                      </button>
                    </div>
                  </div>
                </div>
              ))}
              {STUB_PLANS.length === 0 && (
                <p className="rounded-xl border border-secondary-200 bg-white py-12 text-center text-sm text-secondary-500">
                  No service plans found.
                </p>
              )}
            </div>
          </>
        )}

        <aside className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          <strong>HIPC Rule 10 &amp; 11:</strong> NASC records are health information. Sharing a service plan with a provider
          requires documented consent or a valid HIPC exception. Use the consent module before disclosing.
        </aside>
      </div>
    </AppShell>
  );
}
