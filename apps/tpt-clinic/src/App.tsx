import React, { useState } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useRegisterSW } from 'virtual:pwa-register/react';
import { AuthProvider, useAuth } from '@/contexts/AuthContext';
import { ApiProvider } from '@/contexts/ApiContext';
import { PINProvider, LockScreenOverlay, usePIN } from '@tpt/offline-store';
import { ThemeProvider } from '@tpt/ui';
import { usePowerSave } from '@/hooks/usePowerSave';
import { useOfflineSync } from '@/hooks/useOfflineSync';

// Activates power-save signalling and pre-fetches today's patients into IndexedDB.
function AppSecurity() {
  const { cryptoKey } = usePIN();
  const { isPowerSave } = usePowerSave();
  useOfflineSync(cryptoKey, isPowerSave);
  return null;
}

// Toast shown when a new service worker is waiting to activate.
function UpdateToast() {
  const {
    needRefresh: [needRefresh, setNeedRefresh],
    updateServiceWorker,
  } = useRegisterSW();
  const [dismissed, setDismissed] = useState(false);

  if (!needRefresh || dismissed) return null;

  return (
    <div className="fixed bottom-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 rounded-xl bg-secondary-900 px-4 py-3 text-sm text-white shadow-lg">
      <span>A new version is available.</span>
      <button
        onClick={() => { void updateServiceWorker(true); }}
        className="rounded-md bg-primary-500 px-3 py-1 text-xs font-medium hover:bg-primary-400 transition-colors"
      >
        Reload to update
      </button>
      <button
        onClick={() => { setNeedRefresh(false); setDismissed(true); }}
        className="ml-1 text-secondary-400 hover:text-white"
        aria-label="Dismiss"
      >
        ✕
      </button>
    </div>
  );
}

// Lazy-loaded pages for code splitting
const LoginPage         = React.lazy(() => import('@/pages/LoginPage'));
const DashboardPage     = React.lazy(() => import('@/pages/DashboardPage'));
const PatientListPage   = React.lazy(() => import('@/pages/PatientListPage'));
const NewPatientPage    = React.lazy(() => import('@/pages/NewPatientPage'));
const PatientDetailPage = React.lazy(() => import('@/pages/PatientDetailPage'));
const AppointmentsPage   = React.lazy(() => import('@/pages/AppointmentsPage'));
const EncounterPage      = React.lazy(() => import('@/pages/EncounterPage'));
const PrescriptionsPage  = React.lazy(() => import('@/pages/PrescriptionsPage'));

// Blood bank pages.
const BloodBankDashboard = React.lazy(() => import('@/pages/BloodBankDashboard'));
const DonorsPage         = React.lazy(() => import('@/pages/DonorsPage'));
const InventoryPage      = React.lazy(() => import('@/pages/InventoryPage'));
const CrossmatchPage     = React.lazy(() => import('@/pages/CrossmatchPage'));

// Radiology pages.
const RadiologyDashboard   = React.lazy(() => import('@/pages/RadiologyDashboard'));
const ImagingStudiesPage   = React.lazy(() => import('@/pages/ImagingStudiesPage'));
const RadiologyOrdersPage  = React.lazy(() => import('@/pages/RadiologyOrdersPage'));
const RadiologyReportPage  = React.lazy(() => import('@/pages/RadiologyReportPage'));

// Queue
const QueuePage = React.lazy(() => import('@/pages/QueuePage'));

// Diagnostics
const DiagnosticsPage = React.lazy(() => import('@/pages/DiagnosticsPage'));

// Aged care pages.
const AgedCareDashboard     = React.lazy(() => import('@/pages/AgedCareDashboard'));
const InterRaiAssessmentPage = React.lazy(() => import('@/pages/InterRaiAssessmentPage'));
const NascReferralPage      = React.lazy(() => import('@/pages/NascReferralPage'));
const FundedHoursPage       = React.lazy(() => import('@/pages/FundedHoursPage'));
const CarePlanPage          = React.lazy(() => import('@/pages/CarePlanPage'));

// CAM pages.
const AcupuncturePage    = React.lazy(() => import('@/pages/AcupuncturePage'));
const ChiropracticPage   = React.lazy(() => import('@/pages/ChiropracticPage'));
const OsteopathyPage     = React.lazy(() => import('@/pages/OsteopathyPage'));
const MassagePage        = React.lazy(() => import('@/pages/MassagePage'));
const CounsellingPage    = React.lazy(() => import('@/pages/CounsellingPage'));
const NaturopathyPage    = React.lazy(() => import('@/pages/NaturopathyPage'));
const TcmPage            = React.lazy(() => import('@/pages/TcmPage'));
const NutritionPage      = React.lazy(() => import('@/pages/NutritionPage'));
const VisionPage         = React.lazy(() => import('@/pages/VisionPage'));

