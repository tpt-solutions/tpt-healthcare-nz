# UI Guide — tpt-healthcare-nz Frontend

Design system reference for `apps/tpt-clinic`, `apps/tpt-portal`, and `apps/tpt-admin`.
All apps use **React + Tailwind CSS**. Never introduce third-party component libraries (e.g. MUI, Chakra) — use the patterns below.

---

## Colour Tokens

Defined in each app's `tailwind.config.ts`. Use the semantic names, not raw hex.

### Brand — Primary (Teal)

| Token | Hex | Typical use |
|---|---|---|
| `primary-50` | `#f0fdfa` | Hover backgrounds |
| `primary-100` | `#ccfbf1` | Light tint backgrounds |
| `primary-500` | `#14b8a6` | Accents, icons |
| `primary-600` | `#0d9488` | Buttons, active nav |
| `primary-700` | `#0f766e` | Button hover |

### Brand — Secondary (Slate)

| Token | Hex | Typical use |
|---|---|---|
| `secondary-50` | `#f8fafc` | Page background |
| `secondary-100` | `#f1f5f9` | Table head, filter bar, tab bar |
| `secondary-200` | `#e2e8f0` | Borders, dividers |
| `secondary-400` | `#94a3b8` | Placeholder text, icons |
| `secondary-500` | `#64748b` | Muted / secondary text |
| `secondary-700` | `#334155` | Body text |
| `secondary-900` | `#0f172a` | Headings, sidebar |

### Clinical Alerts

| Token | Hex | Use |
|---|---|---|
| `clinical-urgent` | `#dc2626` | Critical alerts, vitals out of range |
| `clinical-warning` | `#d97706` | Warnings, pending items |
| `clinical-info` | `#2563eb` | Informational banners |
| `clinical-safe` | `#16a34a` | Normal / confirmed status |

There are also four global CSS component classes for quick clinical badges:
`.badge-urgent`, `.badge-warning`, `.badge-info`, `.badge-safe`

---

## Typography

| Role | Class |
|---|---|
| Page heading | `text-lg font-semibold text-secondary-900` |
| Section heading | `text-base font-semibold text-secondary-900` |
| Body text | `text-sm text-secondary-700` |
| Muted / secondary | `text-sm text-secondary-500` |
| Caption / label | `text-xs font-medium text-secondary-500` |
| NHI / codes | `font-mono text-xs` |

Font stack: **Inter** (sans), **JetBrains Mono** (mono).

---

## AppShell

Every page must be wrapped in `<AppShell>`. It renders the sidebar nav, top bar, offline banner, and syncing indicator.

```tsx
import AppShell from '@/components/AppShell';

export default function MyPage() {
  return (
    <AppShell title="My Page">
      {/* page content */}
    </AppShell>
  );
}
```

Props:
- `title?: string` — displayed in the top bar header
- `children: React.ReactNode` — page content, rendered inside `<main className="flex-1 overflow-y-auto p-4 lg:p-6">`

To add a new nav item, edit the `NAV_ITEMS` array in [AppShell.tsx](apps/tpt-clinic/src/components/AppShell.tsx).

---

## Tab Bar

