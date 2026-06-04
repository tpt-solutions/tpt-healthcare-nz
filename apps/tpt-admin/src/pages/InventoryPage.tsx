import { useEffect, useState } from 'react';

interface StockItem {
  id: string;
  sku: string;
  name: string;
  category: string;
  unit: string;
  quantity_on_hand: number;
  reorder_point: number;
  expiry_date?: string;
}

export function InventoryPage() {
  const [items, setItems] = useState<StockItem[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch('/api/v1/practice/inventory')
      .then(r => r.json())
      .then(data => setItems(Array.isArray(data) ? data : []))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <div className="p-6 text-gray-500">Loading inventory…</div>;

  const lowStock = items.filter(i => i.quantity_on_hand <= i.reorder_point);
  const today = new Date().toISOString().slice(0, 10);
  const expiring = items.filter(i => i.expiry_date && i.expiry_date <= new Date(Date.now() + 30 * 86400000).toISOString().slice(0, 10));

  return (
    <div className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Inventory</h1>
        <button className="px-4 py-2 bg-indigo-600 text-white rounded-lg text-sm font-medium hover:bg-indigo-700">
          Add stock item
        </button>
      </div>

      {/* Alert banners */}
      {lowStock.length > 0 && (
        <div className="mb-4 bg-yellow-50 border border-yellow-200 rounded-lg p-3 text-sm text-yellow-800">
          {lowStock.length} item{lowStock.length > 1 ? 's' : ''} below reorder point
        </div>
      )}
      {expiring.length > 0 && (
        <div className="mb-4 bg-red-50 border border-red-200 rounded-lg p-3 text-sm text-red-800">
          {expiring.length} item{expiring.length > 1 ? 's' : ''} expiring within 30 days
        </div>
      )}

      {items.length === 0 ? (
        <div className="bg-white rounded-xl border border-gray-200 p-12 text-center text-gray-500">
          No stock items. Add your first item to start tracking inventory.
        </div>
      ) : (
        <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                {['SKU', 'Name', 'Category', 'On hand', 'Reorder at', 'Expiry'].map(h => (
                  <th key={h} className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {items.map(item => {
                const isLow = item.quantity_on_hand <= item.reorder_point;
                const isExpiring = item.expiry_date && item.expiry_date <= new Date(Date.now() + 30 * 86400000).toISOString().slice(0, 10);
                return (
                  <tr key={item.id} className={`hover:bg-gray-50 ${isLow ? 'bg-yellow-50' : ''}`}>
                    <td className="px-4 py-3 text-sm font-mono text-gray-700">{item.sku}</td>
                    <td className="px-4 py-3 text-sm text-gray-900 font-medium">{item.name}</td>
                    <td className="px-4 py-3 text-sm text-gray-500 capitalize">{item.category}</td>
                    <td className="px-4 py-3 text-sm font-medium" style={{ color: isLow ? '#b45309' : '#111827' }}>
                      {item.quantity_on_hand} {item.unit}
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-500">{item.reorder_point} {item.unit}</td>
                    <td className="px-4 py-3 text-sm" style={{ color: isExpiring ? '#b91c1c' : '#6b7280' }}>
                      {item.expiry_date ?? '—'}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
