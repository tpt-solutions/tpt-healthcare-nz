import { useState } from 'react';

interface PracticeSettings {
  practiceName: string;
  hpiFacilityId: string;
  phoName: string;
  dhbName: string;
  address: string;
  phone: string;
  email: string;
  nzbn: string;
  ediAddress: string;
  defaultAppointmentDuration: number;
  cancellationWindowHours: number;
  telehealthEnabled: boolean;
  accClaimingEnabled: boolean;
  hipcConsentRequired: boolean;
}

const defaultSettings: PracticeSettings = {
  practiceName: 'Auckland City Medical Centre',
  hpiFacilityId: 'F0K068-C',
  phoName: 'Waitematā PHO',
  dhbName: 'Te Whatu Ora — Waitematā',
  address: 'Level 2, 123 Queen Street, Auckland CBD 1010',
  phone: '09 309 0000',
  email: 'admin@aucklandcitymedical.nz',
  nzbn: '9429041234567',
  ediAddress: 'ACMC001',
  defaultAppointmentDuration: 15,
  cancellationWindowHours: 24,
  telehealthEnabled: true,
  accClaimingEnabled: true,
  hipcConsentRequired: true,
};

export function SettingsPage() {
  const [settings, setSettings] = useState(defaultSettings);
  const [saved, setSaved] = useState(false);

  const handleSave = () => {
    // TODO: PUT /api/v1/admin/settings
    setSaved(true);
    setTimeout(() => setSaved(false), 3000);
  };

  const update = <K extends keyof PracticeSettings>(key: K, value: PracticeSettings[K]) => {
    setSettings(prev => ({ ...prev, [key]: value }));
    setSaved(false);
  };

  return (
    <div className="p-6 max-w-3xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Practice Settings</h1>
          <p className="mt-1 text-sm text-gray-500">Configure practice identity, integrations, and defaults.</p>
        </div>
        <button
          onClick={handleSave}
          className="rounded-lg bg-brand-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-brand-700 transition-colors"
        >
          {saved ? 'Saved' : 'Save Changes'}
        </button>
      </div>

      {saved && (
        <div className="bg-green-50 border border-green-200 rounded-xl px-4 py-3 mb-5">
          <p className="text-sm text-green-700">Settings saved successfully.</p>
        </div>
      )}

      {/* Practice identity */}
      <section className="bg-white rounded-xl border border-gray-200 shadow-sm p-5 mb-5">
        <h2 className="text-sm font-semibold text-gray-900 mb-4">Practice Identity</h2>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">Practice Name</label>
            <input type="text" value={settings.practiceName} onChange={e => update('practiceName', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">HPI Facility ID</label>
            <input type="text" value={settings.hpiFacilityId} onChange={e => update('hpiFacilityId', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none font-mono" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">PHO Name</label>
            <input type="text" value={settings.phoName} onChange={e => update('phoName', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">Te Whatu Ora / DHB</label>
            <input type="text" value={settings.dhbName} onChange={e => update('dhbName', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
          <div className="col-span-2">
            <label className="block text-xs font-medium text-gray-700 mb-1">Address</label>
            <input type="text" value={settings.address} onChange={e => update('address', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">Phone</label>
            <input type="tel" value={settings.phone} onChange={e => update('phone', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">Admin Email</label>
            <input type="email" value={settings.email} onChange={e => update('email', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">NZBN</label>
            <input type="text" value={settings.nzbn} onChange={e => update('nzbn', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">EDI Address</label>
            <input type="text" value={settings.ediAddress} onChange={e => update('ediAddress', e.target.value)}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm font-mono focus:border-brand-500 focus:outline-none" />
          </div>
        </div>
      </section>

      {/* Scheduling */}
      <section className="bg-white rounded-xl border border-gray-200 shadow-sm p-5 mb-5">
        <h2 className="text-sm font-semibold text-gray-900 mb-4">Scheduling</h2>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">Default Appointment Duration (minutes)</label>
            <input type="number" min={5} max={120} step={5} value={settings.defaultAppointmentDuration}
              onChange={e => update('defaultAppointmentDuration', Number(e.target.value))}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
          <div>
            <label className="block text-xs font-medium text-gray-700 mb-1">Patient Cancellation Window (hours)</label>
            <input type="number" min={1} max={72} value={settings.cancellationWindowHours}
              onChange={e => update('cancellationWindowHours', Number(e.target.value))}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-brand-500 focus:outline-none" />
          </div>
        </div>
      </section>

      {/* Feature flags */}
      <section className="bg-white rounded-xl border border-gray-200 shadow-sm p-5">
        <h2 className="text-sm font-semibold text-gray-900 mb-4">Feature Flags</h2>
        <div className="space-y-4">
          {[
            { key: 'telehealthEnabled' as const, label: 'Telehealth appointments', desc: 'Enable phone and video appointment types in the patient portal.' },
            { key: 'accClaimingEnabled' as const, label: 'ACC claiming', desc: 'Enable electronic ACC claim lodgement via core/acc/ FHIR client.' },
            { key: 'hipcConsentRequired' as const, label: 'HIPC consent gate', desc: 'Require explicit patient consent before disclosing records to third parties (HIPC Rule 11). Disabling this is not recommended.' },
          ].map(({ key, label, desc }) => (
            <label key={key} className="flex items-start gap-3 cursor-pointer">
              <div className="relative mt-0.5">
                <input
                  type="checkbox"
                  checked={settings[key] as boolean}
                  onChange={e => update(key, e.target.checked)}
                  className="sr-only"
                />
                <div
                  onClick={() => update(key, !settings[key])}
                  className={`h-5 w-9 rounded-full transition-colors ${settings[key] ? 'bg-brand-600' : 'bg-gray-200'}`}
                >
                  <div className={`h-4 w-4 rounded-full bg-white shadow-sm transition-transform mt-0.5 ml-0.5 ${settings[key] ? 'translate-x-4' : 'translate-x-0'}`} />
                </div>
              </div>
              <div>
                <p className="text-sm font-medium text-gray-900">{label}</p>
                <p className="text-xs text-gray-500">{desc}</p>
              </div>
            </label>
          ))}
        </div>
      </section>
    </div>
  );
}