// Allied Health pages.
const AlliedHealthPage   = React.lazy(() => import('@/pages/AlliedHealthPage'));

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

        {/* Blood bank routes */}
        <Route
          path="/blood-bank"
          element={
            <ProtectedRoute>
              <BloodBankDashboard />
            </ProtectedRoute>
          }
        />
        <Route
          path="/blood-bank/donors"
          element={
            <ProtectedRoute>
              <DonorsPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/blood-bank/inventory"
          element={
            <ProtectedRoute>
              <InventoryPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/blood-bank/crossmatch"
          element={
            <ProtectedRoute>
              <CrossmatchPage />
            </ProtectedRoute>
          }
        />

        {/* Radiology routes */}
        <Route
          path="/radiology"
          element={
            <ProtectedRoute>
              <RadiologyDashboard />
            </ProtectedRoute>
          }
        />
        <Route
          path="/radiology/studies"
          element={
            <ProtectedRoute>
              <ImagingStudiesPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/radiology/orders"
          element={
            <ProtectedRoute>
              <RadiologyOrdersPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/radiology/reports"
          element={
            <ProtectedRoute>
              <RadiologyReportPage />
            </ProtectedRoute>
          }
        />

        {/* Queue */}
        <Route
          path="/queue"
          element={
            <ProtectedRoute>
              <QueuePage />
            </ProtectedRoute>
          }
        />

        {/* Diagnostics */}
        <Route
          path="/diagnostics"
          element={
            <ProtectedRoute>
              <DiagnosticsPage />
            </ProtectedRoute>
          }
        />

        {/* Aged care routes */}
        <Route
          path="/aged-care"
          element={
            <ProtectedRoute>
              <AgedCareDashboard />
            </ProtectedRoute>
          }
        />
        <Route
          path="/aged-care/interrai"
          element={
            <ProtectedRoute>
              <InterRaiAssessmentPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/aged-care/nasc"
          element={
            <ProtectedRoute>
              <NascReferralPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/aged-care/funded-hours"
          element={
            <ProtectedRoute>
              <FundedHoursPage />
            </ProtectedRoute>
          }
        />
        <Route
          path="/aged-care/care-plans"
          element={
            <ProtectedRoute>
              <CarePlanPage />
            </ProtectedRoute>
          }
        />

        {/* CAM routes — Complementary & Alternative Medicine
            Each page handles its own tab-based sub-navigation internally.
            The /* wildcard allows hash-based deep-linking to specific tabs. */}
        <Route path="/acupuncture/*"  element={<ProtectedRoute><AcupuncturePage /></ProtectedRoute>} />
        <Route path="/chiropractic/*" element={<ProtectedRoute><ChiropracticPage /></ProtectedRoute>} />
        <Route path="/osteopathy/*"   element={<ProtectedRoute><OsteopathyPage /></ProtectedRoute>} />
        <Route path="/massage/*"      element={<ProtectedRoute><MassagePage /></ProtectedRoute>} />
        <Route path="/counselling/*"  element={<ProtectedRoute><CounsellingPage /></ProtectedRoute>} />
        <Route path="/naturopathy/*"  element={<ProtectedRoute><NaturopathyPage /></ProtectedRoute>} />
        <Route path="/tcm/*"          element={<ProtectedRoute><TcmPage /></ProtectedRoute>} />
        <Route path="/nutrition/*"    element={<ProtectedRoute><NutritionPage /></ProtectedRoute>} />
        <Route path="/vision"         element={<ProtectedRoute><VisionPage /></ProtectedRoute>} />
        <Route path="/allied-health"  element={<ProtectedRoute><AlliedHealthPage /></ProtectedRoute>} />

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
  // Read lock timeout from localStorage (configurable via tpt-admin Settings).
  // Defaults to 30 seconds per MoH Mobile Device Policy for clinical apps.
  const inactivityMs = Number(localStorage.getItem('tpt:lockTimeout') ?? 30_000);

  return (
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
          <ApiProvider>
            <PINProvider inactivityMs={inactivityMs}>
              {/* AppSecurity must be inside PINProvider to access usePIN */}
              <AppSecurity />
              <LockScreenOverlay />
              <AppRoutes />
              <UpdateToast />
            </PINProvider>
          </ApiProvider>
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  );
}
