import { useEffect, useState } from 'react';
import { formatNZD } from '../utils/format';

interface VarianceLine {
  month: number;
  category: string;
  planned_cents: number;
  actual_cents: number;
  variance_cents: number;
  variance_pct: number;
}

interface VarianceReport {
  cost_centre_name: string;
  financial_year: number;
  as_at_month: number;
  lines: VarianceLine[];
  total_planned_cents: number;
  total_actual_cents: number;
  total_variance_cents: number;
}

const MONTHS = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];

export function BudgetPage() {
  const [report, setReport] = useState<VarianceReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [year] = useState(new Date().getFullYear());
  const [month] = useState(new Date().getMonth() + 1);

  useEffect(() => {
    // Real: fetch from /api/v1/practice/cost-centres/{id}/variance/{year}?month={month}
    const mock: VarianceReport = {
      cost_centre_name: 'General Practice',
      financial_year: year,
      as_at_month: month,
      lines: [
        { month: 1, category: 'Staff', planned_cents: 5000000, actual_cents: 5120000, variance_cents: 120000, variance_pct: 2.4 },
        { month: 2, category: 'Staff', planned_cents: 5000000, actual_cents: 4980000, variance_cents: -20000, variance_pct: -0.4 },
        { month: 1, category: 'Supplies', planned_cents: 200000, actual_cents: 185000, variance_cents: -15000, variance_pct: -7.5 },
        { month: 2, category: 'Supplies', planned_cents: 200000, actual_cents: 210000, variance_cents: 10000, variance_pct: 5 },
      ],
      total_planned_cents: 10400000,
      total_actual_cents: 10495000,
      total_variance_cents: 95000,
    };
    setReport(mock);
  }, [year, month]);

  if (loading || !report) return <div className="p-6 text-gray-500">Loading budget…</div>;

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Budget Variance</h1>
          <p className="text-gray-500 text-sm mt-1">{report.cost_centre_name} — FY{report.financial_year} to {MONTHS[report.as_at_month - 1]}</p>
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        {[
          { label: 'Total planned', value: formatNZD(report.total_planned_cents), colour: 'text-gray-900' },
          { label: 'Total actual', value: formatNZD(report.total_actual_cents), colour: 'text-gray-900' },
          { label: 'Variance', value: formatNZD(Math.abs(report.total_variance_cents)), colour: report.total_variance_cents > 0 ? 'text-red-600' : 'text-green-600' },
        ].map(card => (
          <div key={card.label} className="bg-white rounded-xl border border-gray-200 p-4">
            <p className="text-xs text-gray-500 mb-1">{card.label}</p>
            <p className={`text-xl font-semibold ${card.colour}`}>
              {report.total_variance_cents > 0 && card.label === 'Variance' ? '+' : ''}{card.value}
            </p>
          </div>
        ))}
      </div>

      <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              {['Month', 'Category', 'Planned', 'Actual', 'Variance', '%'].map(h => (
                <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {report.lines.map((line, i) => (
              <tr key={i} className="hover:bg-gray-50">
                <td className="px-4 py-3 text-sm text-gray-600">{MONTHS[line.month - 1]}</td>
                <td className="px-4 py-3 text-sm text-gray-900">{line.category}</td>
                <td className="px-4 py-3 text-sm text-gray-600">{formatNZD(line.planned_cents)}</td>
                <td className="px-4 py-3 text-sm text-gray-900 font-medium">{formatNZD(line.actual_cents)}</td>
                <td className={`px-4 py-3 text-sm font-medium ${line.variance_cents > 0 ? 'text-red-600' : 'text-green-600'}`}>
                  {line.variance_cents > 0 ? '+' : ''}{formatNZD(line.variance_cents)}
                </td>
                <td className={`px-4 py-3 text-sm ${line.variance_pct > 0 ? 'text-red-500' : 'text-green-500'}`}>
                  {line.variance_pct > 0 ? '+' : ''}{line.variance_pct.toFixed(1)}%
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
