import { TreatmentPlan, ACCClaim, professionClasses, professionLabels, statusClasses } from './alliedHealthTypes';
import { StatusBadge } from './AlliedHealthComponents';

interface Props {
  plans: TreatmentPlan[];
  claims: ACCClaim[];
}

export default function AlliedHealthDashboard({ plans, claims }: Props) {
  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
        <h2 className="mb-4 text-base font-semibold text-secondary-900">Profession Distribution</h2>
        <div className="grid grid-cols-2 gap-3">
          {(Object.keys(professionLabels) as (keyof typeof professionLabels)[]).map(key => (
            <div key={key} className={`rounded-lg p-3 text-center ${professionClasses[key] ?? ''}`}>
              <p className="text-2xl font-bold">{plans.filter(p => p.profession === key).length}</p>
              <p className="mt-1 text-xs font-medium">{professionLabels[key]}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
        <h2 className="mb-4 text-base font-semibold text-secondary-900">Claim Status Overview</h2>
        <div className="grid grid-cols-2 gap-3">
          {(['accepted', 'under_review', 'submitted', 'draft'] as const).map(status => (
            <div key={status} className={`rounded-lg p-3 text-center ${statusClasses[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
              <p className="text-2xl font-bold">{claims.filter(c => c.status === status).length}</p>
              <p className="mt-1 text-xs font-medium">{status.replace(/_/g, ' ')}</p>
            </div>
          ))}
        </div>
      </div>

      <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        <h2 className="border-b border-secondary-200 px-6 py-4 text-base font-semibold text-secondary-900">Upcoming Reviews</h2>
        <ul className="divide-y divide-secondary-100">
          {plans
            .filter(p => p.status === 'active' || p.status === 'under_review')
            .sort((a, b) => a.reviewDate.localeCompare(b.reviewDate))
            .slice(0, 5)
            .map(plan => (
              <li key={plan.id} className="flex items-center justify-between px-6 py-3">
                <div>
                  <p className="text-sm font-medium text-secondary-900">{plan.patientName}</p>
                  <p className="text-xs text-secondary-500">{professionLabels[plan.profession] ?? plan.profession} · Review: {plan.reviewDate} · {plan.clinician}</p>
                </div>
                <StatusBadge status={plan.status} />
              </li>
            ))}
        </ul>
      </div>

      <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        <h2 className="border-b border-secondary-200 px-6 py-4 text-base font-semibold text-secondary-900">Expiring Claims</h2>
        <ul className="divide-y divide-secondary-100">
          {claims
            .filter(c => c.status === 'accepted')
            .sort((a, b) => a.expiryDate.localeCompare(b.expiryDate))
            .slice(0, 5)
            .map(claim => (
              <li key={claim.id} className="flex items-center justify-between px-6 py-3">
                <div>
                  <p className="text-sm font-medium text-secondary-900">{claim.patientName}</p>
                  <p className="text-xs text-secondary-500">{professionLabels[claim.claimType] ?? claim.claimType} · Expires: {claim.expiryDate} · {claim.usedSessions}/{claim.approvedSessions} sessions used</p>
                </div>
                <StatusBadge status={claim.status} />
              </li>
            ))}
        </ul>
      </div>
    </div>
  );
}
