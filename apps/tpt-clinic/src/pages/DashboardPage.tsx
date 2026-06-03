import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';
import { useAuth } from '@/contexts/AuthContext';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface AppointmentSummary {
  id: string;
  patientName: string;
  patientNhiDisplay: string;
  time: string;
  type: string;
  status: 'booked' | 'arrived' | 'fulfilled' | 'cancelled';
}

interface RecentEncounter {
  id: string;
  patientName: string;
  date: string;
  reason: string;
  practitionerName: string;
}

interface UrgentAlert {
  id: string;
  type: 'lab' | 'medication' | 'referral' | 'acc';
  message: string;
  patientName: string;
  patientId: string;
  createdAt: string;
}

interface DashboardData {
  todayAppointmentCount: number;
  upcomingAppointments: AppointmentSummary[];
  recentEncounters: RecentEncounter[];
  urgentAlerts: UrgentAlert[];
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StatCard({ label, value, icon }: { label: string; value: string | number; icon: React.ReactNode }) {
  return (
    <div className="rounded-xl bg-white p-5 shadow-sm ring-1 ring-secondary-200">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-medium text-secondary-500">{label}</p>
          <p className="mt-1 text-3xl font-semibold text-secondary-900">{value}</p>
        </div>
        <div className="rounded-lg bg-primary-50 p-2 text-primary-600">{icon}</div>
      </div>
    </div>
  );
}

const STATUS_BADGE: Record<AppointmentSummary['status'], string> = {
  booked:    'badge-info',
  arrived:   'badge-warning',
  fulfilled: 'badge-safe',
  cancelled: 'inline-flex items-center rounded-full bg-secondary-100 px-2.5 py-0.5 text-xs font-medium text-secondary-600',
};

const ALERT_BADGE: Record<UrgentAlert['type'], string> = {
  lab:        'badge-urgent',
  medication: 'badge-urgent',
  referral:   'badge-warning',
  acc:        'badge-info',
};

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function DashboardPage() {
  const api = useApi();
  const { user } = useAuth();
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const result = await api.get<DashboardData>('/dashboard');
        if (!cancelled) setData(result);
      } catch {
        if (!cancelled) setError('Failed to load dashboard data.');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => { cancelled = true; };
  }, [api]);

  // Format today's date for NZ display
  const today = new Date().toLocaleDateString('en-NZ', {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  });

  return (
    <AppShell title="Dashboard">
      {/* Welcome row */}
      <div className="mb-6 flex items-baseline justify-between">
        <div>
          <h2 className="text-xl font-semibold text-secondary-900">
            Good {getGreeting()}, {user?.name?.split(' ')[0] ?? 'Doctor'}
          </h2>
          <p className="text-sm text-secondary-500">{today}</p>
        </div>
      </div>

      {loading && (
        <div className="flex items-center justify-center py-20">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
        </div>
      )}

      {error && (
        <div className="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {data && (
        <div className="space-y-6">
          {/* Stat cards */}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard
              label="Today's Appointments"
              value={data.todayAppointmentCount}
              icon={
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
              }
            />
            <StatCard
              label="Urgent Alerts"
              value={data.urgentAlerts.length}
              icon={
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                </svg>
              }
            />
            <StatCard
              label="Recent Encounters"
              value={data.recentEncounters.length}
              icon={
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 00.75-.75 2.25 2.25 0 00-.1-.664m-5.8 0A2.251 2.251 0 0113.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25z" />
                </svg>
              }
            />
            <StatCard
              label="Active Patients"
              value="—"
              icon={
                <svg className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 002.625.372 9.337 9.337 0 004.121-.952 4.125 4.125 0 00-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 018.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0111.964-3.07M12 6.375a3.375 3.375 0 11-6.75 0 3.375 3.375 0 016.75 0zm8.25 2.25a2.625 2.625 0 11-5.25 0 2.625 2.625 0 015.25 0z" />
                </svg>
              }
            />
          </div>

          {/* Urgent alerts */}
          {data.urgentAlerts.length > 0 && (
            <section>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wide text-secondary-500">
                Urgent Alerts
              </h3>
              <div className="space-y-2">
                {data.urgentAlerts.map((alert) => (
                  <div
                    key={alert.id}
                    className="flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 px-4 py-3"
                  >
                    <svg className="mt-0.5 h-4 w-4 shrink-0 text-red-600" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
                    </svg>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className={ALERT_BADGE[alert.type]}>{alert.type.toUpperCase()}</span>
                        <Link
                          to={`/patients/${alert.patientId}`}
                          className="text-sm font-medium text-secondary-900 hover:underline"
                        >
                          {alert.patientName}
                        </Link>
                      </div>
                      <p className="mt-0.5 text-sm text-secondary-700">{alert.message}</p>
                    </div>
                    <span className="shrink-0 text-xs text-secondary-400">
                      {new Date(alert.createdAt).toLocaleTimeString('en-NZ', { hour: '2-digit', minute: '2-digit' })}
                    </span>
                  </div>
                ))}
              </div>
            </section>
          )}

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            {/* Today's appointments */}
            <section>
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-sm font-semibold uppercase tracking-wide text-secondary-500">
                  Today's Appointments
                </h3>
                <Link to="/appointments" className="text-xs font-medium text-primary-600 hover:text-primary-700">
                  View all
                </Link>
              </div>
              <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
                {data.upcomingAppointments.length === 0 ? (
                  <p className="px-4 py-6 text-center text-sm text-secondary-500">No appointments today.</p>
                ) : (
                  <table className="w-full text-sm">
                    <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                      <tr>
                        <th className="px-4 py-3 text-left">Time</th>
                        <th className="px-4 py-3 text-left">Patient</th>
                        <th className="px-4 py-3 text-left">Type</th>
                        <th className="px-4 py-3 text-left">Status</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-secondary-100">
                      {data.upcomingAppointments.map((appt) => (
                        <tr key={appt.id} className="hover:bg-secondary-50">
                          <td className="whitespace-nowrap px-4 py-3 font-mono text-secondary-700">{appt.time}</td>
                          <td className="px-4 py-3">
                            <div className="font-medium text-secondary-900">{appt.patientName}</div>
                            <div className="text-xs text-secondary-400">{appt.patientNhiDisplay}</div>
                          </td>
                          <td className="px-4 py-3 text-secondary-600">{appt.type}</td>
                          <td className="px-4 py-3">
                            <span className={STATUS_BADGE[appt.status]}>{appt.status}</span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </section>

            {/* Recent encounters */}
            <section>
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-sm font-semibold uppercase tracking-wide text-secondary-500">
                  Recent Encounters
                </h3>
              </div>
              <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
                {data.recentEncounters.length === 0 ? (
                  <p className="px-4 py-6 text-center text-sm text-secondary-500">No recent encounters.</p>
                ) : (
                  <table className="w-full text-sm">
                    <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                      <tr>
                        <th className="px-4 py-3 text-left">Date</th>
                        <th className="px-4 py-3 text-left">Patient</th>
                        <th className="px-4 py-3 text-left">Reason</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-secondary-100">
                      {data.recentEncounters.map((enc) => (
                        <tr key={enc.id} className="hover:bg-secondary-50">
                          <td className="whitespace-nowrap px-4 py-3 text-secondary-500">
                            {new Date(enc.date).toLocaleDateString('en-NZ')}
                          </td>
                          <td className="px-4 py-3 font-medium text-secondary-900">{enc.patientName}</td>
                          <td className="px-4 py-3 text-secondary-600">
                            <Link to={`/encounters/${enc.id}`} className="hover:text-primary-600 hover:underline">
                              {enc.reason}
                            </Link>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </section>
          </div>
        </div>
      )}
    </AppShell>
  );
}

function getGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return 'morning';
  if (hour < 17) return 'afternoon';
  return 'evening';
}
