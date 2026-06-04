import { useState } from 'react';
import AppShell from '@/components/AppShell';

type PlanType = 'residential' | 'home-care' | 'day-programme' | 'respite';
type PlanStatus = 'active' | 'on-hold' | 'completed' | 'revoked';
type GoalStatus = 'in-progress' | 'achieved' | 'abandoned' | 'on-hold';

interface Goal {
  id: string;
  description: string;
  status: GoalStatus;
  targetDate?: string;
}

interface CarePlan {
  id: string;
  patientNhi: string;
  planType: PlanType;
  status: PlanStatus;
  responsibleHpi: string;
  startDate: string;
  nextReviewDate: string;
  facilityName?: string;
  goals: Goal[];
  interventionCount: number;
}

const PLAN_TYPE_LABELS: Record<PlanType, string> = {
  residential: 'Residential Care',
  'home-care': 'Home Care',
  'day-programme': 'Day Programme',
  respite: 'Respite',
};

const PLAN_TYPE_COLORS: Record<PlanType, string> = {
  residential: 'bg-purple-100 text-purple-700',
  'home-care': 'bg-green-100 text-green-700',
  'day-programme': 'bg-blue-100 text-blue-700',
  respite: 'bg-amber-100 text-amber-700',
};

const PLAN_STATUS_STYLES: Record<PlanStatus, string> = {
  active: 'bg-green-100 text-green-700',
  'on-hold': 'bg-amber-100 text-amber-700',
  completed: 'bg-secondary-100 text-secondary-600',
  revoked: 'bg-red-100 text-red-700',
};

const GOAL_STATUS_STYLES: Record<GoalStatus, string> = {
  'in-progress': 'bg-blue-100 text-blue-700',
  achieved: 'bg-green-100 text-green-700',
  abandoned: 'bg-red-100 text-red-700',
  'on-hold': 'bg-amber-100 text-amber-700',
};

const STUB_PLANS: CarePlan[] = [
  {
    id: 'cp1',
    patientNhi: 'ZHQ4021',
    planType: 'home-care',
    status: 'active',
    responsibleHpi: 'HPI-CPN-00123',
    startDate: '2026-04-01',
    nextReviewDate: '2026-10-01',
    goals: [
      { id: 'g1', description: 'Maintain safe mobility at home with walker', status: 'in-progress', targetDate: '2026-09-01' },
      { id: 'g2', description: 'Improve medication self-management', status: 'achieved' },
    ],
    interventionCount: 4,
  },
  {
    id: 'cp2',
    patientNhi: 'ZAB1234',
    planType: 'residential',
    status: 'active',
    responsibleHpi: 'HPI-CPN-00456',
    startDate: '2026-06-01',
    nextReviewDate: '2026-09-01',
    facilityName: 'Ryman Healthcare — Ilam',
    goals: [
      { id: 'g3', description: 'Maintain social engagement in facility activities', status: 'in-progress' },
      { id: 'g4', description: 'Reduce fall risk through physiotherapy', status: 'in-progress', targetDate: '2026-08-01' },
    ],
    interventionCount: 6,
  },
];

export default function CarePlanPage() {
  const [typeFilter, setTypeFilter] = useState<PlanType | ''>('');
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const visible = STUB_PLANS.filter((p) => typeFilter === '' || p.planType === typeFilter);

  const isOverdueReview = (date: string) => new Date(date) < new Date('2026-06-05');

  return (
    <AppShell title="Care Plans">
      <div className="space-y-4">
        {/* Toolbar */}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value as PlanType | '')}
            className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
          >
            <option value="">All plan types</option>
            {(Object.keys(PLAN_TYPE_LABELS) as PlanType[]).map((k) => (
              <option key={k} value={k}>{PLAN_TYPE_LABELS[k]}</option>
            ))}
          </select>
          <button className="rounded-md bg-primary-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
            New Care Plan
          </button>
        </div>

        {/* Plans */}
        <div className="space-y-3">
          {visible.length === 0 && (
            <p className="rounded-xl border border-secondary-200 bg-white py-12 text-center text-sm text-secondary-500">
              No care plans found.
            </p>
          )}
          {visible.map((plan) => {
            const expanded = expandedId === plan.id;
            const reviewOverdue = isOverdueReview(plan.nextReviewDate);

            return (
              <div key={plan.id} className="rounded-xl border border-secondary-200 bg-white overflow-hidden">
                {/* Header row */}
                <div
                  className="flex cursor-pointer flex-wrap items-start justify-between gap-3 p-5 hover:bg-secondary-50 transition-colors"
                  onClick={() => setExpandedId(expanded ? null : plan.id)}
                >
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${PLAN_TYPE_COLORS[plan.planType]}`}>
                        {PLAN_TYPE_LABELS[plan.planType]}
                      </span>
                      <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${PLAN_STATUS_STYLES[plan.status]}`}>
                        {plan.status}
                      </span>
                      {reviewOverdue && (
                        <span className="rounded-full bg-red-600 px-2 py-0.5 text-xs font-bold text-white">
                          REVIEW OVERDUE
                        </span>
                      )}
                    </div>
                    <p className="mt-1.5 font-medium text-secondary-900">
                      Patient NHI: <span className="font-mono">{plan.patientNhi}</span>
                    </p>
                    {plan.facilityName && (
                      <p className="text-sm text-secondary-600">{plan.facilityName}</p>
                    )}
                    <div className="mt-1 flex gap-4 text-xs text-secondary-500">
                      <span>Start: {plan.startDate}</span>
                      <span className={reviewOverdue ? 'font-medium text-red-600' : ''}>
                        Review: {plan.nextReviewDate}
                      </span>
                      <span>{plan.goals.length} goals &bull; {plan.interventionCount} interventions</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      onClick={(e) => { e.stopPropagation(); }}
                      className="rounded-md border border-amber-300 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-700 hover:bg-amber-100"
                    >
                      Record Review
                    </button>
                    <button
                      onClick={(e) => { e.stopPropagation(); }}
                      className="rounded-md border border-secondary-300 px-3 py-1 text-xs font-medium text-secondary-600 hover:bg-secondary-50"
                    >
                      Edit
                    </button>
                    <svg
                      className={`h-4 w-4 text-secondary-400 transition-transform ${expanded ? 'rotate-180' : ''}`}
                      fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                    </svg>
                  </div>
                </div>

                {/* Expanded goals panel */}
                {expanded && (
                  <div className="border-t border-secondary-100 bg-secondary-50 px-5 py-4">
                    <div className="flex items-center justify-between mb-3">
                      <h4 className="text-sm font-semibold text-secondary-700">Goals</h4>
                      <button className="text-xs text-primary-600 hover:underline">+ Add Goal</button>
                    </div>
                    <ul className="space-y-2">
                      {plan.goals.map((g) => (
                        <li key={g.id} className="flex items-start gap-3 rounded-lg bg-white px-4 py-3 shadow-sm">
                          <span className={`mt-0.5 shrink-0 rounded-full px-2 py-0.5 text-xs font-medium ${GOAL_STATUS_STYLES[g.status]}`}>
                            {g.status}
                          </span>
                          <div className="flex-1 text-sm text-secondary-700">
                            {g.description}
                            {g.targetDate && (
                              <span className="ml-2 text-xs text-secondary-400">Target: {g.targetDate}</span>
                            )}
                          </div>
                          <button className="text-xs text-secondary-400 hover:text-secondary-700 shrink-0">
                            Update
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </AppShell>
  );
}
