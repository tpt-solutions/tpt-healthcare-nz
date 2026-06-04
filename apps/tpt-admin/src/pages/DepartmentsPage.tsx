import { useEffect, useState } from 'react';

interface Department {
  id: string;
  name: string;
  code: string;
  parent_id?: string;
}

export function DepartmentsPage() {
  const [departments, setDepartments] = useState<Department[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: '', code: '' });

  const load = () => {
    fetch('/api/v1/practice/departments')
      .then(r => r.json())
      .then(data => setDepartments(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const save = async () => {
    await fetch('/api/v1/practice/departments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form),
    });
    setShowForm(false);
    setForm({ name: '', code: '' });
    load();
  };

  const remove = async (id: string) => {
    await fetch(`/api/v1/practice/departments/${id}`, { method: 'DELETE' });
    load();
  };

  if (loading) return <div className="p-6 text-gray-500">Loading departments…</div>;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Departments</h1>
        <button onClick={() => setShowForm(true)}
          className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700">
          Add department
        </button>
      </div>

      {showForm && (
        <div className="bg-white rounded-xl border border-gray-200 p-4 mb-6">
          <h2 className="text-sm font-semibold text-gray-700 mb-3">New department</h2>
          <div className="grid grid-cols-2 gap-3 mb-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Name</label>
              <input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm" placeholder="General Practice" />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Code</label>
              <input value={form.code} onChange={e => setForm(f => ({ ...f, code: e.target.value }))}
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm" placeholder="gp" />
            </div>
          </div>
          <div className="flex gap-2">
            <button onClick={save} className="px-4 py-2 bg-indigo-600 text-white rounded text-sm">Save</button>
            <button onClick={() => setShowForm(false)} className="px-4 py-2 text-gray-600 text-sm">Cancel</button>
          </div>
        </div>
      )}

      {departments.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center text-gray-500">
          No departments configured. Add your first department.
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                {['Name', 'Code', 'Parent', ''].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {departments.map(d => (
                <tr key={d.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm font-medium text-gray-900">{d.name}</td>
                  <td className="px-4 py-3 text-sm font-mono text-gray-600">{d.code}</td>
                  <td className="px-4 py-3 text-sm text-gray-500">{d.parent_id ?? '—'}</td>
                  <td className="px-4 py-3 text-right">
                    <button onClick={() => remove(d.id)} className="text-xs text-red-600 hover:underline">Remove</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
