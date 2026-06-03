import React, { createContext, useContext, useState, useEffect } from 'react';
import type { AuthState, PatientUser } from '../types/auth';

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

// Stub patient for development. In production this comes from the OIDC token
// issued by tpt-identity after the patient authenticates.
const STUB_PATIENT: PatientUser = {
  id: 'patient-001',
  nhi: 'ZZZ0032',
  givenName: 'Aroha',
  familyName: 'Ngata',
  dateOfBirth: '1985-04-12',
  email: 'aroha.ngata@example.nz',
  phone: '021 555 0100',
};

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    isAuthenticated: false,
    isLoading: true,
  });

  useEffect(() => {
    // Check for existing session token in sessionStorage
    const token = sessionStorage.getItem('portal_token');
    if (token) {
      setState({ user: STUB_PATIENT, isAuthenticated: true, isLoading: false });
    } else {
      setState(prev => ({ ...prev, isLoading: false }));
    }
  }, []);

  const login = async (email: string, _password: string) => {
    // TODO: call POST /api/v1/auth/token with credentials
    // For now use stub authentication
    if (!email) throw new Error('Email is required');
    sessionStorage.setItem('portal_token', 'stub-token');
    setState({ user: STUB_PATIENT, isAuthenticated: true, isLoading: false });
  };

  const logout = () => {
    sessionStorage.removeItem('portal_token');
    setState({ user: null, isAuthenticated: false, isLoading: false });
  };

  return (
    <AuthContext.Provider value={{ ...state, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
