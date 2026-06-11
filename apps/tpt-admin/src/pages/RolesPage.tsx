import { useEffect, useState } from 'react';

interface Department {
  id: string;
  name: string;
  code: string;
}

interface RoleAssignment {
  id: string;
  principal_id: string;
  role: string;
  department_id?: string;
  granted_by: string;
  created_at: string;
  revoked_at?: string;
}

const BUILT_IN_ROLES = [
  { value: 'practice_admin',    label: 'Practice Admin',     description: 'Full operational access; no clinical record content' },
  { value: 'clinician',         label: 'Clinician',          description: 'Clinical data + own departments' },
  { value: 'receptionist',      label: 'Receptionist',       description: 'Scheduling, billing, demographics; no clinical notes' },
  { value: 'nurse',             label: 'Nurse',              description: 'Clinical notes read, vitals write; department-scoped' },
  { value: 'pharmacist',        label: 'Pharmacist',         description: 'Pharmacy module only' },
  { value: 'billing_manager',   label: 'Billing Manager',    description: 'Billing + finance; no clinical content' },
  { value: 'inventory_manager', label: 'Inventory Manager',  description: 'Stock, purchase orders, cold-chain' },
  { value: 'roster_manager',    label: 'Roster Manager',     description: 'Shifts, rooms, leave approvals' },
];

const ROLE_LABELS: Record<string, string> = Object.fromEntries(
  BUILT_IN_ROLES.map(r => [r.value, r.label])
);

function roleBadgeClass(role: string): string {
  const map: Record<string, string> = {
    practice_admin:    'bg-purple-100 text-purple-700',
    clinician:         'bg-blue-100 text-blue-700',
    receptionist:      'bg-sky-100 text-sky-700',
    nurse:             'bg-teal-100 text-teal-700',
    pharmacist:        'bg-green-100 text-green-700',
    billing_manager:   'bg-amber-100 text-amber-700',
    inventory_manager: 'bg-orange-100 text-orange-700',
    roster_manager:    'bg-indigo-100 text-indigo-700',
  };
  return map[role] ?? 'bg-gray-100 text-gray-600';
}

const EMPTY_FORM = { principal_id: '', role: '', department_id: '' };

