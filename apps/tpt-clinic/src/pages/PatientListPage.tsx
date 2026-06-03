import React, { FormEvent, useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// NHI validation
// Inline implementation matching @tpt/nz-codes validateNHI contract.
// The old NHI format is [A-Z]{3}[0-9]{4} with a modulo-11 check digit.
// The new Luhn-based format is [A-Z]{3}[0-9]{2}[A-Z]{2}.
// ---------------------------------------------------------------------------

/**
 * Validate an NZ National Health Index number.
 * Returns true if the format and checksum are valid.
 * Mirrors the @tpt/nz-codes validateNHI export.
 */
function validateNHI(raw: string): boolean {
  const nhi = raw.trim().toUpperCase();

  // Old format: 3 alpha + 4 numeric (last digit is check digit)
  const oldFormat = /^[A-Z]{3}\d{4}$/;
  // New Luhn format: 3 alpha + 2 numeric + 2 alpha
  const newFormat = /^[A-Z]{3}\d{2}[A-Z]{2}$/;

  if (!oldFormat.test(nhi) && !newFormat.test(nhi)) return false;

  if (oldFormat.test(nhi)) {
    return validateOldNhi(nhi);
  }
  return validateNewNhi(nhi);
}

function charValue(c: string): number {
  // A=1, B=2, …, Z=24 (I and O are excluded — shifted)
  const excluded = new Set(['I', 'O']);
  let val = 0;
  for (let code = 65; code <= 90; code++) {
    const ch = String.fromCharCode(code);
    if (excluded.has(ch)) continue;
    val++;
    if (ch === c) return val;
  }
  return 0;
}

function validateOldNhi(nhi: string): boolean {
  // Weights: positions 1-6 are multiplied by 7,6,5,4,3,2
  const weights = [7, 6, 5, 4, 3, 2];
  let sum = 0;
  for (let i = 0; i < 6; i++) {
    const ch = nhi[i]!;
    const val = /[A-Z]/.test(ch) ? charValue(ch) : parseInt(ch, 10);
    sum += val * (weights[i] ?? 0);
  }
  const remainder = sum % 11;
  if (remainder === 0) return false; // invalid
  const checkDigit = 11 - remainder;
  if (checkDigit === 10) return false; // would need two digits
  return checkDigit === parseInt(nhi[6]!, 10);
}

function validateNewNhi(nhi: string): boolean {
  // New format uses Luhn algorithm over all 7 characters
  // Each char mapped: digits as-is, alpha via charValue
  const chars = nhi.split('');
  let sum = 0;
  for (let i = 0; i < 7; i++) {
    const ch = chars[i]!;
    let val = /[A-Z]/.test(ch) ? charValue(ch) : parseInt(ch, 10);
    if (i % 2 === 0) {
      val *= 2;
      if (val > 9) val -= 9;
    }
    sum += val;
  }
  return sum % 10 === 0;
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface PatientRow {
  id: string;
  nhi: string;
  nhiDisplay: string;
  name: string;
  dateOfBirth: string;
  gender: string;
  address: string;
  enrolledPractice: string;
}

interface PatientSearchResult {
  patients: PatientRow[];
  total: number;
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function PatientListPage() {
  const api = useApi();

  const [query, setQuery] = useState('');
  const [nhiQuery, setNhiQuery] = useState('');
  const [nhiError, setNhiError] = useState<string | null>(null);
  const [results, setResults] = useState<PatientRow[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searched, setSearched] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  // Initial load — fetch all enrolled patients
  useEffect(() => {
    void runSearch('', '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function runSearch(nameQ: string, nhiQ: string) {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setLoading(true);
    setError(null);
    try {
      const params: Record<string, string> = {};
      if (nameQ) params['name'] = nameQ;
      if (nhiQ) params['nhi'] = nhiQ;

      const data = await api.get<PatientSearchResult>('/patients', { params });
      setResults(data.patients);
      setTotal(data.total);
      setSearched(true);
    } catch (err: unknown) {
      if (err instanceof Error && err.name === 'AbortError') return;
      setError('Failed to search patients.');
    } finally {
      setLoading(false);
    }
  }

  function handleNhiChange(value: string) {
    setNhiQuery(value);
    if (value && !validateNHI(value)) {
      setNhiError('Invalid NHI format (e.g. ABC1234 or ABC12DE)');
    } else {
      setNhiError(null);
    }
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (nhiQuery && !validateNHI(nhiQuery)) return;
    void runSearch(query, nhiQuery);
  }

  return (
    <AppShell title="Patients">
      {/* Search bar */}
      <form onSubmit={handleSubmit} className="mb-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
          {/* Name search */}
          <div className="flex-1">
            <label htmlFor="name-search" className="block text-sm font-medium text-secondary-700">
              Search by name
            </label>
            <input
              id="name-search"
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Patient name…"
              className="mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"
            />
          </div>

          {/* NHI search */}
          <div className="w-full sm:w-48">
            <label htmlFor="nhi-search" className="block text-sm font-medium text-secondary-700">
              Search by NHI
            </label>
            <input
              id="nhi-search"
              type="text"
              value={nhiQuery}
              onChange={(e) => handleNhiChange(e.target.value.toUpperCase())}
              placeholder="e.g. ABC1234"
              maxLength={7}
              className={[
                'mt-1 block w-full rounded-md border bg-white px-3 py-2 font-mono text-sm uppercase shadow-sm focus:outline-none focus:ring-1',
                nhiError
                  ? 'border-red-300 focus:border-red-500 focus:ring-red-500'
                  : 'border-secondary-300 focus:border-primary-500 focus:ring-primary-500',
              ].join(' ')}
            />
            {nhiError && (
              <p className="mt-1 text-xs text-red-600">{nhiError}</p>
            )}
          </div>

          <button
            type="submit"
            disabled={loading || (!!nhiQuery && !!nhiError)}
            className="rounded-md bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 disabled:opacity-50 sm:self-end"
          >
            {loading ? 'Searching…' : 'Search'}
          </button>

          <Link
            to="/patients/new"
            className="flex items-center gap-1.5 rounded-md bg-secondary-700 px-4 py-2 text-sm font-medium text-white hover:bg-secondary-800 sm:self-end"
          >
            <svg className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
            </svg>
            New Patient
          </Link>
        </div>
      </form>

      {/* Error */}
      {error && (
        <div className="mb-4 rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
          {error}
        </div>
      )}

      {/* Results */}
      {searched && (
        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-secondary-200">
          <div className="border-b border-secondary-100 px-4 py-3">
            <p className="text-sm text-secondary-500">
              {loading ? 'Loading…' : `${total.toLocaleString()} patient${total !== 1 ? 's' : ''} found`}
            </p>
          </div>

          {results.length === 0 && !loading ? (
            <p className="px-4 py-8 text-center text-sm text-secondary-500">
              No patients found. Try a different search or{' '}
              <Link to="/patients/new" className="font-medium text-primary-600 hover:underline">
                register a new patient
              </Link>
              .
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="bg-secondary-50 text-xs font-medium uppercase tracking-wide text-secondary-500">
                  <tr>
                    <th className="px-4 py-3 text-left">Name</th>
                    <th className="px-4 py-3 text-left">NHI</th>
                    <th className="px-4 py-3 text-left">Date of Birth</th>
                    <th className="px-4 py-3 text-left">Gender</th>
                    <th className="px-4 py-3 text-left">Address</th>
                    <th className="px-4 py-3 text-left">Enrolled Practice</th>
                    <th className="px-4 py-3" />
                  </tr>
                </thead>
                <tbody className="divide-y divide-secondary-100">
                  {results.map((patient) => (
                    <tr key={patient.id} className="hover:bg-secondary-50">
                      <td className="px-4 py-3 font-medium text-secondary-900">{patient.name}</td>
                      <td className="px-4 py-3 font-mono text-secondary-600">{patient.nhiDisplay}</td>
                      <td className="px-4 py-3 text-secondary-600">
                        {new Date(patient.dateOfBirth).toLocaleDateString('en-NZ')}
                      </td>
                      <td className="px-4 py-3 text-secondary-600">{patient.gender}</td>
                      <td className="px-4 py-3 text-secondary-600">{patient.address}</td>
                      <td className="px-4 py-3 text-secondary-600">{patient.enrolledPractice}</td>
                      <td className="px-4 py-3">
                        <Link
                          to={`/patients/${patient.id}`}
                          className="font-medium text-primary-600 hover:text-primary-700 hover:underline"
                        >
                          View
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </AppShell>
  );
}
