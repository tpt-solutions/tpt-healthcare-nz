export interface AdminUser {
  id: string;
  givenName: string;
  familyName: string;
  email: string;
  role: 'practice_manager' | 'billing_admin' | 'super_admin';
  practiceName: string;
}

export interface AuthState {
  user: AdminUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}