export function RolesPage() {
  const [assignments, setAssignments] = useState<RoleAssignment[]>([]);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState(EMPTY_FORM);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [revoking, setRevoking] = useState<string | null>(null);

  const loadAssignments = () =>
    fetch('/api/v1/practice/roles')
      .then(r => r.json())
      .then(data => setAssignments(Array.isArray(data) ? data : []));

  useEffect(() => {
    Promise.all([
      fetch('/api/v1/practice/roles').then(r => r.json()),
      fetch('/api/v1/practice/departments').then(r => r.json()),
    ]).then(([roles, depts]) => {
      setAssignments(Array.isArray(roles) ? roles : []);
      setDepartments(Array.isArray(depts) ? depts : []);
    }).finally(() => setLoading(false));
  }, []);

  const grant = async () => {
    if (!form.principal_id.trim() || !form.role) {
      setError('User ID and role are required.');
      return;
    }
    setSaving(true);
    setError('');
    try {
      const res = await fetch('/api/v1/practice/roles', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          principal_id:  form.principal_id.trim(),
          role:          form.role,
          department_id: form.department_id || undefined,
        }),
      });
      if (!res.ok) {
        const text = await res.text();
        setError(text || 'Failed to grant role.');
        return;
      }
      setShowForm(false);
      setForm(EMPTY_FORM);
      await loadAssignments().then(d => setAssignments(Array.isArray(d) ? d : []));
    } finally {
      setSaving(false);
    }
  };

  const revoke = async (id: string) => {
    setRevoking(id);
    try {
      await fetch(`/api/v1/practice/roles/${id}`, { method: 'DELETE' });
      setAssignments(prev => prev.filter(a => a.id !== id));
    } finally {
      setRevoking(null);
    }
  };

  const deptName = (id?: string) => {
    if (!id) return null;
    return departments.find(d => d.id === id)?.name ?? id;
  };

  if (loading) return <div className="p-6 text-gray-500">Loading roles…</div>;

  return (
    <div className="p-6 max-w-4xl">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Role Assignments</h1>
          <p className="mt-1 text-sm text-gray-500">
            Assign built-in roles to staff, optionally scoped to a department.
            A tenant-wide assignment (no department) applies across all departments.
          </p>
        </div>
        <button
          onClick={() => { setShowForm(true); setError(''); }}
          className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 flex-shrink-0"
        >
          Grant role
        </button>
      </div>

      {/* Grant form */}
      {showForm && (
        <div className="bg-white rounded-xl border border-gray-200 p-5 mb-6">
          <h2 className="text-sm font-semibold text-gray-800 mb-4">New role assignment</h2>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3 mb-4">
            {/* User ID */}
            <div className="sm:col-span-1">
              <label className="block text-xs font-medium text-gray-600 mb-1">User ID (JWT sub)</label>
              <input
                value={form.principal_id}
                onChange={e => setForm(f => ({ ...f, principal_id: e.target.value }))}
                placeholder="auth0|abc123 or hpi:99-ZZZ-99"
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
              />
            </div>

            {/* Role */}
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">Role</label>
              <select
                value={form.role}
                onChange={e => setForm(f => ({ ...f, role: e.target.value }))}
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 bg-white"
              >
                <option value="">Select a role…</option>
                {BUILT_IN_ROLES.map(r => (
                  <option key={r.value} value={r.value}>{r.label}</option>
                ))}
              </select>
              {form.role && (
                <p className="mt-1 text-xs text-gray-400">
                  {BUILT_IN_ROLES.find(r => r.value === form.role)?.description}
                </p>
              )}
            </div>

            {/* Department (optional) */}
            <div>
              <label className="block text-xs font-medium text-gray-600 mb-1">
                Department <span className="font-normal text-gray-400">(optional — leave blank for tenant-wide)</span>
              </label>
              <select
                value={form.department_id}
                onChange={e => setForm(f => ({ ...f, department_id: e.target.value }))}
                className="w-full border border-gray-300 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 bg-white"
              >
                <option value="">All departments (tenant-wide)</option>
                {departments.map(d => (
                  <option key={d.id} value={d.id}>{d.name} ({d.code})</option>
                ))}
              </select>
            </div>
          </div>

          {error && (
            <p className="mb-3 text-sm text-red-600 bg-red-50 rounded px-3 py-2">{error}</p>
          )}

          <div className="flex gap-2">
            <button
              onClick={grant}
              disabled={saving}
              className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700 disabled:opacity-50"
            >
              {saving ? 'Saving…' : 'Grant role'}
            </button>
            <button
              onClick={() => { setShowForm(false); setForm(EMPTY_FORM); setError(''); }}
              className="px-4 py-2 text-gray-600 rounded-lg text-sm hover:bg-gray-100"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Assignments table */}
      {assignments.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center text-gray-500">
          No role assignments yet. Grant a role to get started.
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                {['User', 'Role', 'Scope', 'Granted', ''].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wide">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {assignments.map(a => {
                const dept = deptName(a.department_id);
                return (
                  <tr key={a.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3">
                      <p className="text-sm font-mono text-gray-800 truncate max-w-[180px]" title={a.principal_id}>
                        {a.principal_id}
                      </p>
                      <p className="text-xs text-gray-400 truncate max-w-[180px]" title={`Granted by ${a.granted_by}`}>
                        by {a.granted_by}
                      </p>
                    </td>
                    <td className="px-4 py-3">
                      <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${roleBadgeClass(a.role)}`}>
                        {ROLE_LABELS[a.role] ?? a.role}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-600">
                      {dept ? (
                        <span className="inline-flex items-center gap-1">
                          <svg className="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 21h16.5M4.5 3h15M5.25 3v18m13.5-18v18M9 6.75h1.5m-1.5 3h1.5m-1.5 3h1.5m3-6H15m-1.5 3H15m-1.5 3H15M9 21v-3.375c0-.621.504-1.125 1.125-1.125h3.75c.621 0 1.125.504 1.125 1.125V21" />
                          </svg>
                          {dept}
                        </span>
                      ) : (
                        <span className="text-gray-400 italic">Tenant-wide</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs text-gray-400">
                      {new Date(a.created_at).toLocaleDateString('en-NZ')}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => revoke(a.id)}
                        disabled={revoking === a.id}
                        className="text-xs text-red-600 hover:underline disabled:opacity-40"
                      >
                        {revoking === a.id ? 'Revoking…' : 'Revoke'}
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Role legend */}
      <div className="mt-8">
        <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">Role reference</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          {BUILT_IN_ROLES.map(r => (
            <div key={r.value} className="flex items-start gap-2 p-2 rounded-lg bg-gray-50">
              <span className={`mt-0.5 inline-flex rounded-full px-2 py-0.5 text-xs font-medium flex-shrink-0 ${roleBadgeClass(r.value)}`}>
                {r.label}
              </span>
              <p className="text-xs text-gray-500">{r.description}</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
