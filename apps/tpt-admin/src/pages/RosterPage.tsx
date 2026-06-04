import { useEffect, useState } from 'react';

interface Shift {
  id: string;
  principal_id: string;
  shift_start: string;
  shift_end: string;
  shift_type: 'ordinary' | 'on_call' | 'overtime';
  department_id?: string;
  notes?: string;
}

export function RosterPage() {
  const [shifts, setShifts] = useState<Shift[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/v1/practice/roster')
      .then(r => r.json())
      .then(data => setShifts(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <div className="p-6 text-gray-500">Loading roster…</div>;
  }

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Staff Roster</h1>
        <button className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700">
          Add shift
        </button>
      </div>

      {shifts.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center">
          <p className="text-gray-500">No shifts scheduled. Add the first shift to get started.</p>
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                {['Staff', 'Start', 'End', 'Type', 'Department', 'Payroll sync'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {shifts.map(s => (
                <tr key={s.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm text-gray-900">{s.principal_id}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{new Date(s.shift_start).toLocaleString('en-NZ')}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{new Date(s.shift_end).toLocaleString('en-NZ')}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                      s.shift_type === 'overtime' ? 'bg-orange-100 text-orange-700' :
                      s.shift_type === 'on_call' ? 'bg-blue-100 text-blue-700' :
                      'bg-green-100 text-green-700'
                    }`}>
                      {s.shift_type.replace('_', ' ')}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-sm text-gray-500">{s.department_id ?? '—'}</td>
                  <td className="px-4 py-3 text-sm text-gray-500">{s.notes ?? 'Pending'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
