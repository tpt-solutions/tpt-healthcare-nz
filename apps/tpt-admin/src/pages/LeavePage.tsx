import { useEffect, useState } from 'react';

interface LeaveRequest {
  id: string;
  principal_id: string;
  leave_type: string;
  start_date: string;
  end_date: string;
  status: 'pending' | 'approved' | 'declined' | 'cancelled';
  notes?: string;
}

const STATUS_COLOURS: Record<string, string> = {
  pending:   'bg-yellow-100 text-yellow-700',
  approved:  'bg-green-100 text-green-700',
  declined:  'bg-red-100 text-red-700',
  cancelled: 'bg-gray-100 text-gray-500',
};

export function LeavePage() {
  const [requests, setRequests] = useState<LeaveRequest[]>([]);
  const [loading, setLoading] = useState(true);

  const load = () => {
    fetch('/api/v1/practice/leave')
      .then(r => r.json())
      .then(data => setRequests(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false));
  };

  useEffect(load, []);

  const action = async (id: string, verb: 'approve' | 'decline') => {
    await fetch(`/api/v1/practice/leave/${id}/${verb}`, { method: 'POST' });
    load();
  };

  if (loading) return <div className="p-6 text-gray-500">Loading leave requests…</div>;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Leave Requests</h1>
        <button className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700">
          Request leave
        </button>
      </div>

      {requests.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center text-gray-500">No leave requests.</div>
      ) : (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                {['Staff', 'Type', 'Start', 'End', 'Status', 'Actions'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {requests.map(r => (
                <tr key={r.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm text-gray-900">{r.principal_id}</td>
                  <td className="px-4 py-3 text-sm text-gray-600 capitalize">{r.leave_type}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{r.start_date}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{r.end_date}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_COLOURS[r.status]}`}>
                      {r.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 flex gap-2">
                    {r.status === 'pending' && (
                      <>
                        <button onClick={() => action(r.id, 'approve')}
                          className="text-xs text-green-600 hover:underline">Approve</button>
                        <button onClick={() => action(r.id, 'decline')}
                          className="text-xs text-red-600 hover:underline">Decline</button>
                      </>
                    )}
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
