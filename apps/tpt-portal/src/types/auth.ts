export interface PatientUser {
  id: string;
  nhi: string; // Encrypted/masked for display: e.g. "ZZZ****"
  givenName: string;
  familyName: string;
  dateOfBirth: string;
  email: string;
  phone?: string;
}

export interface AuthState {
  user: PatientUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}
