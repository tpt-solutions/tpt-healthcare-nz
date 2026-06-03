interface Prescription {
  id: string;
  medication: string;
  nzmtCode: string;
  dose: string;
  frequency: string;
  prescriber: string;
  issuedDate: string;
  expiryDate: string | null;
  status: 'active' | 'completed' | 'stopped';
  repeatsRemaining: number | null;
  subsidised: boolean; // PHARMAC subsidy status
  instructions: string;
}

const prescriptions: Prescription[] = [
  {
    id: 'rx-1',
    medication: 'Metformin 500 mg tablets',
    nzmtCode: '10037281000116105',
    dose: '500 mg',
    frequency: 'Twice daily with meals',
    prescriber: 'Dr. Hemi Walker',
    issuedDate: '2026-05-05',
    expiryDate: '2026-11-05',
    status: 'active',
    repeatsRemaining: 3,
    subsidised: true,
    instructions: 'Take one tablet twice daily with food. Do not crush or chew.',
  },
  {
    id: 'rx-2',
    medication: 'Lisinopril 10 mg tablets',
    nzmtCode: '10119001000116100',
    dose: '10 mg',
    frequency: 'Once daily in the morning',
    prescriber: 'Dr. Hemi Walker',
    issuedDate: '2026-04-12',
    expiryDate: '2026-10-12',
    status: 'active',
    repeatsRemaining: 5,
    subsidised: true,
    instructions: 'Take one tablet each morning. May cause dizziness initially.',
  },
  {
    id: 'rx-3',
    medication: 'Amoxicillin 500 mg capsules',
    nzmtCode: '10011591000116107',
    dose: '500 mg',
    frequency: 'Three times daily for 7 days',
    prescriber: 'Dr. Hemi Walker',
    issuedDate: '2025-11-20',
    expiryDate: '2025-11-27',
    status: 'completed',
    repeatsRemaining: 0,
    subsidised: true,
    instructions: 'Complete the full course even if you feel better.',
  },
];

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-NZ', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

function statusBadge(status: Prescription['status']) {
  const map = {
    active: 'bg-green-100 text-green-700',
    completed: 'bg-gray-100 text-gray-600',
    stopped: 'bg-red-100 text-red-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${map[status]}`}>
      {status}
    </span>
  );
}

export function PrescriptionsPage() {
  const active = prescriptions.filter(rx => rx.status === 'active');
  const history = prescriptions.filter(rx => rx.status !== 'active');

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">My Prescriptions</h1>
        <p className="mt-1 text-sm text-gray-500">
          Current and past prescriptions issued by your care team.
        </p>
      </div>

      {/* PHARMAC note */}
      <div className="bg-green-50 border border-green-200 rounded-xl px-4 py-3 mb-6">
        <p className="text-xs text-green-800">
          <span className="font-semibold">PHARMAC-subsidised medicines</span> are available at your pharmacy for a
          standard $5 co-payment per item. Confirm subsidy eligibility with your pharmacist.
        </p>
      </div>

      {/* Active prescriptions */}
      <section className="mb-8">
        <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">
          Active Prescriptions ({active.length})
        </h2>
        <div className="space-y-3">
          {active.map(rx => (
            <div key={rx.id} className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center gap-3 mb-1">
                    <h3 className="text-sm font-semibold text-gray-900">{rx.medication}</h3>
                    {statusBadge(rx.status)}
                    {rx.subsidised && (
                      <span className="inline-flex rounded-full bg-green-100 text-green-700 px-2.5 py-0.5 text-xs font-medium">
                        PHARMAC Subsidised
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-gray-400 mb-2">
                    NZMT: <code className="bg-gray-100 rounded px-1">{rx.nzmtCode}</code>
                  </p>
                  <p className="text-sm text-gray-600">{rx.dose} &mdash; {rx.frequency}</p>
                  <p className="text-xs text-gray-500 mt-1 bg-gray-50 rounded-lg px-3 py-2">
                    {rx.instructions}
                  </p>
                  <div className="flex gap-4 mt-2">
                    <p className="text-xs text-gray-400">Prescribed by {rx.prescriber}</p>
                    {rx.repeatsRemaining !== null && (
                      <p className="text-xs text-gray-400">{rx.repeatsRemaining} repeats remaining</p>
                    )}
                  </div>
                </div>
                <div className="ml-6 text-right flex-shrink-0">
                  <p className="text-xs text-gray-500">Issued</p>
                  <p className="text-xs font-medium text-gray-700">{formatDate(rx.issuedDate)}</p>
                  {rx.expiryDate && (
                    <>
                      <p className="text-xs text-gray-500 mt-1">Expires</p>
                      <p className="text-xs font-medium text-gray-700">{formatDate(rx.expiryDate)}</p>
                    </>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Prescription history */}
      {history.length > 0 && (
        <section>
          <h2 className="text-sm font-semibold text-gray-700 uppercase tracking-wide mb-3">
            Past Prescriptions
          </h2>
          <div className="space-y-3">
            {history.map(rx => (
              <div key={rx.id} className="bg-white rounded-xl border border-gray-100 p-5 opacity-75">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="flex items-center gap-3">
                      <h3 className="text-sm font-medium text-gray-700">{rx.medication}</h3>
                      {statusBadge(rx.status)}
                    </div>
                    <p className="text-xs text-gray-400 mt-0.5">{rx.dose} &mdash; {rx.frequency}</p>
                    <p className="text-xs text-gray-400">Prescribed by {rx.prescriber}</p>
                  </div>
                  <p className="text-xs text-gray-400">{formatDate(rx.issuedDate)}</p>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
