import React, { FormEvent, useEffect, useState } from 'react';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface BloodProduct {
  id: string;
  productType: string;
  abo: string;
  rhd: string;
  donorId: string | null;
  status: string;
  volumeMl: number;
  collectionDate: string;
  expiryDate: string;
  storageLocation: string | null;
  updatedAt: string;
}

interface InventoryListResponse {
  products: BloodProduct[];
  total: number;
}

// ---------------------------------------------------------------------------
// Status badge
// ---------------------------------------------------------------------------

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    collected: 'bg-purple-100 text-purple-700',
    tested: 'bg-blue-100 text-blue-700',
    stored: 'bg-green-100 text-green-700',
    crossmatched: 'bg-amber-100 text-amber-700',
    issued: 'bg-orange-100 text-orange-700',
    transfused: 'bg-secondary-100 text-secondary-600',
    discarded: 'bg-red-100 text-red-700',
    quarantined: 'bg-yellow-100 text-yellow-700',
  };
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colors[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function InventoryPage() {
  const api = useApi();

  const [products, setProducts] = useState<BloodProduct[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [productTypeFilter, setProductTypeFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('stored');
  const [aboFilter, setAboFilter] = useState('');

  // Create product form
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newProduct, setNewProduct] = useState({
    productType: 'rbc',
    abo: 'O',
    rhd: 'POSITIVE',
    donorId: '',
    volumeMl: 450,
    collectionDate: '',
    expiryDate: '',
    storageLocation: '',
  });

  // Status update
  const [updateProductId, setUpdateProductId] = useState<string | null>(null);
  const [newStatus, setNewStatus] = useState('tested');
  const [newLocation, setNewLocation] = useState('');

  useEffect(() => {
    loadProducts();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [productTypeFilter, statusFilter, aboFilter]);

  async function loadProducts() {
    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = {};
      if (productTypeFilter) params['productType'] = productTypeFilter;
      if (statusFilter) params['status'] = statusFilter;
      if (aboFilter) params['abo'] = aboFilter;

      const data = await api.get<InventoryListResponse>('/inventory', { params });
      setProducts(data.products);
      setTotal(data.total);
    } catch {
      setError('Failed to load inventory');
    } finally {
      setLoading(false);
    }
  }

  async function handleCreateProduct(e: FormEvent) {
    e.preventDefault();
    try {
      await api.post('/inventory', {
        ...newProduct,
        collectionDate: new Date(newProduct.collectionDate).toISOString(),
        expiryDate: new Date(newProduct.expiryDate).toISOString(),
      });
      setShowCreateForm(false);
      setNewProduct({
        productType: 'rbc', abo: 'O', rhd: 'POSITIVE',
        donorId: '', volumeMl: 450,
        collectionDate: '', expiryDate: '', storageLocation: '',
      });
      await loadProducts();
    } catch {
      setError('Failed to create product');
    }
  }

  async function handleUpdateStatus() {
    if (!updateProductId) return;
    try {
      await api.put(`/inventory/${updateProductId}/status`, {
        newStatus,
        storageLocation: newLocation,
      });
      setUpdateProductId(null);
      setNewStatus('tested');
      setNewLocation('');
      await loadProducts();
    } catch {
      setError('Failed to update product status');
    }
  }

  return (
    <AppShell title="Blood Inventory">
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {/* Toolbar */}
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="flex flex-wrap gap-2">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          >
            <option value="">All Statuses</option>
            <option value="collected">Collected</option>
            <option value="tested">Tested</option>
            <option value="stored">Stored</option>
            <option value="crossmatched">Crossmatched</option>
            <option value="issued">Issued</option>
            <option value="quarantined">Quarantined</option>
            <option value="discarded">Discarded</option>
          </select>
          <select
            value={productTypeFilter}
            onChange={(e) => setProductTypeFilter(e.target.value)}
            className="rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          >
            <option value="">All Types</option>
            <option value="rbc">RBC</option>
            <option value="platelets">Platelets</option>
            <option value="plasma">Plasma</option>
            <option value="cryo">Cryo</option>
            <option value="whole-blood">Whole Blood</option>
          </select>
          <select
            value={aboFilter}
            onChange={(e) => setAboFilter(e.target.value)}
            className="rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
          >
            <option value="">All ABO</option>
            <option value="A">A</option>
            <option value="B">B</option>
            <option value="AB">AB</option>
            <option value="O">O</option>
          </select>
        </div>
        <button
          onClick={() => setShowCreateForm(true)}
          className="flex items-center gap-1.5 rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 sm:ml-auto"
        >
          <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
          </svg>
          Add Product
        </button>
      </div>

      {/* Create form */}
      {showCreateForm && (
        <form onSubmit={handleCreateProduct} className="mb-6 rounded-xl border border-primary-200 bg-primary-50 p-4">
          <h3 className="mb-3 text-sm font-semibold text-primary-800">Add Blood Product</h3>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3 lg:grid-cols-4">
            <div>
              <label className="block text-xs font-medium text-primary-700">Type</label>
              <select
                value={newProduct.productType}
                onChange={(e) => setNewProduct({ ...newProduct, productType: e.target.value })}
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm"
              >
                <option value="rbc">RBC</option>
                <option value="platelets">Platelets</option>
                <option value="plasma">Plasma</option>
                <option value="cryo">Cryo</option>
                <option value="whole-blood">Whole Blood</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-primary-700">ABO</label>
              <select
                value={newProduct.abo}
                onChange={(e) => setNewProduct({ ...newProduct, abo: e.target.value })}
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm"
              >
                <option value="A">A</option>
                <option value="B">B</option>
                <option value="AB">AB</option>
                <option value="O">O</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-primary-700">RhD</label>
              <select
                value={newProduct.rhd}
                onChange={(e) => setNewProduct({ ...newProduct, rhd: e.target.value })}
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm"
              >
                <option value="POSITIVE">Positive</option>
                <option value="NEGATIVE">Negative</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-primary-700">Volume (mL)</label>
              <input
                type="number"
                value={newProduct.volumeMl}
                onChange={(e) => setNewProduct({ ...newProduct, volumeMl: parseInt(e.target.value) || 0 })}
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-primary-700">Collection Date</label>
              <input
                type="date"
                value={newProduct.collectionDate}
                onChange={(e) => setNewProduct({ ...newProduct, collectionDate: e.target.value })}
                required
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-primary-700">Expiry Date</label>
              <input
                type="date"
                value={newProduct.expiryDate}
                onChange={(e) => setNewProduct({ ...newProduct, expiryDate: e.target.value })}
                required
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-primary-700">Storage Location</label>
              <input
                type="text"
                value={newProduct.storageLocation}
                onChange={(e) => setNewProduct({ ...newProduct, storageLocation: e.target.value })}
                placeholder="Fridge A-1"
                className="mt-1 block w-full rounded-md border border-primary-300 bg-white px-3 py-2 text-sm"
              />
            </div>
          </div>
          <div className="mt-3 flex gap-2">
            <button type="submit" className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
              Save
            </button>
            <button type="button" onClick={() => setShowCreateForm(false)} className="rounded-md bg-white px-4 py-2 text-sm font-medium text-secondary-700 ring-1 ring-secondary-300">
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Status update modal */}
      {updateProductId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-full max-w-sm rounded-xl bg-white p-6 shadow-lg">
            <h3 className="mb-4 text-sm font-semibold text-secondary-900">Update Product Status</h3>
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-secondary-700">New Status</label>
                <select
                  value={newStatus}
                  onChange={(e) => setNewStatus(e.target.value)}
                  className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm"
                >
                  <option value="collected">Collected</option>
                  <option value="tested">Tested</option>
                  <option value="stored">Stored</option>
                  <option value="quarantined">Quarantined</option>
                  <option value="discarded">Discarded</option>
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-secondary-700">Storage Location</label>
                <input
                  type="text"
                  value={newLocation}
                  onChange={(e) => setNewLocation(e.target.value)}
                  placeholder="Fridge B-2"
                  className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm"
                />
              </div>
              <div className="flex justify-end gap-2">
                <button onClick={() => setUpdateProductId(null)} className="rounded-md bg-white px-4 py-2 text-sm font-medium text-secondary-700 ring-1 ring-secondary-300">
                  Cancel
                </button>
                <button onClick={handleUpdateStatus} className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700">
                  Update
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Product table */}
      <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
        <div className="border-b border-secondary-100 px-4 py-3">
          <p className="text-sm text-secondary-500">
            {loading ? 'Loading…' : `${total.toLocaleString()} unit${total !== 1 ? 's' : ''}`}
          </p>
        </div>

        {!loading && products.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-secondary-500">No products found.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                <tr>
                  <th className="px-4 py-3 text-left">Type</th>
                  <th className="px-4 py-3 text-left">ABO</th>
                  <th className="px-4 py-3 text-left">RhD</th>
                  <th className="px-4 py-3 text-left">Status</th>
                  <th className="px-4 py-3 text-left">Volume</th>
                  <th className="px-4 py-3 text-left">Location</th>
                  <th className="px-4 py-3 text-left">Expiry</th>
                  <th className="px-4 py-3" />
                </tr>
              </thead>
              <tbody className="divide-y divide-secondary-100">
                {products.map((p) => {
                  const expires = new Date(p.expiryDate);
                  const daysLeft = Math.ceil((expires.getTime() - Date.now()) / (1000 * 60 * 60 * 24));
                  return (
                    <tr key={p.id} className="hover:bg-secondary-50">
                      <td className="px-4 py-3 font-medium text-secondary-900">{p.productType}</td>
                      <td className="px-4 py-3 font-mono text-secondary-900">{p.abo}</td>
                      <td className="px-4 py-3 text-secondary-600">{p.rhd === 'POSITIVE' ? '+' : '-'}</td>
                      <td className="px-4 py-3"><StatusBadge status={p.status} /></td>
                      <td className="px-4 py-3 text-secondary-600">{p.volumeMl} mL</td>
                      <td className="px-4 py-3 text-secondary-600">{p.storageLocation ?? '—'}</td>
                      <td className="px-4 py-3">
                        <span className={`font-medium ${daysLeft <= 1 ? 'text-red-600' : daysLeft <= 3 ? 'text-yellow-600' : 'text-secondary-600'}`}>
                          {expires.toLocaleDateString('en-NZ')}
                          {daysLeft > 0 && daysLeft <= 7 && ` (${daysLeft}d)`}
                        </span>
                      </td>
                      <td className="px-4 py-3">
                        {p.status !== 'transfused' && p.status !== 'discarded' && (
                          <button
                            onClick={() => {
                              setUpdateProductId(p.id);
                              setNewStatus(p.status === 'collected' ? 'tested' : 'stored');
                            }}
                            className="rounded px-2 py-1 text-xs font-medium text-primary-700 hover:bg-primary-50"
                          >
                            Update
                          </button>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </AppShell>
  );
}