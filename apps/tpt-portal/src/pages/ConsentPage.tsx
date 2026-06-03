import { useState } from 'react';

/**
 * Consent management page — implements HIPC Rule 10 (use) and Rule 11 (disclosure).
 *
 * Consent records map to FHIR R5 Consent resources managed by core/consent/.
 * Changes here call POST /api/v1/fhir/Consent via @tpt/api-client and are
 * audited automatically by the consent middleware.
 */

type ConsentStatus = 'granted' | 'revoked';

interface ConsentItem {
  id: string;
  title: string;
  rule: 'rule10' | 'rule11';
  description: string;
  detail: string;
  status: ConsentStatus;
  grantedDate: string | null;
  revokedDate: string | null;
}

const initialConsents: ConsentItem[] = [
  {
    id: 'consent-sharing-gp',
    title: 'Share records with my GP',
    rule: 'rule11',
    description: 'Allows your registered GP to access your full health record held by TPT Healthcare.',
    detail:
      'Under HIPC Rule 11, health information may only be disclosed to third parties with your consent or under a specific exception. This consent allows your General Practitioner to view your encounters, diagnoses, medications, and test results within TPT Healthcare.',
    status: 'granted',
    grantedDate: '2024-01-15',
    revokedDate: null,
  },
  {
    id: 'consent-research',
    title: 'Anonymised research and quality improvement',
    rule: 'rule10',
    description:
      'Allows de-identified data from your records to be used for approved health research and PHO quality improvement.',
    detail:
      'Under HIPC Rule 10, health information must only be used for the purpose for which it was collected unless you consent otherwise. This consent allows de-identified (not personally identifiable) data to contribute to anonymised health research approved by an ethics committee, and to PHO quality reporting.',
    status: 'granted',
    grantedDate: '2024-01-15',
    revokedDate: null,
  },
  {
    id: 'consent-specialist-referral',
    title: 'Share records with specialists on referral',
    rule: 'rule11',
    description:
      'Allows specialists you are referred to by your GP to access the relevant parts of your health record.',
    detail:
      'When your GP refers you to a specialist, this consent permits the specialist to access the portions of your TPT Healthcare record that are relevant to the referral. Access is limited to the referred specialty and the period of that care episode.',
    status: 'granted',
    grantedDate: '2024-01-15',
    revokedDate: null,
  },
  {
    id: 'consent-pharmacy',
    title: 'Share medication history with pharmacies',
    rule: 'rule11',
    description:
      'Allows pharmacies dispensing your prescriptions to view your current medication list to check for interactions.',
    detail:
      'Pharmacists are required to check for drug interactions before dispensing. This consent allows your dispensing pharmacy to view your current medication list held in TPT Healthcare to support safe dispensing.',
    status: 'revoked',
    grantedDate: '2024-01-15',
    revokedDate: '2025-08-20',
  },
  {
    id: 'consent-mental-health',
    title: 'Include mental health records in sharing',
    rule: 'rule11',
    description:
      'Elevates mental health record sharing for care providers (off by default — extra-sensitive under HIPC).',
    detail:
      'Mental health records carry an extra-sensitive classification under the Health Information Privacy Code. By default, they are excluded from all sharing consents. Enabling this allows your mental health records to be shared with care providers under the same rules as your other health records. You may turn this off at any time.',
    status: 'revoked',
    grantedDate: null,
    revokedDate: null,
  },
];

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString('en-NZ', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  });
}

function RuleBadge({ rule }: { rule: ConsentItem['rule'] }) {
  return rule === 'rule11' ? (
    <span className="inline-flex rounded-full bg-purple-100 text-purple-700 px-2 py-0.5 text-xs font-medium">
      HIPC Rule 11 — Disclosure
    </span>
  ) : (
    <span className="inline-flex rounded-full bg-indigo-100 text-indigo-700 px-2 py-0.5 text-xs font-medium">
      HIPC Rule 10 — Use
    </span>
  );
}

