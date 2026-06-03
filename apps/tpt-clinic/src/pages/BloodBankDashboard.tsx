import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface DonorSummary {
  id: string;
  nhi: string;
  bloodGroup: string;
  rhd: string;
  status: string;
  totalDonations: number;
  lastDonationAt: string | null;
  haemoglobinGdl: number | null;
}

interface InventorySummary {
  id: string;
  productType: string;
  abo: string;
  rhd: string;
  status: string;
  expiryDate: string;
}

interface CrossmatchSummary {
  id: string;
  patientId: string;
  status: string;
  compatibility: string;
  requestedAt: string;
}

interface DonorListResponse {
  donors: DonorSummary[];
  total: number;
}

interface InventoryListResponse {
  products: InventorySummary[];
  total: number;
}

interface CrossmatchListResponse {
  crossmatches: CrossmatchSummary[];
  total: number;
}

// ---------------------------------------------------------------------------
// Stats card
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

// ---------------------------------------------------------------------------
// Recent activity table
// ---------------------------------------------------------------------------

function RecentCrossmatches({ crossmatches }: { crossmatches: CrossmatchSummary[] }) {
  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <div className="border-b border-secondary-100 px-4 py-3">
        <h2 className="text-sm font-semibold text-secondary-900">Recent Cross-matches</h2>
      </div>
      {crossmatches.length === 0 ? (
        <p className="px-4 py-6 text-center text-sm text-secondary-500">No recent cross-matches</p>
      ) : (
        <table className="w-full text-sm">
          <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
            <tr>
              <th className="px-4 py-2 text-left">Patient</th>
              <th className="px-4 py-2 text-left">Status</th>
              <th className="px-4 py-2 text-left">Compatibility</th>
              <th className="px-4 py-2 text-left">Requested</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-secondary-100">
            {crossmatches.map((xm) => (
              <tr key={xm.id} className="hover:bg-secondary-50">
                <td className="px-4 py-2 font-medium text-secondary-900">{xm.patientId}</td>
                <td className="px-4 py-2">
                  <span
                    className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${
                      xm.status === 'transfused'
                        ? 'bg-green-100 text-green-700'
                        : xm.status === 'issued'
                          ? 'bg-blue-100 text-blue-700'
                          : xm.status === 'cancelled'
                            ? 'bg-red-100 text-red-700'
                            : xm.status === 'incompatible'
                              ? 'bg-yellow-100 text-yellow-700'
                              : 'bg-secondary-100 text-secondary-600'
                    }`}
                  >
                    {xm.status}
                  </span>
                </td>
                <td className="px-4 py-2 text-secondary-600">{xm.compatibility}</td>
                <td className="px-4 py-2 text-secondary-600">
                  {new Date(xm.requestedAt).toLocaleDateString('en-NZ')}
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
// Expiring products table
// ---------------------------------------------------------------------------

function ExpiringProducts({ products }: { products: InventorySummary[] }) {
  const expiring = products.filter(
    (p) => p.status !== 'transfused' && p.status !== 'discarded',
  );

  return (
    <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
      <div className="border-b border-secondary-100 px-4 py-3">
        <h2 className="text-sm font-semibold text-secondary-900">Expiring Blood Products</h2>
      </div>
      {expiring.length === 0 ? (
        <p className="px-4 py-6 text-center text-sm text-secondary-500">No expiring products</p>
      ) : (
        <table className="w-full text-sm">
          <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
            <tr>
              <th className="px-4 py-2 text-left">Type</th>
              <th className="px-4 py-2 text-left">ABO/RhD</th>
              <th className="px-4 py-2 text-left">Status</th>
              <th className="px-4 py-2 text-left">Expiry</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-secondary-100">
            {expiring.map((p) => {
              const expires = new Date(p.expiryDate);
              const daysLeft = Math.ceil(
                (expires.getTime() - Date.now()) / (1000 * 60 * 60 * 24),
              );
              return (
                <tr key={p.id} className="hover:bg-secondary-50">
                  <td className="px-4 py-2 font-medium text-secondary-900">{p.productType}</td>
                  <td className="px-4 py-2 font-mono text-secondary-600">
                    {p.abo}{p.rhd === 'POSITIVE' ? '+' : '-'}
                  </td>
                  <td className="px-4 py-2 text-secondary-600">{p.status}</td>
                  <td className="px-4 py-2">
                    <span
                      className={`font-medium ${
                        daysLeft <= 1 ? 'text-red-600' : daysLeft <= 3 ? 'text-yellow-600' : 'text-secondary-600'
                      }`}
                    >
                      {expires.toLocaleDateString('en-NZ')} ({daysLeft}d)
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function BloodBankDashboard() {
  const api = useApi();

  const [donors, setDonors] = useState<DonorSummary[]>([]);
  const [products, setProducts] = useState<InventorySummary[]>([]);
  const [crossmatches, setCrossmatches] = useState<CrossmatchSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadData() {
      try {
        const [d, p, x] = await Promise.all([
          api.get<DonorListResponse>('/donors', { params: { status: 'active' } }),
          api.get<InventoryListResponse>('/inventory/expiring'),
          api.get<CrossmatchListResponse>('/crossmatches', { params: { status: '' } }),
        ]);
        setDonors(d.donors);
        setProducts(p.products);
        setCrossmatches(x.crossmatches);
      } catch (err) {
        setError('Failed to load blood bank data');
      } finally {
        setLoading(false);
      }
    }
    void loadData();
  }, [api]);

  const activeDonors = donors.filter((d) => d.status === 'active').length;
  const availableUnits = products.filter(
    (p) => p.status === 'stored' || p.status === 'tested',
  ).length;
  const pendingCrossmatches = crossmatches.filter((x) => x.status === 'matched').length;

  return (
    <AppShell title="Blood Bank">
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
              label="Active Donors"
              value={activeDonors}
              color="border-primary-200"
              linkTo="/blood-bank/donors"
            />
            <StatCard
              label="Available Units"
              value={availableUnits}
              color="border-green-200"
              linkTo="/blood-bank/inventory"
            />
            <StatCard
              label="Pending Cross-matches"
              value={pendingCrossmatches}
              color="border-amber-200"
              linkTo="/blood-bank/crossmatch"
            />
            <StatCard
              label="Total Donors"
              value={donors.length}
              color="border-secondary-200"
              linkTo="/blood-bank/donors"
            />
          </div>

          {/* Two-column layout */}
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <RecentCrossmatches crossmatches={crossmatches.slice(0, 10)} />
            <ExpiringProducts products={products} />
          </div>

          {/* Quick actions */}
          <div className="mt-6 flex flex-wrap gap-3">
            <Link
              to="/blood-bank/donors"
              className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700"
            >
              Manage Donors
            </Link>
            <Link
              to="/blood-bank/inventory"
              className="rounded-md bg-secondary-700 px-4 py-2 text-sm font-medium text-white hover:bg-secondary-800"
            >
              View Inventory
            </Link>
            <Link
              to="/blood-bank/crossmatch"
              className="rounded-md bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700"
            >
              Cross-match
            </Link>
          </div>
        </>
      )}
    </AppShell>
  );
}