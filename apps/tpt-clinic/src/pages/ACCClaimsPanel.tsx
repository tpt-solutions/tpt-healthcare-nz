import { ACCClaim } from './alliedHealthTypes';
import { ProfessionBadge, StatusBadge, ProgressBar } from './AlliedHealthComponents';

interface Props {
  claims: ACCClaim[];
  onNewClaim: () => void;
}

export default function ACCClaimsPanel({ claims, onNewClaim }: Props) {
  return (
    <div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
        <h2 className="text-base font-semibold text-secondary-900">ACC Claims</h2>
        <button
          onClick={onNewClaim}
          className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          + New Claim
        </button>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
            <tr>
              <th className="px-6 py-3 text-left">Patient</th>
              <th className="px-6 py-3 text-left">Claim Type</th>
              <th className="px-6 py-3 text-left">ACC Number</th>
              <th className="px-6 py-3 text-left">Diagnosis</th>
              <th className="px-6 py-3 text-left">Body Region</th>
              <th className="px-6 py-3 text-left">Status</th>
              <th className="px-6 py-3 text-left">Sessions</th>
              <th className="px-6 py-3 text-left">Expiry</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-secondary-100">
            {claims.map(claim => (
              <tr key={claim.id} className="hover:bg-secondary-50">
                <td className="px-6 py-3">
                  <p className="font-medium text-secondary-900">{claim.patientName}</p>
                  <p className="font-mono text-xs text-secondary-500">NHI: {claim.patientNHI}</p>
                </td>
                <td className="px-6 py-3"><ProfessionBadge profession={claim.claimType} /></td>
                <td className="px-6 py-3 font-mono text-xs font-medium text-secondary-700">{claim.accNumber}</td>
                <td className="max-w-[200px] truncate px-6 py-3 text-secondary-700">{claim.diagnosis}</td>
                <td className="px-6 py-3 text-secondary-600">{claim.bodyRegion.replace(/_/g, ' ')}</td>
                <td className="px-6 py-3"><StatusBadge status={claim.status} /></td>
                <td className="px-6 py-3">
                  <ProgressBar used={claim.usedSessions} approved={claim.approvedSessions} profession={claim.claimType} />
                </td>
                <td className="px-6 py-3 text-secondary-500">{claim.expiryDate}</td>
              </tr>
            ))}
            {claims.length === 0 && (
              <tr>
                <td colSpan={8} className="px-6 py-8 text-center text-sm text-secondary-400">No ACC claims found</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