export function ConsentPage() {
  const [consents, setConsents] = useState(initialConsents);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [pendingAction, setPendingAction] = useState<{ id: string; action: ConsentStatus } | null>(null);

  const toggle = (id: string) => {
    setExpandedId(prev => (prev === id ? null : id));
  };

  const confirmAction = () => {
    if (!pendingAction) return;
    const now = new Date().toISOString().slice(0, 10);
    setConsents(prev =>
      prev.map(c => {
        if (c.id !== pendingAction.id) return c;
        if (pendingAction.action === 'granted') {
          return { ...c, status: 'granted', grantedDate: now, revokedDate: null };
        }
        return { ...c, status: 'revoked', revokedDate: now };
      }),
    );
    // TODO: call POST /api/v1/fhir/Consent via @tpt/api-client to persist
    setPendingAction(null);
  };

  const granted = consents.filter(c => c.status === 'granted');
  const revoked = consents.filter(c => c.status === 'revoked');

  return (
    <div className="p-6 max-w-3xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Manage My Consent</h1>
        <p className="mt-1 text-sm text-gray-500">
          Control how your health information is used and shared, as required by HIPC Rules 10 and 11.
        </p>
      </div>

      {/* Explainer */}
      <div className="bg-primary-50 border border-primary-200 rounded-xl px-5 py-4 mb-8">
        <h2 className="text-sm font-semibold text-primary-900 mb-1">Your rights under the Health Information Privacy Code</h2>
        <p className="text-xs text-primary-800 leading-relaxed">
          <span className="font-semibold">Rule 10</span> requires that your health information is only used for the
          purpose for which it was collected.{' '}
          <span className="font-semibold">Rule 11</span> requires that it is not disclosed to others without your
          consent or a lawful reason. You may grant or revoke these consents at any time. Revocation takes effect
          immediately and is recorded in your audit trail.
        </p>
      </div>

      {/* Stats */}
      <div className="flex gap-4 mb-6">
        <div className="flex-1 bg-green-50 border border-green-200 rounded-xl px-4 py-3 text-center">
          <p className="text-2xl font-bold text-green-700">{granted.length}</p>
          <p className="text-xs text-green-600 font-medium">Consents Granted</p>
        </div>
        <div className="flex-1 bg-gray-50 border border-gray-200 rounded-xl px-4 py-3 text-center">
          <p className="text-2xl font-bold text-gray-600">{revoked.length}</p>
          <p className="text-xs text-gray-500 font-medium">Consents Revoked / Not Granted</p>
        </div>
      </div>

      {/* Consent list */}
      <div className="space-y-3">
        {consents.map(consent => {
          const isExpanded = expandedId === consent.id;
          const isGranted = consent.status === 'granted';

          return (
            <div
              key={consent.id}
              className={`bg-white rounded-xl border shadow-sm overflow-hidden transition-colors ${
                isGranted ? 'border-green-200' : 'border-gray-200'
              }`}
            >
              {/* Row */}
              <div className="flex items-center justify-between px-5 py-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3 flex-wrap">
                    <h3 className="text-sm font-semibold text-gray-900">{consent.title}</h3>
                    <RuleBadge rule={consent.rule} />
                  </div>
                  <p className="text-xs text-gray-500 mt-1">{consent.description}</p>
                  {isGranted && consent.grantedDate && (
                    <p className="text-xs text-green-600 mt-0.5">Granted {formatDate(consent.grantedDate)}</p>
                  )}
                  {!isGranted && consent.revokedDate && (
                    <p className="text-xs text-gray-400 mt-0.5">Revoked {formatDate(consent.revokedDate)}</p>
                  )}
                </div>
                <div className="flex items-center gap-3 ml-4 flex-shrink-0">
                  <button
                    onClick={() => toggle(consent.id)}
                    className="text-xs text-brand-600 hover:underline"
                  >
                    {isExpanded ? 'Less' : 'Details'}
                  </button>
                  {isGranted ? (
                    <button
                      onClick={() => setPendingAction({ id: consent.id, action: 'revoked' })}
                      className="rounded-lg border border-red-200 bg-red-50 px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-100 transition-colors"
                    >
                      Revoke
                    </button>
                  ) : (
                    <button
                      onClick={() => setPendingAction({ id: consent.id, action: 'granted' })}
                      className="rounded-lg border border-green-200 bg-green-50 px-3 py-1.5 text-xs font-medium text-green-700 hover:bg-green-100 transition-colors"
                    >
                      Grant
                    </button>
                  )}
                </div>
              </div>

              {/* Expanded detail */}
              {isExpanded && (
                <div className="px-5 pb-4 border-t border-gray-100 bg-gray-50">
                  <p className="text-xs text-gray-600 leading-relaxed mt-3">{consent.detail}</p>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Confirm modal */}
      {pendingAction && (() => {
        const item = consents.find(c => c.id === pendingAction.id);
        if (!item) return null;
        const isGrant = pendingAction.action === 'granted';
        return (
          <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
            <div className="bg-white rounded-2xl shadow-xl p-6 w-full max-w-sm mx-4">
              <h2 className="text-base font-semibold text-gray-900 mb-2">
                {isGrant ? 'Grant consent?' : 'Revoke consent?'}
              </h2>
              <p className="text-sm text-gray-600 mb-1 font-medium">{item.title}</p>
              <p className="text-sm text-gray-500 mb-5">{item.description}</p>
              {!isGrant && (
                <div className="bg-amber-50 border border-amber-200 rounded-lg px-3 py-2 mb-5">
                  <p className="text-xs text-amber-700">
                    Revoking this consent takes effect immediately. Care providers who previously had access may lose it.
                    This action is recorded in your audit trail.
                  </p>
                </div>
              )}
              <div className="flex gap-3">
                <button
                  onClick={() => setPendingAction(null)}
                  className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={confirmAction}
                  className={`flex-1 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors ${
                    isGrant ? 'bg-green-600 hover:bg-green-700' : 'bg-red-600 hover:bg-red-700'
                  }`}
                >
                  {isGrant ? 'Grant consent' : 'Revoke consent'}
                </button>
              </div>
            </div>
          </div>
        );
      })()}
    </div>
  );
}
