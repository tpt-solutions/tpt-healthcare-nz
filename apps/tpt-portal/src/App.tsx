import { useState } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, ErrorBoundary } from '@tpt/ui';
import { useRegisterSW } from 'virtual:pwa-register/react';
import { AuthProvider } from './contexts/AuthContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { NavLayout } from './components/NavLayout';
import { LoginPage } from './pages/LoginPage';
import { DashboardPage } from './pages/DashboardPage';
import { AppointmentsPage } from './pages/AppointmentsPage';
import { BookAppointmentPage } from './pages/BookAppointmentPage';
import { WaitingPage } from './pages/WaitingPage';
import { RecordsPage } from './pages/RecordsPage';
import { PrescriptionsPage } from './pages/PrescriptionsPage';
import { MessagesPage } from './pages/MessagesPage';
import { ConsentPage } from './pages/ConsentPage';
import { usePushSetup } from './hooks/usePushSetup';
import { PINProvider, LockScreenOverlay, usePIN } from '@tpt/offline-store';
import { usePowerSave } from './hooks/usePowerSave';
import { useOfflineSync } from './hooks/useOfflineSync';

function AppSecurity() {
  const { cryptoKey } = usePIN();
  const { isPowerSave } = usePowerSave();
  usePushSetup();
  useOfflineSync(cryptoKey, isPowerSave);
  return null;
}

function UpdateToast() {
  const {
    needRefresh: [needRefresh, setNeedRefresh],
    updateServiceWorker,
  } = useRegisterSW();
  const [dismissed, setDismissed] = useState(false);

  if (!needRefresh || dismissed) return null;

  return (
    <div className="fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 rounded-xl bg-gray-900 px-4 py-3 text-sm text-white shadow-lg">
      <span>A new version is available.</span>
      <button
        onClick={() => { void updateServiceWorker(true); }}
        className="rounded-md bg-teal-500 px-3 py-1 text-xs font-medium hover:bg-teal-400 transition-colors"
      >
        Reload to update
      </button>
      <button
        onClick={() => { setNeedRefresh(false); setDismissed(true); }}
        className="ml-1 text-gray-400 hover:text-white"
        aria-label="Dismiss"
      >
        ✕
      </button>
    </div>
  );
}

export default function App() {
  // Patient portal: 2-minute inactivity lock (less sensitive data than staff app).
  // Configurable via tpt-admin Settings stored in localStorage.
  const inactivityMs = Number(localStorage.getItem('tpt:lockTimeout') ?? 120_000);

  return (
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
        <PINProvider inactivityMs={inactivityMs}>
          <AppSecurity />
          <LockScreenOverlay />
          <ErrorBoundary>
            <Routes>
              <Route path="/login" element={<LoginPage />} />

              <Route
                element={
                  <ProtectedRoute>
                    <NavLayout />
                  </ProtectedRoute>
                }
              >
                <Route path="/dashboard" element={<DashboardPage />} />
                <Route path="/waiting" element={<WaitingPage />} />
                <Route path="/appointments" element={<AppointmentsPage />} />
                <Route path="/appointments/book" element={<BookAppointmentPage />} />
                <Route path="/records" element={<RecordsPage />} />
                <Route path="/prescriptions" element={<PrescriptionsPage />} />
                <Route path="/messages" element={<MessagesPage />} />
                <Route path="/consent" element={<ConsentPage />} />
              </Route>

              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route path="*" element={<Navigate to="/dashboard" replace />} />
            </Routes>
          </ErrorBoundary>
        <UpdateToast />
        </PINProvider>
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  );
}