Use the pill-style tab bar for in-page navigation. Copy this component locally into each page file (don't abstract into a shared component unless three or more pages share the exact same tab type).

```tsx
type Tab = 'overview' | 'records' | 'claims';

function TabBar({ active, onSelect }: { active: Tab; onSelect: (t: Tab) => void }) {
  const tabs: { id: Tab; label: string }[] = [
    { id: 'overview', label: 'Overview' },
    { id: 'records',  label: 'Records' },
    { id: 'claims',   label: 'ACC Claims' },
  ];
  return (
    <div className="mb-6 flex gap-1 overflow-x-auto rounded-lg bg-secondary-100 p-1">
      {tabs.map(t => (
        <button
          key={t.id}
          onClick={() => onSelect(t.id)}
          className={`flex-shrink-0 rounded-md px-3 py-1.5 text-sm font-medium transition-colors ${
            active === t.id
              ? 'bg-white text-primary-700 shadow-sm'
              : 'text-secondary-600 hover:text-secondary-900'
          }`}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}
```

To show record counts in tab labels: `{t.label}{t.count > 0 ? \` (${t.count})\` : ''}`.

---

## Stat Cards

Four-up summary row at the top of dashboard / overview tabs.

```tsx
interface StatCard { label: string; value: number; color: 'primary' | 'green' | 'amber' | 'secondary' }

function StatCards({ stats }: { stats: StatCard[] }) {
  const border: Record<string, string> = {
    primary:   'border-primary-200',
    green:     'border-green-200',
    amber:     'border-amber-200',
    secondary: 'border-secondary-200',
  };
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {stats.map(s => (
        <div key={s.label} className={`rounded-xl border ${border[s.color]} bg-white p-5 shadow-sm`}>
          <p className="text-sm font-medium text-secondary-500">{s.label}</p>
          <p className="mt-2 text-3xl font-bold text-secondary-900">{s.value}</p>
        </div>
      ))}
    </div>
  );
}
```

---

## Status Badges

Inline pill spans. Never use button elements for badges.

```tsx
// Map status strings to Tailwind classes
const statusClasses: Record<string, string> = {
  active:       'bg-blue-100 text-blue-800',
  completed:    'bg-green-100 text-green-800',
  under_review: 'bg-amber-100 text-amber-800',
  draft:        'bg-secondary-100 text-secondary-600',
  declined:     'bg-red-100 text-red-800',
  on_hold:      'bg-sky-100 text-sky-800',
};

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`inline-block rounded-full px-2.5 py-0.5 text-xs font-medium ${statusClasses[status] ?? 'bg-secondary-100 text-secondary-600'}`}>
      {status.replace(/_/g, ' ')}
    </span>
  );
}
```

---

## Data Tables

Standard table pattern. Always wrap in `overflow-x-auto` for mobile.

```tsx
<div className="rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
  {/* Header row with action button */}
  <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
    <h2 className="text-base font-semibold text-secondary-900">Section Title</h2>
    <button className="rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700">
      + New Item
    </button>
  </div>

  <div className="overflow-x-auto">
    <table className="w-full text-sm">
      <thead className="bg-secondary-50 text-xs font-medium uppercase text-secondary-500">
        <tr>
          <th className="px-6 py-3 text-left">Column</th>
          {/* ... */}
        </tr>
      </thead>
      <tbody className="divide-y divide-secondary-100">
        {rows.map(row => (
          <tr key={row.id} className="hover:bg-secondary-50">
            <td className="px-6 py-3 text-secondary-700">{row.value}</td>
          </tr>
        ))}
        {rows.length === 0 && (
          <tr>
            <td colSpan={N} className="px-6 py-8 text-center text-sm text-secondary-400">
              No items found
            </td>
          </tr>
        )}
      </tbody>
    </table>
  </div>
</div>
```

Patient name + NHI cell pattern:
```tsx
<td className="px-6 py-3">
  <p className="font-medium text-secondary-900">{row.patientName}</p>
  <p className="font-mono text-xs text-secondary-500">NHI: {row.patientNHI}</p>
</td>
```

---

## Filter Bar

Search + select filters above a table.

```tsx
<div className="mb-4 flex flex-wrap gap-3 rounded-xl bg-white p-3 shadow-sm ring-1 ring-secondary-200">
  {/* Search */}
  <div className="relative flex-1 min-w-[200px]">
    <svg className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-secondary-400" .../>
    <input
      type="search"
      placeholder="Search…"
      value={search}
      onChange={e => setSearch(e.target.value)}
      className="w-full rounded-md border border-secondary-300 py-1.5 pl-9 pr-3 text-sm focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400"
    />
  </div>
  {/* Dropdown */}
  <select
    value={filter}
    onChange={e => setFilter(e.target.value)}
    className="rounded-md border border-secondary-300 px-3 py-1.5 text-sm focus:border-primary-400 focus:outline-none focus:ring-1 focus:ring-primary-400"
  >
    <option value="all">All</option>
    {/* ... */}
  </select>
</div>
```

---

## Buttons

| Variant | Classes |
|---|---|
| Primary | `rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700` |
| Secondary / outline | `rounded-md border border-secondary-300 px-4 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-50` |
| Small primary | `rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700` |
| Danger | `rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700` |

---

## Modal / Dialog

Full-screen overlay with a centred panel. No external library — pure Tailwind.

```tsx
{open && (
  <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div className="absolute inset-0 bg-black/50" onClick={onClose} />
    <div className="relative w-full max-w-lg rounded-xl bg-white shadow-xl">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-secondary-200 px-6 py-4">
        <h2 className="text-lg font-semibold text-secondary-900">Title</h2>
        <button onClick={onClose} className="rounded p-1 text-secondary-400 hover:text-secondary-700">
          <svg className="h-5 w-5" .../>  {/* ×  icon */}
        </button>
      </div>
      {/* Body */}
      <div className="p-6">{/* content */}</div>
      {/* Footer */}
      <div className="flex justify-end gap-3 border-t border-secondary-200 px-6 py-4">
        <button onClick={onClose} className="...secondary button...">Cancel</button>
        <button className="...primary button...">Confirm</button>
      </div>
    </div>
  </div>
)}
```

---

## Toast Notifications

Auto-dismissing bottom-right toast. Implement inline per page — don't share across pages.

```tsx
type Severity = 'success' | 'error' | 'info' | 'warning';

const toastBg: Record<Severity, string> = {
  success: 'bg-green-600',
  error:   'bg-red-600',
  info:    'bg-blue-600',
  warning: 'bg-amber-500',
};

function Toast({ message, severity, onClose }: { message: string; severity: Severity; onClose: () => void }) {
  useEffect(() => {
    const t = setTimeout(onClose, 5000);
    return () => clearTimeout(t);
  }, [message, onClose]);
  return (
    <div className={`fixed bottom-4 right-4 z-50 flex items-center gap-3 rounded-lg px-4 py-3 text-sm text-white shadow-lg ${toastBg[severity]}`}>
      <span>{message}</span>
      <button onClick={onClose} className="ml-2 font-bold opacity-75 hover:opacity-100">×</button>
    </div>
  );
}

// Usage in page:
const [toast, setToast] = useState<{ message: string; severity: Severity } | null>(null);
// ...
{toast && <Toast message={toast.message} severity={toast.severity} onClose={() => setToast(null)} />}
```

---

## Loading Spinner

```tsx
<div className="flex h-32 items-center justify-center">
  <div className="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" />
</div>
```

---

## Empty State

```tsx
<div className="flex h-32 flex-col items-center justify-center gap-2 text-secondary-400">
  <p className="text-sm">No records found.</p>
  <p className="text-xs">Use the button above to add the first one.</p>
</div>
```

---

## Card / Panel

```tsx
<div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
  <h2 className="mb-4 text-base font-semibold text-secondary-900">Section</h2>
  {/* content */}
</div>
```

Use `shadow-sm ring-1 ring-secondary-200` (not `border`) for card outlines — it renders sharper at fractional pixel densities.

---

## Offline / Sync Banners

Rendered automatically by `AppShell` using the `useNetworkStatus` hook. No action needed in page components.

---

## Icons

Use inline SVG only — no icon library. Source from [Heroicons v2](https://heroicons.com) (outline style, `strokeWidth={1.5}`).

```tsx
<svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
  <path strokeLinecap="round" strokeLinejoin="round" d="..." />
</svg>
```

Standard sizes: `h-4 w-4` (small / inline), `h-5 w-5` (nav / button), `h-6 w-6` (feature icons).
