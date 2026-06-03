import React from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { AuthProvider, useAuth } from '@/contexts/AuthContext';
import { ApiProvider } from '@/contexts/ApiContext';

// Lazy-loaded pages for code splitting
const LoginPage        = React.lazy(() => import('@/pages/LoginPage'));
const DashboardPage    = React.lazy(() => import('@/pages/DashboardPage'));
const PatientListPage  = React.lazy(() => import('@/pages/PatientListPage'));
const NewPatientPage   = React.lazy(() => import('@/pages/NewPatientPage'));
const PatientDetailPage = React.lazy(() => import('@/pages/PatientDetailPage'));
const AppointmentsPage  = React.lazy(() => import('@/pages/AppointmentsPage'));
const EncounterPage     = React.lazy(() => import('@/pages/EncounterPage'));
const PrescriptionsPage = React.lazy(() => import('@/pages/PrescriptionsPage'));

// ---------------------------------------------------------------------------
// Protected route guard
// ---------------------------------------------------------------------------
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}

// ---------------------------------------------------------------------------
// Suspense fallback
// ---------------------------------------------------------------------------
function PageLoader() {
  return (
    <div className="flex h-screen items-center justify-center bg-secondary-50">
      <div className="flex flex-col items-center gap-3">
        <div className="h-10 w-10 animate-spin rounded-full border-4 border-primary-500 border-t-transparent" />
        <p className="text-sm text-secondary-500">Loading…</p>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Router tree
// ---------------------------------------------------------------------------
function AppRoutes() {
  return (
    <React.Suspense fallback={<PageLoader />}>
      <Routes>
        {/* Public */}
        <Route path="/login" element={<LoginPage />} />

        {/* Root redirect */}
        <Route path="/" element={<Navigate to="/dashboard" replace />} />

        {/* Protected routes */}
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <DashboardPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/patients"
          element={
            <ProtectedRoute>
              <PatientListPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/patients/new"
          element={
            <ProtectedRoute>
              <NewPatientPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/patients/:id"
          element={
            <ProtectedRoute>
              <PatientDetailPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/appointments"
          element={
            <ProtectedRoute>
              <AppointmentsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/encounters/:id"
          element={
            <ProtectedRoute>
              <EncounterPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/prescriptions"
          element={
            <ProtectedRoute>
              <PrescriptionsPage />
            </ProtectedRoute>
          }
        />

        {/* Catch-all */}
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </React.Suspense>
  );
}

// ---------------------------------------------------------------------------
// Root app — provider composition
// ---------------------------------------------------------------------------
export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <ApiProvider>
          <AppRoutes />
        </ApiProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
