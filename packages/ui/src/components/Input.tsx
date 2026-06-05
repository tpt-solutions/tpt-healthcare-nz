import React, { useId } from "react";

export interface InputProps
  extends Omit<React.InputHTMLAttributes<HTMLInputElement>, "id"> {
  label: string;
  error?: string;
  hint?: string;
  required?: boolean;
}

export function Input({
  label,
  error,
  hint,
  required = false,
  className = "",
  ...props
}: InputProps) {
  const generatedId = useId();
  const inputId = `input-${generatedId}`;
  const errorId = error ? `error-${generatedId}` : undefined;
  const hintId = hint ? `hint-${generatedId}` : undefined;

  const describedBy = [hintId, errorId].filter(Boolean).join(" ") || undefined;

  return (
    <div className="flex flex-col gap-1">
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

      <input
        id={inputId}
        required={required}
        aria-required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={[
          "block w-full rounded-md border px-3 py-2 text-sm shadow-sm",
          "placeholder:text-secondary-400",
          "focus:outline-none focus:ring-2 focus:ring-offset-0",
          "disabled:cursor-not-allowed disabled:bg-secondary-50 disabled:text-secondary-500",
          error
            ? "border-red-400 focus:border-red-500 focus:ring-red-400"
            : "border-secondary-300 focus:border-primary-500 focus:ring-primary-500",
          className,
        ]
          .filter(Boolean)
          .join(" ")}
        {...props}
      />

      {error && (
        <p
          id={errorId}
          role="alert"
          className="flex items-center gap-1 text-xs text-red-600"
        >
          <svg
            className="h-3.5 w-3.5 flex-shrink-0"
            viewBox="0 0 20 20"
            fill="currentColor"
            aria-hidden="true"
          >
            <path
              fillRule="evenodd"
              d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0v-4.5A.75.75 0 0110 5zm0 10a1 1 0 100-2 1 1 0 000 2z"
              clipRule="evenodd"
            />
          </svg>
          {error}
        </p>
      )}
    </div>
  );
}
