import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { ThemeProvider } from '@tpt/ui';
import { ProtectedRoute } from './components/ProtectedRoute';
import { NavLayout } from './components/NavLayout';
import { LoginPage } from './pages/LoginPage';
import { DashboardPage } from './pages/DashboardPage';
import { PractitionersPage } from './pages/PractitionersPage';
import { BillingPage } from './pages/BillingPage';
import { ReportsPage } from './pages/ReportsPage';
import { AuditPage } from './pages/AuditPage';
import { SettingsPage } from './pages/SettingsPage';
import { SecurityPage } from './pages/SecurityPage';
import { ClinicsPage } from './pages/ClinicsPage';
// Milestone 11 — Practice Management & Operations
import { OnboardingWizard } from './pages/OnboardingWizard';
import { RosterPage } from './pages/RosterPage';
import { RoomsPage } from './pages/RoomsPage';
import { LeavePage } from './pages/LeavePage';
import { InvoicesPage } from './pages/InvoicesPage';
import { InventoryPage } from './pages/InventoryPage';
import { BudgetPage } from './pages/BudgetPage';
import { DepartmentsPage } from './pages/DepartmentsPage';
import { RolesPage } from './pages/RolesPage';
import { IntegrationsPage } from './pages/IntegrationsPage';
import { ACCProviderPage } from './pages/ACCProviderPage';

export default function App() {
  return (
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/onboarding" element={<OnboardingWizard />} />

          <Route
            element={
              <ProtectedRoute>
                <NavLayout />
              </ProtectedRoute>
            }
          >
            {/* Existing routes */}
            <Route path="/dashboard" element={<DashboardPage />} />
            <Route path="/clinics" element={<ClinicsPage />} />
            <Route path="/practitioners" element={<PractitionersPage />} />
            <Route path="/billing" element={<BillingPage />} />
            <Route path="/reports" element={<ReportsPage />} />
            <Route path="/audit" element={<AuditPage />} />
            <Route path="/settings" element={<SettingsPage />} />
            <Route path="/security" element={<SecurityPage />} />
            {/* Milestone 11 — Operations */}
            <Route path="/roster" element={<RosterPage />} />
            <Route path="/rooms" element={<RoomsPage />} />
            <Route path="/leave" element={<LeavePage />} />
            <Route path="/invoices" element={<InvoicesPage />} />
            <Route path="/inventory" element={<InventoryPage />} />
            <Route path="/budget" element={<BudgetPage />} />
            <Route path="/departments" element={<DepartmentsPage />} />
            <Route path="/roles" element={<RolesPage />} />
            {/* Milestone 11 — Integrations */}
            <Route path="/integrations" element={<IntegrationsPage />} />
            {/* Milestone 12 — ACC provider registration */}
            <Route path="/acc-provider" element={<ACCProviderPage />} />
          </Route>

          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  );
}
