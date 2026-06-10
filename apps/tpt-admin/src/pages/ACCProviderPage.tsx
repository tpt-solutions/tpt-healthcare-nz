import { useState } from 'react';

interface ProviderStatus {
  providerNumber: string;
  status: 'active' | 'inactive' | 'pending' | 'unknown';
  registeredName?: string;
  disciplines?: string[];
  lastVerified?: string;
}

type VerifyState = 'idle' | 'verifying' | 'verified' | 'error';

export function ACCProviderPage() {
  const [providerNumber, setProviderNumber] = useState('');
  const [saved, setSaved] = useState('');
  const [verifyState, setVerifyState] = useState<VerifyState>('idle');
  const [providerStatus, setProviderStatus] = useState<ProviderStatus | null>(null);
  const [errorMessage, setErrorMessage] = useState('');
  const [saveSuccess, setSaveSuccess] = useState(false);

  const handleVerify = async () => {
    const trimmed = providerNumber.trim();
    if (!trimmed) {
      setErrorMessage('Enter an ACC provider number before verifying.');
      return;
    }
    setVerifyState('verifying');
    setErrorMessage('');
    setProviderStatus(null);

    try {
      // Stub: replace with real API call to practice server /api/v1/acc/provider/verify
      await new Promise<void>((resolve) => setTimeout(resolve, 1200));
      // Simulate a successful verification response
      setProviderStatus({
        providerNumber: trimmed,
        status: 'active',
        registeredName: 'Wellington Acupuncture Clinic Ltd',
        disciplines: ['Acupuncture', 'Massage Therapy'],
        lastVerified: new Date().toISOString(),
      });
      setVerifyState('verified');
    } catch {
      setErrorMessage('Unable to reach ACC to verify this provider number. Check your network and try again.');
      setVerifyState('error');
    }
  };

  const handleSave = async () => {
    const trimmed = providerNumber.trim();
    if (!trimmed) return;
    setSaveSuccess(false);
    try {
      // Stub: replace with real PATCH /api/v1/settings/acc-provider
      await new Promise<void>((resolve) => setTimeout(resolve, 600));
      setSaved(trimmed);
      setSaveSuccess(true);
    } catch {
      setErrorMessage('Failed to save provider number. Please try again.');
    }
  };

  const statusBadge = (status: ProviderStatus['status']) => {
    const map: Record<ProviderStatus['status'], { label: string; classes: string }> = {
      active:   { label: 'Active',   classes: 'bg-green-100 text-green-800' },
      inactive: { label: 'Inactive', classes: 'bg-red-100 text-red-800' },
      pending:  { label: 'Pending',  classes: 'bg-yellow-100 text-yellow-800' },
      unknown:  { label: 'Unknown',  classes: 'bg-gray-100 text-gray-600' },
    };
    const { label, classes } = map[status];
    return (
      <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${classes}`}>
        {label}
      </span>
    );
  };

  return (
    <div className="p-8 max-w-2xl">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">ACC Provider Registration</h1>
        <p className="mt-1 text-sm text-gray-500">
          Verify and store your practice's ACC Treatment Provider number. This number is required
          for lodging claims and requesting purchase orders.
        </p>
      </div>

      {/* Current stored number */}
      {saved && (
        <div className="mb-6 p-4 rounded-lg bg-blue-50 border border-blue-200">
          <p className="text-sm text-blue-800">
            <span className="font-medium">Stored provider number:</span>{' '}
            <span className="font-mono">{saved}</span>
          </p>
        </div>
      )}

      <div className="bg-white rounded-xl border border-gray-200 p-6 space-y-5">
        {/* Input */}
        <div>
          <label htmlFor="providerNumber" className="block text-sm font-medium text-gray-700 mb-1">
            ACC Provider Number
          </label>
          <div className="flex gap-3">
            <input
              id="providerNumber"
              type="text"
              value={providerNumber}
              onChange={(e) => {
                setProviderNumber(e.target.value);
                setVerifyState('idle');
                setProviderStatus(null);
                setErrorMessage('');
                setSaveSuccess(false);
              }}
              placeholder="e.g. P12345"
              className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-brand-500 focus:border-transparent"
            />
            <button
              type="button"
              onClick={handleVerify}
              disabled={verifyState === 'verifying' || !providerNumber.trim()}
              className="px-4 py-2 rounded-lg bg-brand-600 text-white text-sm font-medium hover:bg-brand-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {verifyState === 'verifying' ? 'Verifying…' : 'Verify with ACC'}
            </button>
          </div>
          <p className="mt-1 text-xs text-gray-400">
            Your ACC provider number is issued when you register as a Treatment Provider.
          </p>
        </div>

        {/* Error */}
        {errorMessage && (
          <div className="rounded-lg bg-red-50 border border-red-200 px-4 py-3">
            <p className="text-sm text-red-700">{errorMessage}</p>
          </div>
        )}

        {/* Verification result */}
        {providerStatus && verifyState === 'verified' && (
          <div className="rounded-lg bg-gray-50 border border-gray-200 p-4 space-y-3">
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium text-gray-700">Verification result</p>
              {statusBadge(providerStatus.status)}
            </div>
            <dl className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <dt className="text-gray-500">Provider number</dt>
                <dd className="font-mono font-medium text-gray-900">{providerStatus.providerNumber}</dd>
              </div>
              {providerStatus.registeredName && (
                <div>
                  <dt className="text-gray-500">Registered name</dt>
                  <dd className="font-medium text-gray-900">{providerStatus.registeredName}</dd>
                </div>
              )}
              {providerStatus.disciplines && providerStatus.disciplines.length > 0 && (
                <div className="col-span-2">
                  <dt className="text-gray-500 mb-1">Funded disciplines</dt>
                  <dd className="flex flex-wrap gap-2">
                    {providerStatus.disciplines.map((d) => (
                      <span key={d} className="px-2 py-0.5 rounded bg-brand-50 text-brand-700 text-xs font-medium">
                        {d}
                      </span>
                    ))}
                  </dd>
                </div>
              )}
              {providerStatus.lastVerified && (
                <div className="col-span-2">
                  <dt className="text-gray-500">Verified at</dt>
                  <dd className="text-gray-700">
                    {new Date(providerStatus.lastVerified).toLocaleString('en-NZ', {
                      dateStyle: 'medium',
                      timeStyle: 'short',
                    })}
                  </dd>
                </div>
              )}
            </dl>
          </div>
        )}

        {/* Save */}
        {verifyState === 'verified' && providerStatus?.status === 'active' && (
          <div className="flex items-center justify-between pt-2 border-t border-gray-100">
            {saveSuccess ? (
              <p className="text-sm text-green-700 font-medium">Provider number saved.</p>
            ) : (
              <p className="text-sm text-gray-500">Save to apply this number to all ACC claim submissions.</p>
            )}
            <button
              type="button"
              onClick={handleSave}
              className="px-4 py-2 rounded-lg bg-green-600 text-white text-sm font-medium hover:bg-green-700 transition-colors"
            >
              Save Provider Number
            </button>
          </div>
        )}
      </div>

      {/* Help */}
      <div className="mt-6 rounded-lg bg-amber-50 border border-amber-200 p-4">
        <h3 className="text-sm font-semibold text-amber-900 mb-1">Getting your ACC provider number</h3>
        <p className="text-sm text-amber-800">
          If you haven't registered as a Treatment Provider, contact ACC on{' '}
          <span className="font-medium">0800 222 070</span> or visit{' '}
          <span className="font-medium">providers.acc.co.nz</span> to complete your registration
          before you can lodge claims.
        </p>
      </div>
    </div>
  );
}
