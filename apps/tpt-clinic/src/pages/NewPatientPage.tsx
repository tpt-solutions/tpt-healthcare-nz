import React, { FormEvent, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import AppShell from '@/components/AppShell';
import { useApi } from '@/contexts/ApiContext';

// ---------------------------------------------------------------------------
// NHI validation (same logic as PatientListPage)
// ---------------------------------------------------------------------------

function charValue(c: string): number {
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
  const weights = [7, 6, 5, 4, 3, 2];
  let sum = 0;
  for (let i = 0; i < 6; i++) {
    const ch = nhi[i]!;
    const val = /[A-Z]/.test(ch) ? charValue(ch) : parseInt(ch, 10);
    sum += val * (weights[i] ?? 0);
  }
  const remainder = sum % 11;
  if (remainder === 0) return false;
  const checkDigit = 11 - remainder;
  if (checkDigit === 10) return false;
  return checkDigit === parseInt(nhi[6]!, 10);
}

function validateNewNhi(nhi: string): boolean {
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

function validateNHI(raw: string): boolean {
  const nhi = raw.trim().toUpperCase();
  const oldFormat = /^[A-Z]{3}\d{4}$/;
  const newFormat = /^[A-Z]{3}\d{2}[A-Z]{2}$/;
  if (!oldFormat.test(nhi) && !newFormat.test(nhi)) return false;
  if (oldFormat.test(nhi)) return validateOldNhi(nhi);
  return validateNewNhi(nhi);
}

// ---------------------------------------------------------------------------
// NZ ethnicities (HISO 10001.2)
// ---------------------------------------------------------------------------

const NZ_ETHNICITIES = [
  'New Zealand European',
  'Māori',
  'Samoan',
  'Cook Island Māori',
  'Tongan',
  'Niuean',
  'Chinese',
  'Indian',
  'Other Pacific Peoples',
  'Other Asian',
  'Middle Eastern',
  'Latin American',
  'African',
  'Other Ethnicity',
  'Prefer not to say',
];

// ---------------------------------------------------------------------------
// Form types
// ---------------------------------------------------------------------------

interface NewPatientForm {
  nhi: string;
  firstName: string;
  lastName: string;
  dateOfBirth: string;
  gender: string;
  ethnicity: string;
  phone: string;
  email: string;
  addressLine1: string;
  addressLine2: string;
  suburb: string;
  city: string;
  postcode: string;
  emergencyContactName: string;
  emergencyContactPhone: string;
  emergencyContactRelationship: string;
}

const EMPTY_FORM: NewPatientForm = {
  nhi: '',
  firstName: '',
  lastName: '',
  dateOfBirth: '',
  gender: '',
  ethnicity: '',
  phone: '',
  email: '',
  addressLine1: '',
  addressLine2: '',
  suburb: '',
  city: '',
  postcode: '',
  emergencyContactName: '',
  emergencyContactPhone: '',
  emergencyContactRelationship: '',
};

interface FieldProps {
  label: string;
  htmlFor: string;
  required?: boolean;
  error?: string;
  children: React.ReactNode;
}

function Field({ label, htmlFor, required, error, children }: FieldProps) {
  return (
    <div>
      <label htmlFor={htmlFor} className="block text-sm font-medium text-secondary-700">
        {label}
        {required && <span className="ml-0.5 text-red-500">*</span>}
      </label>
      {children}
      {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
    </div>
  );
}

const inputClass =
  'mt-1 block w-full rounded-md border border-secondary-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500';

const inputErrorClass =
  'mt-1 block w-full rounded-md border border-red-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-red-500 focus:outline-none focus:ring-1 focus:ring-red-500';

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function NewPatientPage() {
  const api = useApi();
  const navigate = useNavigate();

  const [form, setForm] = useState<NewPatientForm>(EMPTY_FORM);
  const [nhiError, setNhiError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);

  function update<K extends keyof NewPatientForm>(k: K, v: NewPatientForm[K]) {
    setForm((prev) => ({ ...prev, [k]: v }));
  }

  function handleNhiChange(value: string) {
    const upper = value.toUpperCase();
    update('nhi', upper);
    if (upper && !validateNHI(upper)) {
      setNhiError('Invalid NHI format. Old format: ABC1234, new format: ABC12DE');
    } else {
      setNhiError(null);
    }
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (form.nhi && !validateNHI(form.nhi)) return;

    setSaving(true);
    setServerError(null);
    try {
      const result = await api.post<{ id: string }>('/patients', form);
      void navigate(`/patients/${result.id}`, { replace: true });
    } catch (err) {
      setServerError(err instanceof Error ? err.message : 'Failed to register patient.');
    } finally {
      setSaving(false);
    }
  }

  return (
    <AppShell title="Register New Patient">
      <div className="mx-auto max-w-2xl">
        <form onSubmit={(e) => void handleSubmit(e)} className="space-y-8" noValidate>
          {serverError && (
            <div className="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700 ring-1 ring-red-200">
              {serverError}
            </div>
          )}

          {/* Section: Identity */}
          <section className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="mb-5 text-sm font-semibold uppercase tracking-wide text-secondary-500">
              Patient Identity
            </h2>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="NHI Number" htmlFor="nhi" error={nhiError ?? undefined}>
                <input
                  id="nhi"
                  type="text"
                  value={form.nhi}
                  onChange={(e) => handleNhiChange(e.target.value)}
                  placeholder="e.g. ABC1234"
                  maxLength={7}
                  className={nhiError ? inputErrorClass : inputClass + ' font-mono uppercase'}
                />
                <p className="mt-1 text-xs text-secondary-400">
                  Leave blank to have NHI assigned by the Ministry.
                </p>
              </Field>

              <div className="sm:col-span-2 grid grid-cols-2 gap-4">
                <Field label="First name" htmlFor="firstName" required>
                  <input
                    id="firstName"
                    type="text"
                    required
                    value={form.firstName}
                    onChange={(e) => update('firstName', e.target.value)}
                    className={inputClass}
                  />
                </Field>
                <Field label="Last name" htmlFor="lastName" required>
                  <input
                    id="lastName"
                    type="text"
                    required
                    value={form.lastName}
                    onChange={(e) => update('lastName', e.target.value)}
                    className={inputClass}
                  />
                </Field>
              </div>

              <Field label="Date of birth" htmlFor="dob" required>
                <input
                  id="dob"
                  type="date"
                  required
                  value={form.dateOfBirth}
                  onChange={(e) => update('dateOfBirth', e.target.value)}
                  max={new Date().toISOString().slice(0, 10)}
                  className={inputClass}
                />
              </Field>

              <Field label="Gender" htmlFor="gender" required>
                <select
                  id="gender"
                  required
                  value={form.gender}
                  onChange={(e) => update('gender', e.target.value)}
                  className={inputClass}
                >
                  <option value="" disabled>Select…</option>
                  <option value="male">Male</option>
                  <option value="female">Female</option>
                  <option value="other">Other / Non-binary</option>
                  <option value="unknown">Unknown / Prefer not to say</option>
                </select>
              </Field>

              <Field label="Ethnicity" htmlFor="ethnicity">
                <select
                  id="ethnicity"
                  value={form.ethnicity}
                  onChange={(e) => update('ethnicity', e.target.value)}
                  className={inputClass}
                >
                  <option value="">Select…</option>
                  {NZ_ETHNICITIES.map((eth) => (
                    <option key={eth} value={eth}>{eth}</option>
                  ))}
                </select>
              </Field>
            </div>
          </section>

          {/* Section: Contact */}
          <section className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="mb-5 text-sm font-semibold uppercase tracking-wide text-secondary-500">
              Contact Information
            </h2>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Phone" htmlFor="phone">
                <input
                  id="phone"
                  type="tel"
                  value={form.phone}
                  onChange={(e) => update('phone', e.target.value)}
                  placeholder="+64 21 000 0000"
                  className={inputClass}
                />
              </Field>

              <Field label="Email" htmlFor="email">
                <input
                  id="email"
                  type="email"
                  value={form.email}
                  onChange={(e) => update('email', e.target.value)}
                  placeholder="patient@example.com"
                  className={inputClass}
                />
              </Field>

              <div className="sm:col-span-2">
                <Field label="Address line 1" htmlFor="addr1">
                  <input
                    id="addr1"
                    type="text"
                    value={form.addressLine1}
                    onChange={(e) => update('addressLine1', e.target.value)}
                    placeholder="Street number and name"
                    className={inputClass}
                  />
                </Field>
              </div>

              <div className="sm:col-span-2">
                <Field label="Address line 2" htmlFor="addr2">
                  <input
                    id="addr2"
                    type="text"
                    value={form.addressLine2}
                    onChange={(e) => update('addressLine2', e.target.value)}
                    placeholder="Apartment, unit, suite…"
                    className={inputClass}
                  />
                </Field>
              </div>

              <Field label="Suburb" htmlFor="suburb">
                <input
                  id="suburb"
                  type="text"
                  value={form.suburb}
                  onChange={(e) => update('suburb', e.target.value)}
                  className={inputClass}
                />
              </Field>

              <Field label="City" htmlFor="city">
                <input
                  id="city"
                  type="text"
                  value={form.city}
                  onChange={(e) => update('city', e.target.value)}
                  className={inputClass}
                />
              </Field>

              <Field label="Postcode" htmlFor="postcode">
                <input
                  id="postcode"
                  type="text"
                  value={form.postcode}
                  onChange={(e) => update('postcode', e.target.value)}
                  maxLength={4}
                  placeholder="e.g. 1010"
                  className={inputClass + ' font-mono'}
                />
              </Field>
            </div>
          </section>

          {/* Section: Emergency contact */}
          <section className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-secondary-200">
            <h2 className="mb-5 text-sm font-semibold uppercase tracking-wide text-secondary-500">
              Emergency Contact
            </h2>
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field label="Full name" htmlFor="ec-name">
                <input
                  id="ec-name"
                  type="text"
                  value={form.emergencyContactName}
                  onChange={(e) => update('emergencyContactName', e.target.value)}
                  className={inputClass}
                />
              </Field>

              <Field label="Relationship" htmlFor="ec-rel">
                <select
                  id="ec-rel"
                  value={form.emergencyContactRelationship}
                  onChange={(e) => update('emergencyContactRelationship', e.target.value)}
                  className={inputClass}
                >
                  <option value="">Select…</option>
                  <option value="spouse">Spouse / Partner</option>
                  <option value="parent">Parent</option>
                  <option value="child">Child</option>
                  <option value="sibling">Sibling</option>
                  <option value="friend">Friend</option>
                  <option value="caregiver">Caregiver</option>
                  <option value="other">Other</option>
                </select>
              </Field>

              <Field label="Phone" htmlFor="ec-phone">
                <input
                  id="ec-phone"
                  type="tel"
                  value={form.emergencyContactPhone}
                  onChange={(e) => update('emergencyContactPhone', e.target.value)}
                  placeholder="+64 21 000 0000"
                  className={inputClass}
                />
              </Field>
            </div>
          </section>

          {/* Actions */}
          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={() => void navigate(-1)}
              className="rounded-md px-5 py-2 text-sm font-medium text-secondary-700 hover:bg-secondary-100"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving || !!nhiError || !form.firstName || !form.lastName || !form.dateOfBirth || !form.gender}
              className="rounded-md bg-primary-600 px-5 py-2 text-sm font-semibold text-white hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {saving ? 'Registering…' : 'Register Patient'}
            </button>
          </div>
        </form>
      </div>
    </AppShell>
  );
}
