import { SessionNote } from './alliedHealthTypes';
import { ProfessionBadge, StatusBadge } from './AlliedHealthComponents';

interface Props {
  sessions: SessionNote[];
  onNewSession: () => void;
}

export default function SessionNotesPanel({ sessions, onNewSession }: Props) {
  return (
    <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
        <h2 className="text-base font-semibold text-secondary-900">Session Notes</h2>
        <button
          onClick={onNewSession}
          className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          + New Session
        </button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
            <tr>
              <th className="px-6 py-3 text-left">Patient</th>
              <th className="px-6 py-3 text-left">Profession</th>
              <th className="px-6 py-3 text-left">Clinician</th>
              <th className="px-6 py-3 text-left">Date</th>
              <th className="px-6 py-3 text-left">Session #</th>
              <th className="px-6 py-3 text-left">Duration</th>
              <th className="px-6 py-3 text-left">Charge Code</th>
              <th className="px-6 py-3 text-left">Status</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-secondary-100">
            {sessions.map(session => (
              <tr key={session.id} className="hover:bg-secondary-50">
                <td className="px-6 py-3">
                  <p className="font-medium text-secondary-900">{session.patientName}</p>
                  <p className="font-mono text-xs text-secondary-500">NHI: {session.patientNHI}</p>
                </td>
                <td className="px-6 py-3"><ProfessionBadge profession={session.profession} /></td>
                <td className="px-6 py-3 text-secondary-700">{session.clinician}</td>
                <td className="px-6 py-3 text-secondary-500">{session.sessionDate}</td>
                <td className="px-6 py-3 text-secondary-700">{session.sessionNumber}</td>
                <td className="px-6 py-3 text-secondary-700">{session.durationMinutes} min</td>
                <td className="px-6 py-3 font-mono text-xs text-secondary-600">{session.chargeCode}</td>
                <td className="px-6 py-3"><StatusBadge status={session.status} /></td>
              </tr>
            ))}
            {sessions.length === 0 && (
              <tr>
                <td colSpan={8} className="px-6 py-8 text-center text-sm text-secondary-400">No session notes found</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
