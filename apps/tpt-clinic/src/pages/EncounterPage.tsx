import React, { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface Coding {
  system: string;
  code: string;
  display: string;
}

interface Observation {
  id: string;
  code: Coding;
  value: string;
  unit?: string;
  effectiveDateTime: string;
  interpretation?: string;
}

interface EncounterDetail {
  id: string;
  patientId: string;
  patientName: string;
  patientNhiDisplay: string;
  practitionerId: string;
  practitionerName: string;
  date: string;
  status: string;
  type: string;
  reasonCode?: Coding;
  notes: string;
  observations: Observation[];
  diagnoses: Array<{
    id: string;
    code: Coding;
    rank: number;
    use: string;
  }>;
  prescriptionsIssued: Array<{
    id: string;
    medicationName: string;
    dose: string;
    frequency: string;
  }>;
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function EncounterPage() {
  const { id } = useParams<{ id: string }>();
  const api = useApi();

  const [encounter, setEncounter] = useState<EncounterDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    void api.get<EncounterDetail>(`/encounters/${id}`)
      .then((d) => { if (!cancelled) setEncounter(d); })
      .catch(() => { if (!cancelled) setError('Encounter not found or access denied.'); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [api, id]);

  return (
    <AppShell title="Encounter">
      {loading && (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
        </div>
      )}

      {error && (
        <div className="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {encounter && (
        <div className="space-y-5">
          {/* Header card */}
          <div className="rounded-xl bg-white p-5 shadow-sm ring-1 ring-secondary-200">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <div className="flex items-center gap-2">
                  <span className="badge-info">{encounter.status}</span>
                  <span className="text-sm text-secondary-500">{encounter.type}</span>
                </div>
                <h2 className="mt-2 text-lg font-semibold text-secondary-900">
                  {encounter.reasonCode?.display ?? 'Encounter'}
                </h2>
                <p className="text-sm text-secondary-500">
                  {new Date(encounter.date).toLocaleDateString('en-NZ', {
                    weekday: 'long',
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                  })}
                </p>
              </div>
              <div className="text-right text-sm">
                <div>
                  <span className="text-secondary-500">Patient: </span>
                  <Link
                    to={`/patients/${encounter.patientId}`}
                    className="font-medium text-primary-600 hover:underline"
                  >
                    {encounter.patientName}
                  </Link>
                  <span className="ml-1 font-mono text-xs text-secondary-400">
                    {encounter.patientNhiDisplay}
                  </span>
                </div>
                <div className="mt-1">
                  <span className="text-secondary-500">Clinician: </span>
                  <span className="font-medium text-secondary-800">{encounter.practitionerName}</span>
                </div>
              </div>
            </div>
          </div>

          {/* Clinical notes */}
          <section className="rounded-xl bg-white p-5 shadow-sm ring-1 ring-secondary-200">
            <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-secondary-500">
              Clinical Notes
            </h3>
            {encounter.notes ? (
              <p className="whitespace-pre-wrap text-sm text-secondary-800">{encounter.notes}</p>
            ) : (
              <p className="text-sm text-secondary-400 italic">No notes recorded.</p>
            )}
          </section>

          {/* Diagnoses */}
          {encounter.diagnoses.length > 0 && (
            <section className="rounded-xl bg-white p-5 shadow-sm ring-1 ring-secondary-200">
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-secondary-500">
                Diagnoses
              </h3>
              <ul className="divide-y divide-secondary-100">
                {encounter.diagnoses.map((dx) => (
                  <li key={dx.id} className="flex items-start gap-3 py-2.5">
                    <span className="mt-0.5 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-secondary-200 text-xs font-bold text-secondary-600">
                      {dx.rank}
                    </span>
                    <div>
                      <span className="text-sm font-medium text-secondary-900">{dx.code.display}</span>
                      <span className="ml-2 font-mono text-xs text-secondary-400">{dx.code.code}</span>
                      <span className="ml-2 text-xs text-secondary-400">({dx.use})</span>
                    </div>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {/* Observations */}
          {encounter.observations.length > 0 && (
            <section className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
              <div className="border-b border-secondary-100 px-5 py-3">
                <h3 className="text-sm font-semibold uppercase tracking-wide text-secondary-500">
                  Observations
                </h3>
              </div>
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                  <tr>
                    <th className="px-4 py-3 text-left">Measurement</th>
                    <th className="px-4 py-3 text-left">Value</th>
                    <th className="px-4 py-3 text-left">Time</th>
                    <th className="px-4 py-3 text-left">Interpretation</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {encounter.observations.map((obs) => (
                    <tr key={obs.id} className="hover:bg-secondary-50">
                      <td className="px-4 py-3 font-medium text-secondary-900">{obs.code.display}</td>
                      <td className="px-4 py-3 text-secondary-700">
                        {obs.value}
                        {obs.unit && <span className="ml-1 text-xs text-secondary-400">{obs.unit}</span>}
                      </td>
                      <td className="px-4 py-3 text-secondary-500">
                        {new Date(obs.effectiveDateTime).toLocaleTimeString('en-NZ', {
                          hour: '2-digit',
                          minute: '2-digit',
                        })}
                      </td>
                      <td className="px-4 py-3">
                        {obs.interpretation ? (
                          <span
                            className={
                              obs.interpretation === 'H' || obs.interpretation === 'HH'
                                ? 'badge-urgent'
                                : obs.interpretation === 'L' || obs.interpretation === 'LL'
                                  ? 'badge-warning'
                                  : 'badge-safe'
                            }
                          >
                            {obs.interpretation}
                          </span>
                        ) : (
                          <span className="text-secondary-400">—</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          )}

          {/* Prescriptions issued */}
          {encounter.prescriptionsIssued.length > 0 && (
            <section className="rounded-xl bg-white p-5 shadow-sm ring-1 ring-secondary-200">
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-secondary-500">
                Prescriptions Issued
              </h3>
              <ul className="divide-y divide-secondary-100">
                {encounter.prescriptionsIssued.map((rx) => (
                  <li key={rx.id} className="flex items-center justify-between py-2.5">
                    <div>
                      <span className="text-sm font-medium text-secondary-900">{rx.medicationName}</span>
                      <span className="ml-2 text-sm text-secondary-500">
                        {rx.dose} — {rx.frequency}
                      </span>
                    </div>
                    <Link
                      to="/prescriptions"
                      className="text-xs font-medium text-primary-600 hover:underline"
                    >
                      View
                    </Link>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </div>
      )}
    </AppShell>
  );
}
