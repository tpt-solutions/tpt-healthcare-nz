import { useState } from 'react';
import { useNavigate } from 'react-router-dom';

const STEPS = [
  { label: 'Practice details', desc: 'Name, HPI ID, address and contact information.' },
  { label: 'Departments', desc: 'Configure departments (GP, Pharmacy, Lab, Admin, Nursing).' },
  { label: 'Staff & roles', desc: 'Invite initial staff and assign roles and departments.' },
  { label: 'Accounting', desc: 'Connect Xero, QuickBooks Online, or FreshBooks.' },
  { label: 'Payroll', desc: 'Connect PayHero, iPayroll, FlexiTime, or Datacom.' },
  { label: 'Inventory', desc: 'Set up stock categories and cold-chain settings.' },
  { label: 'Review & launch', desc: 'Confirm all integrations and go live.' },
];

export function OnboardingWizard() {
  const [step, setStep] = useState(1);
  const [saving, setSaving] = useState(false);
  const navigate = useNavigate();

  const advance = async () => {
    setSaving(true);
    try {
      await fetch(`/api/v1/practice/onboarding/step/${step}`, { method: 'PUT' });
      if (step === STEPS.length) {
        navigate('/dashboard');
        return;
      }
      setStep(s => s + 1);
    } finally {
      setSaving(false);
    }
  };

  const current = STEPS[step - 1];

  return (
    <div className="min-h-screen bg-gray-50 flex items-center justify-center p-6">
      <div className="bg-white rounded-2xl shadow-lg w-full max-w-2xl p-8">
        <h1 className="text-2xl font-bold text-gray-900 mb-2">Welcome to tpt-healthcare</h1>
        <p className="text-gray-500 mb-8">Let's get your practice set up — this takes about 10 minutes.</p>

        {/* Step indicator */}
        <div className="flex items-center gap-1 mb-8">
          {STEPS.map((_, i) => (
            <div
              key={i}
              className={`h-2 flex-1 rounded-full transition-colors ${
                i + 1 < step ? 'bg-indigo-500' : i + 1 === step ? 'bg-indigo-400' : 'bg-gray-200'
              }`}
            />
          ))}
        </div>

        {/* Current step */}
        <div className="mb-8">
          <p className="text-xs font-medium text-indigo-600 uppercase tracking-wide mb-1">
            Step {step} of {STEPS.length}
          </p>
          <h2 className="text-xl font-semibold text-gray-900 mb-2">{current.label}</h2>
          <p className="text-gray-600">{current.desc}</p>
        </div>

        {/* Step-specific content placeholder */}
        <div className="bg-gray-50 rounded-xl p-6 mb-8 min-h-32 flex items-center justify-center text-gray-400 text-sm">
          {current.label} configuration form
        </div>

        {/* Actions */}
        <div className="flex justify-between">
          <button
            onClick={() => setStep(s => Math.max(1, s - 1))}
            disabled={step === 1}
            className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900 disabled:opacity-40"
          >
            Back
          </button>
          <button
            onClick={advance}
            disabled={saving}
            className="px-6 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
          >
            {saving ? 'Saving…' : step === STEPS.length ? 'Launch' : 'Continue'}
          </button>
        </div>
      </div>
    </div>
  );
}
