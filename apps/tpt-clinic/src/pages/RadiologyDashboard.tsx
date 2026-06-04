import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface ImagingStudySummary {
  id: string;
  patientNhi: string;
  modality: string;
  bodyPart: string;
  studyDate: string | null;
  description: string;
  status: string;
  numSeries: number;
  numInstances: number;
}

interface RadiologyOrderSummary {
  id: string;
  patientNhi: string;
  modality: string;
  priority: string;
  status: string;
  requestedAt: string;
  scheduledAt: string | null;
}

interface RadiologyReportSummary {
  id: string;
  patientNhi: string;
  radiologistHpi: string;
  status: string;
  createdAt: string;
  signedAt: string | null;
}

interface StudyListResponse { studies: ImagingStudySummary[]; total: number }
interface OrderListResponse { orders: RadiologyOrderSummary[]; total: number }
interface ReportListResponse { reports: RadiologyReportSummary[]; total: number }

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StatCard({
  label,
  value,
  color,
  linkTo,
}: {
  label: string;
  value: string | number;
  color: string;
  linkTo: string;
}) {
  return (
    <Link
      to={linkTo}
      className={`rounded-xl border ${color} bg-white p-5 shadow-sm transition-shadow hover:shadow-md`}
    >
      <p className="text-sm font-medium text-secondary-500">{label}</p>
      <p className="mt-2 text-3xl font-bold text-secondary-900">{value}</p>
    </Link>
  );
}

