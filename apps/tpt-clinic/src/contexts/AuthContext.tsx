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
}

const AuthContext = createContext<AuthContextValue | null>(null);

/**
 * JWT access token is intentionally stored only in module-level memory —
 * never in localStorage or sessionStorage — to reduce XSS token-theft risk.
 * On page reload the user will need to re-authenticate.
 */
let inMemoryToken: string | null = null;

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({
    user: null,
    isAuthenticated: false,
    isLoading: false,
  });

  const getToken = useCallback((): string | null => inMemoryToken, []);

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

      const data = (await response.json()) as {
        access_token: string;
        user: AuthUser;
      };

      inMemoryToken = data.access_token;

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
    setState({ user: null, isAuthenticated: false, isLoading: false });
    // Optionally revoke server-side session
    void fetch('/api/v1/auth/logout', { method: 'POST' }).catch(() => undefined);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({ ...state, login, logout, getToken }),
    [state, login, logout, getToken],
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
