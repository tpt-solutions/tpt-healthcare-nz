import React, { useState, useId } from "react";
import { validateNHI } from "@tpt/nz-codes";

export interface NHIInputProps {
  label?: string;
  value: string;
  onChange: (value: string) => void;
  required?: boolean;
  disabled?: boolean;
  hint?: string;
  className?: string;
}

type ValidationState = "empty" | "valid" | "invalid";

function getValidationState(value: string): ValidationState {
  if (!value || value.trim() === "") return "empty";
  return validateNHI(value.trim().toUpperCase()) ? "valid" : "invalid";
}

const errorMessages: Record<string, string> = {
  invalid:
    "Invalid NHI format. Expected format: ABC1234 (old) or ABC12DE (new Luhn).",
};

export function NHIInput({
  label = "NHI Number",
  value,
  onChange,
  required = false,
  disabled = false,
  hint = "National Health Index number — e.g. ABC1234 or ABC12DE",
  className = "",
}: NHIInputProps) {
  const [touched, setTouched] = useState(false);
  const generatedId = useId();
  const inputId = `nhi-${generatedId}`;
  const hintId = `nhi-hint-${generatedId}`;
  const errorId = `nhi-error-${generatedId}`;

  const state = getValidationState(value);
  const showError = touched && state === "invalid";
  const showValid = state === "valid";

  const describedBy = [hintId, showError ? errorId : undefined]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={`flex flex-col gap-1 ${className}`}>
      <label
        htmlFor={inputId}
        className="block text-sm font-medium text-secondary-700"
      >
        {label}
        {required && (
          <span className="ml-1 text-red-500" aria-hidden="true">
            *
          </span>
        )}
      </label>

      {hint && (
        <p id={hintId} className="text-xs text-secondary-500">
          {hint}
        </p>
      )}

      <div className="relative">
        <input
          id={inputId}
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onBlur={() => setTouched(true)}
          disabled={disabled}
          required={required}
          aria-required={required}
          aria-invalid={showError ? true : undefined}
          aria-describedby={describedBy || undefined}
          placeholder="e.g. ABC1234"
          maxLength={10}
          className={[
            "block w-full rounded-md border px-3 py-2 pr-9 text-sm shadow-sm uppercase",
            "placeholder:text-secondary-400 placeholder:normal-case",
            "focus:outline-none focus:ring-2 focus:ring-offset-0",
            "disabled:cursor-not-allowed disabled:bg-secondary-50 disabled:text-secondary-500",
            showError
              ? "border-red-400 focus:border-red-500 focus:ring-red-400"
              : showValid
              ? "border-green-500 focus:border-green-500 focus:ring-green-400"
              : "border-secondary-300 focus:border-primary-500 focus:ring-primary-500",
          ]
            .filter(Boolean)
            .join(" ")}
        />

        {/* Validation indicator icon */}
        {showValid && (
          <span
            className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-green-600"
            aria-hidden="true"
          >
            <svg
              className="h-4 w-4"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path
                fillRule="evenodd"
                d="M16.704 4.153a.75.75 0 01.143 1.052l-8 10.5a.75.75 0 01-1.127.075l-4.5-4.5a.75.75 0 011.06-1.06l3.894 3.893 7.48-9.817a.75.75 0 011.05-.143z"
                clipRule="evenodd"
              />
            </svg>
          </span>
        )}

        {showError && (
          <span
            className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-red-500"
            aria-hidden="true"
          >
            <svg
              className="h-4 w-4"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path
                fillRule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0v-4.5A.75.75 0 0110 5zm0 10a1 1 0 100-2 1 1 0 000 2z"
                clipRule="evenodd"
              />
            </svg>
          </span>
        )}
      </div>

      {showError && (
        <p
          id={errorId}
          role="alert"
          className="flex items-center gap-1 text-xs text-red-600"
        >
          {errorMessages.invalid}
        </p>
      )}

      {showValid && (
        <p className="text-xs text-green-600" aria-live="polite">
          Valid NHI number.
        </p>
      )}
    </div>
  );
}
