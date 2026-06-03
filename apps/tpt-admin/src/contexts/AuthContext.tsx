import React, { createContext, useContext, useState, useEffect } from 'react';
import type { AuthState, AdminUser } from '../types/auth';

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

const STUB_ADMIN: AdminUser = {
  id: 'admin-001',
  givenName: 'Tama',
  familyName: 'Parata',
  email: 'tama.parata@aucklandcitymedical.nz',
  role: 'practice_manager',
  practiceName: 'Auckland City Medical Centre',
};

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    isAuthenticated: false,
    isLoading: true,
  });

  useEffect(() => {
    const token = sessionStorage.getItem('admin_token');
    if (token) {
      setState({ user: STUB_ADMIN, isAuthenticated: true, isLoading: false });
    } else {
      setState(prev => ({ ...prev, isLoading: false }));
    }
  }, []);

  const login = async (email: string, _password: string) => {
    if (!email) throw new Error('Email is required');
    // TODO: call POST /api/v1/auth/token with admin credentials
    sessionStorage.setItem('admin_token', 'stub-admin-token');
    setState({ user: STUB_ADMIN, isAuthenticated: true, isLoading: false });
  };

  const logout = () => {
    sessionStorage.removeItem('admin_token');
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
