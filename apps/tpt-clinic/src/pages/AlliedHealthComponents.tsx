import { useEffect } from 'react';
import {
  Tab,
  ToastSeverity,
  professionClasses,
  professionLabels,
  professionProgressColor,
  statusClasses,
  mockTreatmentPlans,
  mockACCClaims,
  mockSessionNotes,
} from './alliedHealthTypes';

export function ProfessionBadge({ profession }: { profession: string }) {
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${professionClasses[profession] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {professionLabels[profession] ?? profession}
    </span>
  );
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${statusClasses[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}

export function ProgressBar({ used, approved, profession }: { used: number; approved: number; profession: string }) {
  const pct = approved > 0 ? Math.min((used / approved) * 100, 100) : 0;
  const barColor = used >= approved ? 'bg-red-500' : (professionProgressColor[profession] ?? 'bg-primary-500');
  return (
    <div>
      <span className="text-sm">{used} / {approved}</span>
      <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-secondary-200">
        <div className={`h-full rounded-full ${barColor}`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

export function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: { id: Tab; label: string; count: number }[] = [
    { id: 'plans', label: 'Treatment Plans', count: mockTreatmentPlans.length },
    { id: 'claims', label: 'ACC Claims', count: mockACCClaims.length },
    { id: 'sessions', label: 'Session Notes', count: mockSessionNotes.length },
    { id: 'dashboard', label: 'Dashboard', count: 0 },
  ];
  return (
    <div className="mb-6 flex gap-1 overflow-x-auto rounded-lg bg-secondary-100 p-1">
      {tabs.map(t => (
        <button
          key={t.id}
          onClick={() => onSelect(t.id)}
          className={`flex-shrink-0 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === t.id
              ? 'bg-white text-primary-700 shadow-sm'
              : 'text-secondary-600 hover:text-secondary-900'
          }`}
        >
          {t.label}{t.count > 0 ? ` (${t.count})` : ''}
        </button>
      ))}
    </div>
  );
}

const toastBg: Record<ToastSeverity, string> = {
  success: 'bg-green-600',
  error: 'bg-red-600',
  info: 'bg-blue-600',
  warning: 'bg-amber-500',
};

export function Toast({ message, severity, onClose }: { message: string; severity: ToastSeverity; onClose: () => void }) {
  useEffect(() => {
    const t = setTimeout(onClose, 5000);
    return () => clearTimeout(t);
  }, [message, onClose]);
  return (
    <div className={`fixed bottom-4 right-4 z-50 flex items-center gap-3 rounded-lg px-4 py-3 text-sm text-white shadow-lg ${toastBg[severity]}`}>
      <span>{message}</span>
      <button onClick={onClose} className="ml-2 font-bold opacity-75 hover:opacity-100">×</button>
    </div>
  );
}
