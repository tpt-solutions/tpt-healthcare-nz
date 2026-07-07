import React, {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from 'react';

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  /** HPI CPN (Common Person Number) for the authenticated practitioner */
  hpiCpn?: string;
  /** Roles granted to this practitioner (e.g. "gp", "nurse", "admin") */
  roles: string[];
}

interface LoginResponse {
  access_token: string;
  user: AuthUser;
  /** Tenant UUID for the authenticated practice, sent as X-Tenant-ID on API calls */
  tenant_id?: string;
}

interface AuthState {
  user: AuthUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

interface AuthContextValue extends AuthState {
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  /** Returns the current in-memory access token, or null if not authenticated */
  getToken: () => string | null;
  /** Returns the current in-memory tenant ID, or null if not authenticated */
  getTenantId: () => string | null;
}

const AuthContext = createContext<AuthContextValue | null>(null);

/**
 * JWT access token is intentionally stored only in module-level memory —
 * never in localStorage or sessionStorage — to reduce XSS token-theft risk.
 * On page reload the user will need to re-authenticate.
 */
let inMemoryToken: string | null = null;
let inMemoryTenantId: string | null = null;

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    isAuthenticated: false,
    isLoading: false,
  });

  const getToken = useCallback((): string | null => inMemoryToken, []);
  const getTenantId = useCallback((): string | null => inMemoryTenantId, []);

  const login = useCallback(async (email: string, password: string): Promise<void> => {
    setState((prev) => ({ ...prev, isLoading: true }));
    try {
      const response = await fetch('/api/v1/auth/token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      if (!response.ok) {
        const body = (await response.json().catch(() => ({}))) as { message?: string };
        throw new Error(body.message ?? 'Invalid credentials');
      }

      const data = (await response.json()) as LoginResponse;

      inMemoryToken = data.access_token;
      inMemoryTenantId = data.tenant_id ?? null;

      setState({
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (err) {
      setState({ user: null, isAuthenticated: false, isLoading: false });
      throw err;
    }
  }, []);

  const logout = useCallback(() => {
    inMemoryToken = null;
    inMemoryTenantId = null;
    setState({ user: null, isAuthenticated: false, isLoading: false });
    // Optionally revoke server-side session
    void fetch('/api/v1/auth/logout', { method: 'POST' }).catch(() => undefined);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ ...state, login, logout, getToken, getTenantId }),
    [state, login, logout, getToken, getTenantId],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an <AuthProvider>');
  }
  return ctx;
}
