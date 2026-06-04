import { useEffect, useState } from 'react';

interface Booking {
  id: string;
  room_id: string;
  booked_by: string;
  start_time: string;
  end_time: string;
  appointment_ref?: string;
}

export function RoomsPage() {
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/v1/practice/rooms/bookings')
      .then(r => r.json())
      .then(data => setBookings(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-6 text-gray-500">Loading room bookings…</div>;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Room Bookings</h1>
        <button className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700">
          Book room
        </button>
      </div>

      {bookings.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center text-gray-500">
          No room bookings found.
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                {['Room', 'Booked by', 'Start', 'End', 'Appointment'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {bookings.map(b => (
                <tr key={b.id} className="hover:bg-gray-50">
                  <td className="px-4 py-3 text-sm text-gray-900">{b.room_id}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{b.booked_by}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{new Date(b.start_time).toLocaleString('en-NZ')}</td>
                  <td className="px-4 py-3 text-sm text-gray-600">{new Date(b.end_time).toLocaleString('en-NZ')}</td>
                  <td className="px-4 py-3 text-sm text-gray-500">{b.appointment_ref ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