function ModalityBadge({ modality }: { modality: string }) {
  const colours: Record<string, string> = {
    CT: 'bg-blue-100 text-blue-700',
    MR: 'bg-purple-100 text-purple-700',
    XR: 'bg-green-100 text-green-700',
    US: 'bg-teal-100 text-teal-700',
    NM: 'bg-orange-100 text-orange-700',
    PT: 'bg-yellow-100 text-yellow-700',
    CR: 'bg-slate-100 text-slate-700',
  };
  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${colours[modality] ?? 'bg-secondary-100 text-secondary-600'}`}
    >
      {modality}
    </span>
  );
}

function PriorityBadge({ priority }: { priority: string }) {
  const colours: Record<string, string> = {
    stat: 'bg-red-100 text-red-700',
    urgent: 'bg-amber-100 text-amber-700',
    routine: 'bg-secondary-100 text-secondary-600',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colours[priority] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {priority}
    </span>
  );
}

function RecentStudiesTable({ studies }: { studies: ImagingStudySummary[] }) {
  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <div className="flex items-center justify-between border-b border-secondary-100 px-4 py-3">
        <h2 className="text-sm font-semibold text-secondary-900">Recent Imaging Studies</h2>
        <Link to="/radiology/studies" className="text-xs text-primary-600 hover:underline">
          View all
        </Link>
      </div>
      {studies.length === 0 ? (
        <p className="px-4 py-6 text-center text-sm text-secondary-500">No recent studies</p>
      ) : (
        <table className="w-full text-sm">
          <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
            <tr>
              <th className="px-4 py-2 text-left">Patient NHI</th>
              <th className="px-4 py-2 text-left">Modality</th>
              <th className="px-4 py-2 text-left">Description</th>
              <th className="px-4 py-2 text-left">Status</th>
              <th className="px-4 py-2 text-left">Date</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-secondary-100">
            {studies.map((s) => (
              <tr key={s.id} className="hover:bg-secondary-50">
                <td className="px-4 py-2 font-mono text-xs font-medium text-secondary-900">{s.patientNhi}</td>
                <td className="px-4 py-2">
                  <ModalityBadge modality={s.modality} />
                </td>
                <td className="px-4 py-2 max-w-[200px] truncate text-secondary-600">{s.description || s.bodyPart || '—'}</td>
                <td className="px-4 py-2">
                  <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                    s.status === 'available' ? 'bg-green-100 text-green-700' : 'bg-secondary-100 text-secondary-600'
                  }`}>
                    {s.status}
                  </span>
                </td>
                <td className="px-4 py-2 text-secondary-600">
                  {s.studyDate ? new Date(s.studyDate).toLocaleDateString('en-NZ') : '—'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function PendingOrdersTable({ orders }: { orders: RadiologyOrderSummary[] }) {
  const pending = orders.filter((o) => o.status === 'draft' || o.status === 'active');

  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <div className="flex items-center justify-between border-b border-secondary-100 px-4 py-3">
        <h2 className="text-sm font-semibold text-secondary-900">Pending Orders</h2>
        <Link to="/radiology/orders" className="text-xs text-primary-600 hover:underline">
          View all
        </Link>
      </div>
      {pending.length === 0 ? (
        <p className="px-4 py-6 text-center text-sm text-secondary-500">No pending orders</p>
      ) : (
        <table className="w-full text-sm">
          <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
            <tr>
              <th className="px-4 py-2 text-left">Patient NHI</th>
              <th className="px-4 py-2 text-left">Modality</th>
              <th className="px-4 py-2 text-left">Priority</th>
              <th className="px-4 py-2 text-left">Status</th>
              <th className="px-4 py-2 text-left">Requested</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-secondary-100">
            {pending.slice(0, 8).map((o) => (
              <tr key={o.id} className="hover:bg-secondary-50">
                <td className="px-4 py-2 font-mono text-xs font-medium text-secondary-900">{o.patientNhi}</td>
                <td className="px-4 py-2">
                  <ModalityBadge modality={o.modality} />
                </td>
                <td className="px-4 py-2">
                  <PriorityBadge priority={o.priority} />
                </td>
                <td className="px-4 py-2 text-secondary-600">{o.status}</td>
                <td className="px-4 py-2 text-secondary-600">
                  {new Date(o.requestedAt).toLocaleDateString('en-NZ')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function RadiologyDashboard() {
  const api = useApi();

  const [studies, setStudies] = useState<ImagingStudySummary[]>([]);
  const [orders, setOrders] = useState<RadiologyOrderSummary[]>([]);
  const [reports, setReports] = useState<RadiologyReportSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadData() {
      try {
        const [s, o, r] = await Promise.all([
          api.get<StudyListResponse>('/imaging-studies'),
          api.get<OrderListResponse>('/radiology-orders'),
          api.get<ReportListResponse>('/radiology-reports'),
        ]);
        setStudies(s.studies ?? []);
        setOrders(o.orders ?? []);
        setReports(r.reports ?? []);
      } catch {
        setError('Failed to load radiology data');
      } finally {
        setLoading(false);
      }
    }
    void loadData();
  }, [api]);

  const availableStudies = studies.filter((s) => s.status === 'available').length;
  const pendingOrders = orders.filter((o) => o.status === 'draft' || o.status === 'active').length;
  const draftReports = reports.filter((r) => r.status === 'draft' || r.status === 'preliminary').length;
  const statOrders = orders.filter((o) => o.priority === 'stat' && (o.status === 'draft' || o.status === 'active')).length;

  return (
    <AppShell title="Radiology">
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
        </div>
      ) : (
        <>
          {/* Stats */}
          <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              label="Available Studies"
              value={availableStudies}
              color="border-primary-200"
              linkTo="/radiology/studies"
            />
            <StatCard
              label="Pending Orders"
              value={pendingOrders}
              color="border-amber-200"
              linkTo="/radiology/orders"
            />
            <StatCard
              label="STAT Orders"
              value={statOrders}
              color="border-red-200"
              linkTo="/radiology/orders"
            />
            <StatCard
              label="Unsigned Reports"
              value={draftReports}
              color="border-secondary-200"
              linkTo="/radiology/reports"
            />
          </div>

          {/* Tables */}
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <RecentStudiesTable studies={studies.slice(0, 8)} />
            <PendingOrdersTable orders={orders} />
          </div>

          {/* Quick actions */}
          <div className="mt-6 flex flex-wrap gap-3">
            <Link
              to="/radiology/orders"
              className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700"
            >
              New Order
            </Link>
            <Link
              to="/radiology/studies"
              className="rounded-md bg-secondary-700 px-4 py-2 text-sm font-medium text-white hover:bg-secondary-800"
            >
              Browse Studies
            </Link>
            <Link
              to="/radiology/reports"
              className="rounded-md bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700"
            >
              Pending Reports
            </Link>
          </div>
        </>
      )}
    </AppShell>
  );
}
