import React, { useState } from 'react';
import { Link, NavLink, useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { useNetworkStatus, formatRelativeTime } from '@/hooks/useNetworkStatus';

interface NavItem {
  label: string;
  to: string;
  icon: React.ReactNode;
}

const NAV_ITEMS: NavItem[] = [
  {
    label: 'Dashboard',
    to: '/dashboard',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
      </svg>
    ),
  },
  {
    label: 'Patients',
    to: '/patients',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
    ),
  },
  {
    label: 'Appointments',
    to: '/appointments',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
      </svg>
    ),
  },
  {
    label: 'Queue',
    to: '/queue',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z" />
      </svg>
    ),
  },
  {
    label: 'Prescriptions',
    to: '/prescriptions',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
    ),
  },
  {
    label: 'Blood Bank',
    to: '/blood-bank',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L12 2Z" />
      </svg>
    ),
  },
  {
    label: 'Radiology',
    to: '/radiology',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M6.827 6.175A2.31 2.31 0 015.186 7.23c-.38.054-.757.112-1.134.175C2.999 7.58 2.25 8.507 2.25 9.574V18a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9.574c0-1.067-.75-1.994-1.802-2.169a47.865 47.865 0 00-1.134-.175 2.31 2.31 0 01-1.64-1.055l-.822-1.316a2.192 2.192 0 00-1.736-1.039 48.774 48.774 0 00-5.232 0 2.192 2.192 0 00-1.736 1.039l-.821 1.316z" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 12.75a4.5 4.5 0 11-9 0 4.5 4.5 0 019 0zM18.75 10.5h.008v.008h-.008V10.5z" />
      </svg>
    ),
  },
  {
    label: 'Diagnostics',
    to: '/diagnostics',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714a2.25 2.25 0 001.36 2.05l.214.096m-1.574-7.846c.25.023.499.05.75.082M6 18.375a3.375 3.375 0 006.75 0M6 18.375A3.375 3.375 0 012.625 15H21.375A3.375 3.375 0 0118 18.375m-12 0h12" />
      </svg>
    ),
  },
  {
    label: 'Aged Care',
    to: '/aged-care',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
      </svg>
    ),
  },
  // CAM modules — Complementary & Alternative Medicine
  {
    label: 'Acupuncture',
    to: '/acupuncture',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m-4-4l4 4 4-4M6 8l6-6 6 6" />
      </svg>
    ),
  },
  {
    label: 'Chiropractic',
    to: '/chiropractic',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" />
      </svg>
    ),
  },
  {
    label: 'Massage',
    to: '/massage',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    ),
  },
  {
    label: 'Counselling',
    to: '/counselling',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155" />
      </svg>
    ),
  },
  {
    label: 'Naturopathy',
    to: '/naturopathy',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714a2.25 2.25 0 001.36 2.05l.214.096m-1.574-7.846c.25.023.499.05.75.082M6 18.375a3.375 3.375 0 006.75 0M6 18.375A3.375 3.375 0 012.625 15H21.375A3.375 3.375 0 0118 18.375m-12 0h12" />
      </svg>
    ),
  },
  {
    label: 'TCM',
    to: '/tcm',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v12m-3-2.818l.879.659c1.171.879 3.07.879 4.242 0 1.172-.879 1.172-2.303 0-3.182C13.536 12.219 12.768 12 12 12c-.725 0-1.45-.22-2.064-.659a2.454 2.454 0 01-.783-1.277 2.46 2.46 0 01.167-1.51 2.465 2.465 0 011.003-1.064c.614-.44 1.34-.659 2.064-.659.768 0 1.536.22 2.121.659l.879.659" />
      </svg>
    ),
  },
  {
    label: 'Nutrition',
    to: '/nutrition',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12l8.954-8.955a1.126 1.126 0 011.591 0l8.955 8.955M3.75 9.75v10.5a.75.75 0 00.75.75h3.75a.75.75 0 00.75-.75v-4.5a.75.75 0 01.75-.75h3a.75.75 0 01.75.75v4.5a.75.75 0 00.75.75h3.75a.75.75 0 00.75-.75V9.75" />
      </svg>
    ),
  },
  {
    label: 'Vision',
    to: '/vision',
    icon: (
      <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
        <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
    ),
  },
];

interface AppShellProps {
  children: React.ReactNode;
  title?: string;
}

