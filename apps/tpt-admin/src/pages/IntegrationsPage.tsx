import { useEffect, useState } from 'react';

interface ProviderStatus {
  provider_type: string;
  provider_name: string;
  ok: boolean;
  last_checked_at: string;
  latency_ms: number;
  organisation_name?: string;
  error_text?: string;
}

const TYPE_LABELS: Record<string, string> = {
  accounting:  'Accounting',
  payroll:     'Payroll',
  sms:         'SMS',
  email:       'Email',
  storage:     'Storage',
  payment:     'Payment',
  fax:         'Fax / Secure messaging',
  video:       'Video / Telehealth',
};

export function IntegrationsPage() {
  const [statuses, setStatuses] = useState<ProviderStatus[]>([]);
  const [loading, setLoading] = useState(true);

  const load = () => {
    fetch('/api/v1/health/providers')
      .then(r => r.json())
      .then(data => {
        if (Array.isArray(data?.providers)) setStatuses(data.providers);
        else if (Array.isArray(data)) setStatuses(data);
      })
      .catch(() => setStatuses([]))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const triggerSync = async (type: string) => {
    if (type === 'accounting') await fetch('/api/v1/practice/accounting/sync', { method: 'POST' });
    if (type === 'payroll') await fetch('/api/v1/practice/payroll/sync', { method: 'POST' });
  };

  if (loading) return <div className="p-6 text-gray-500">Loading integrations…</div>;

  const grouped = Object.groupBy(statuses, s => s.provider_type) as Record<string, ProviderStatus[]>;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Integrations</h1>
        <button onClick={load} className="px-4 py-2 bg-white border border-gray-300 text-gray-700 rounded-lg text-sm hover:bg-gray-50">
          Refresh status
        </button>
      </div>

      {statuses.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center text-gray-500">
          No providers configured. Complete the onboarding wizard to connect your first integration.
        </div>
      ) : (
        <div className="space-y-6">
          {Object.entries(TYPE_LABELS).map(([type, label]) => {
            const group = grouped[type] ?? [];
            if (group.length === 0) return null;
            return (
              <div key={type} className="bg-white rounded-xl border border-gray-200 overflow-hidden">
                <div className="px-4 py-3 bg-gray-50 border-b border-gray-200 flex items-center justify-between">
                  <h2 className="text-sm font-semibold text-gray-700">{label}</h2>
                  {(type === 'accounting' || type === 'payroll') && (
                    <button onClick={() => triggerSync(type)}
                      className="text-xs text-indigo-600 hover:underline">
                      Trigger sync
                    </button>
                  )}
                </div>
                <div className="divide-y divide-gray-100">
                  {group.map(s => (
                    <div key={s.provider_name} className="px-4 py-3 flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-gray-900 capitalize">{s.provider_name}</p>
                        {s.organisation_name && (
                          <p className="text-xs text-gray-500">{s.organisation_name}</p>
                        )}
                        {!s.ok && s.error_text && (
                          <p className="text-xs text-red-500 mt-0.5">{s.error_text}</p>
                        )}
                      </div>
                      <div className="flex items-center gap-3">
                        <span className="text-xs text-gray-400">{s.latency_ms}ms</span>
                        <span className={`inline-flex items-center gap-1 text-xs font-medium ${s.ok ? 'text-green-700' : 'text-red-600'}`}>
                          <span className={`h-2 w-2 rounded-full ${s.ok ? 'bg-green-500' : 'bg-red-500'}`} />
                          {s.ok ? 'Connected' : 'Error'}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
