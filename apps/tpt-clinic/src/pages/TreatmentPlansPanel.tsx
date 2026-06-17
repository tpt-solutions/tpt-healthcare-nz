import { useState } from 'react';
import { TreatmentPlan } from './alliedHealthTypes';
import { ProfessionBadge, StatusBadge, ProgressBar } from './AlliedHealthComponents';

interface Props {
  plans: TreatmentPlan[];
  onNewPlan: () => void;
}

export default function TreatmentPlansPanel({ plans, onNewPlan }: Props) {
  const [selectedPlan, setSelectedPlan] = useState<TreatmentPlan | null>(null);

  return (
    <>
      <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
          <h2 className="text-base font-semibold text-secondary-900">Treatment Plans</h2>
          <button
            onClick={onNewPlan}
            className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
          >
            + New Plan
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
              <tr>
                <th className="px-6 py-3 text-left">Patient</th>
                <th className="px-6 py-3 text-left">Profession</th>
                <th className="px-6 py-3 text-left">Clinician</th>
                <th className="px-6 py-3 text-left">Diagnosis</th>
                <th className="px-6 py-3 text-left">Status</th>
                <th className="px-6 py-3 text-left">ACC Claim</th>
                <th className="px-6 py-3 text-left">Sessions</th>
                <th className="px-6 py-3 text-left">Review</th>
                <th className="px-6 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-secondary-100">
              {plans.map(plan => (
                <tr
                  key={plan.id}
                  className="cursor-pointer hover:bg-secondary-50"
                  onClick={() => setSelectedPlan(plan)}
                >
                  <td className="px-6 py-3">
                    <p className="font-medium text-secondary-900">{plan.patientName}</p>
                    <p className="font-mono text-xs text-secondary-500">NHI: {plan.patientNHI}</p>
                  </td>
                  <td className="px-6 py-3"><ProfessionBadge profession={plan.profession} /></td>
                  <td className="px-6 py-3 text-secondary-700">{plan.clinician}</td>
                  <td className="max-w-[200px] truncate px-6 py-3 text-secondary-700">{plan.diagnosis}</td>
                  <td className="px-6 py-3"><StatusBadge status={plan.status} /></td>
                  <td className="px-6 py-3 font-mono text-xs text-secondary-600">{plan.accNumber ?? '—'}</td>
                  <td className="px-6 py-3">
                    <ProgressBar used={plan.sessionsUsed} approved={plan.sessionsApproved} profession={plan.profession} />
                  </td>
                  <td className="px-6 py-3 text-secondary-500">{plan.reviewDate}</td>
                  <td className="px-6 py-3 text-right">
                    <button
                      onClick={e => { e.stopPropagation(); setSelectedPlan(plan); }}
                      className="rounded p-1 text-secondary-400 hover:text-primary-600"
                      aria-label="View"
                    >
                      <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                        <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                      </svg>
                    </button>
                  </td>
                </tr>
              ))}
              {plans.length === 0 && (
                <tr>
                  <td colSpan={9} className="px-6 py-8 text-center text-sm text-secondary-400">No treatment plans found</td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {selectedPlan && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/50" onClick={() => setSelectedPlan(null)} />
          <div className="relative w-full max-w-2xl rounded-xl bg-white shadow-xl">
            <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
              <div className="flex items-center gap-3">
                <ProfessionBadge profession={selectedPlan.profession} />
                <h2 className="text-lg font-semibold text-secondary-900">{selectedPlan.patientName}</h2>
              </div>
              <button onClick={() => setSelectedPlan(null)} className="rounded p-1 text-secondary-400 hover:text-secondary-700">
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="grid grid-cols-2 gap-4 p-6">
              <div>
                <p className="text-xs font-medium text-secondary-500">NHI</p>
                <p className="mt-0.5 font-mono text-sm font-semibold text-secondary-900">{selectedPlan.patientNHI}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Clinician</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.clinician}</p>
              </div>
              <div className="col-span-2">
                <p className="text-xs font-medium text-secondary-500">Diagnosis</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.diagnosis}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">ACC Claim</p>
                <p className="mt-0.5 font-mono text-sm text-secondary-900">{selectedPlan.accNumber ?? 'N/A'}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Status</p>
                <div className="mt-0.5"><StatusBadge status={selectedPlan.status} /></div>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Sessions</p>
                <ProgressBar used={selectedPlan.sessionsUsed} approved={selectedPlan.sessionsApproved} profession={selectedPlan.profession} />
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Start Date</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.startDate}</p>
              </div>
              <div>
                <p className="text-xs font-medium text-secondary-500">Review Date</p>
                <p className="mt-0.5 text-sm text-secondary-900">{selectedPlan.reviewDate}</p>
              </div>
              <div className="col-span-2 border-t border-secondary-100 pt-4">
                <p className="text-xs font-medium text-secondary-500">Goals &amp; Interventions</p>
                <p className="mt-1 text-sm text-secondary-500">View detailed goals, interventions, and outcome measures in the profession-specific module.</p>
              </div>
            </div>
            <div className="flex justify-end gap-3 border-t border-secondary-200 px-6 py-4">
              <button onClick={() => setSelectedPlan(null)} className="rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50">
                Close
              </button>
              <button className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                Edit Plan
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
