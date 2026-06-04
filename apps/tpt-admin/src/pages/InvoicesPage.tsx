import { useEffect, useState } from 'react';
import { formatNZD } from '../utils/format';

interface Invoice {
  id: string;
  patient_nhi: string;
  funding_type: string;
  total_cents: number;
  patient_amount_cents: number;
  status: 'draft' | 'issued' | 'overdue' | 'paid' | 'cancelled';
  issued_at?: string;
  due_date?: string;
  paid_at?: string;
}

const STATUS_COLOURS: Record<string, string> = {
  draft:     'bg-gray-100 text-gray-500',
  issued:    'bg-blue-100 text-blue-700',
  overdue:   'bg-red-100 text-red-700',
  paid:      'bg-green-100 text-green-700',
  cancelled: 'bg-gray-100 text-gray-400',
};

export function InvoicesPage() {
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [filter, setFilter] = useState<string>('all');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Real: fetch from /api/v1/practice/invoices
    const mock: Invoice[] = [
      { id: '1', patient_nhi: 'ZZZ0001', funding_type: 'pho', total_cents: 3500, patient_amount_cents: 1000, status: 'issued', issued_at: '2026-06-01', due_date: '2026-07-01' },
      { id: '2', patient_nhi: 'ZZZ0002', funding_type: 'acc', total_cents: 12000, patient_amount_cents: 0, status: 'paid', issued_at: '2026-05-15', paid_at: '2026-05-20' },
      { id: '3', patient_nhi: 'ZZZ0003', funding_type: 'private', total_cents: 25000, patient_amount_cents: 25000, status: 'overdue', issued_at: '2026-04-01', due_date: '2026-05-01' },
    ];
    setInvoices(mock);
    setLoading(false);
  }, []);

  const filtered = filter === 'all' ? invoices : invoices.filter(i => i.status === filter);

  if (loading) return <div className="p-6 text-gray-500">Loading invoices…</div>;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Patient Invoices</h1>
        <button className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700">
          New invoice
        </button>
      </div>

      {/* Filter tabs */}
      <div className="flex gap-2 mb-4">
        {['all', 'draft', 'issued', 'overdue', 'paid'].map(s => (
          <button
            key={s}
            onClick={() => setFilter(s)}
            className={`px-3 py-1.5 rounded-full text-xs font-medium capitalize ${
              filter === s ? 'bg-indigo-600 text-white' : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
            }`}
          >
            {s}
          </button>
        ))}
      </div>

      <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              {['Patient NHI', 'Funding', 'Total', 'Patient owes', 'Status', 'Due date'].map(h => (
                <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {filtered.map(inv => (
              <tr key={inv.id} className="hover:bg-gray-50 cursor-pointer">
                <td className="px-4 py-3 text-sm font-mono text-gray-900">{inv.patient_nhi}</td>
                <td className="px-4 py-3 text-sm text-gray-600 uppercase">{inv.funding_type}</td>
                <td className="px-4 py-3 text-sm text-gray-900 font-medium">{formatNZD(inv.total_cents)}</td>
                <td className="px-4 py-3 text-sm text-gray-900">{formatNZD(inv.patient_amount_cents)}</td>
                <td className="px-4 py-3">
                  <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${STATUS_COLOURS[inv.status]}`}>
                    {inv.status}
                  </span>
                </td>
                <td className="px-4 py-3 text-sm text-gray-500">
                  {inv.status === 'paid' ? `Paid ${inv.paid_at}` : (inv.due_date ?? '—')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