export default function AppShell({ children, title }: AppShellProps) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const { online, lastSynced, isSyncing } = useNetworkStatus();

  function handleLogout() {
    logout();
    void navigate('/login', { replace: true });
  }

  return (
    <div className="flex h-full">
      {/* Sidebar — desktop */}
      <aside className="hidden w-64 flex-col bg-secondary-900 lg:flex">
        {/* Logo */}
        <div className="flex h-16 items-center gap-3 px-6">
          <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary-500">
            <span className="text-sm font-bold text-white">T</span>
          </div>
          <span className="text-lg font-semibold text-white">TPT Clinic</span>
        </div>

        {/* Nav */}
        <nav className="flex-1 space-y-1 px-3 py-4">
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                [
                  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary-600 text-white'
                    : 'text-secondary-300 hover:bg-secondary-800 hover:text-white',
                ].join(' ')
              }
            >
              {item.icon}
              {item.label}
            </NavLink>
          ))}
        </nav>

        {/* User info */}
        <div className="border-t border-secondary-700 p-4">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary-600 text-sm font-medium text-white">
              {user?.name?.charAt(0).toUpperCase() ?? '?'}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-white">{user?.name ?? 'Unknown'}</p>
              <p className="truncate text-xs text-secondary-400">{user?.email ?? ''}</p>
            </div>
            <button
              onClick={handleLogout}
              title="Sign out"
              className="rounded p-1 text-secondary-400 hover:text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
              </svg>
            </button>
          </div>
        </div>
      </aside>

      {/* Main content area */}
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Top bar */}
        <header className="flex h-16 items-center justify-between border-b border-secondary-200 bg-white px-4 lg:px-6">
          {/* Mobile hamburger */}
          <button
            className="rounded-md p-2 text-secondary-500 hover:bg-secondary-100 lg:hidden"
            onClick={() => setMobileMenuOpen((v) => !v)}
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>

          {/* Page title */}
          {title && (
            <h1 className="text-lg font-semibold text-secondary-900 lg:ml-0">{title}</h1>
          )}

          {/* Right-hand actions */}
          <div className="flex items-center gap-2 ml-auto">
            <Link
              to="/patients/new"
              className="hidden items-center gap-2 rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-primary-700 sm:flex"
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
              </svg>
              New Patient
            </Link>
          </div>
        </header>

        {/* Mobile nav overlay */}
        {mobileMenuOpen && (
          <div className="absolute inset-0 z-50 flex lg:hidden">
            <div
              className="fixed inset-0 bg-black/50"
              onClick={() => setMobileMenuOpen(false)}
            />
            <aside className="relative flex w-64 flex-col bg-secondary-900">
              <div className="flex h-16 items-center gap-3 px-6">
                <div className="flex h-8 w-8 items-center justify-center rounded-md bg-primary-500">
                  <span className="text-sm font-bold text-white">T</span>
                </div>
                <span className="text-lg font-semibold text-white">TPT Clinic</span>
              </div>
              <nav className="flex-1 space-y-1 px-3 py-4">
                {NAV_ITEMS.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    onClick={() => setMobileMenuOpen(false)}
                    className={({ isActive }) =>
                      [
                        'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                        isActive
                          ? 'bg-primary-600 text-white'
                          : 'text-secondary-300 hover:bg-secondary-800 hover:text-white',
                      ].join(' ')
                    }
                  >
                    {item.icon}
                    {item.label}
                  </NavLink>
                ))}
              </nav>
            </aside>
          </div>
        )}

        {/* Offline / stale-data banner */}
        {!online && (
          <div className="flex items-center gap-2 border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-800">
            <svg className="h-4 w-4 flex-shrink-0" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
            </svg>
            <span>
              Offline — showing cached data
              {lastSynced && ` from ${formatRelativeTime(lastSynced)}`}.
              {' '}Changes will sync when reconnected.
            </span>
          </div>
        )}
        {online && isSyncing && (
          <div className="flex items-center gap-2 border-b border-blue-100 bg-blue-50 px-4 py-2 text-sm text-blue-700">
            <svg className="h-4 w-4 animate-spin flex-shrink-0" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
            </svg>
            <span>Syncing…</span>
          </div>
        )}

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-4 lg:p-6">{children}</main>
      </div>
    </div>
  );
}
