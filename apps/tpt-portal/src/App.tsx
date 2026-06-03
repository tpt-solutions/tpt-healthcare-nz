import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import { ProtectedRoute } from './components/ProtectedRoute';
import { NavLayout } from './components/NavLayout';
import { LoginPage } from './pages/LoginPage';
import { DashboardPage } from './pages/DashboardPage';
import { AppointmentsPage } from './pages/AppointmentsPage';
import { RecordsPage } from './pages/RecordsPage';
import { PrescriptionsPage } from './pages/PrescriptionsPage';
import { MessagesPage } from './pages/MessagesPage';
import { ConsentPage } from './pages/ConsentPage';

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
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
            <Route path="/appointments" element={<AppointmentsPage />} />
            <Route path="/records" element={<RecordsPage />} />
            <Route path="/prescriptions" element={<PrescriptionsPage />} />
            <Route path="/messages" element={<MessagesPage />} />
            <Route path="/consent" element={<ConsentPage />} />
          </Route>

          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="*" element={<Navigate to="/dashboard" replace />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}
