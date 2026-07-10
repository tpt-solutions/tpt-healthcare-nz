export interface PatientBannerProps {
  name: string;
  nhi: string;
  dob: string;
  gender: string;
  address?: string;
  gpName?: string;
  className?: string;
}

function BannerField({
  label,
  value,
}: {
  label: string;
  value: string | undefined;
}) {
  if (!value) return null;
  return (
    <div className="flex flex-col">
      <span className="text-xs font-medium uppercase tracking-wide text-primary-200">
        {label}
      </span>
      <span className="text-sm font-medium text-white">{value}</span>
    </div>
  );
}

export function PatientBanner({
  name,
  nhi,
  dob,
  gender,
  address,
  gpName,
  className = "",
}: PatientBannerProps) {
  return (
    <div
      role="region"
      aria-label="Patient identification banner"
      className={[
        "flex flex-wrap items-center gap-x-8 gap-y-3",
        "bg-primary-700 px-5 py-3 shadow-md",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      {/* Patient name + NHI — most prominent */}
      <div className="flex flex-col">
        <span className="text-lg font-bold text-white leading-tight">
          {name}
        </span>
        <span
          className="text-sm font-semibold text-primary-100 tracking-wider"
          aria-label={`National Health Index: ${nhi}`}
        >
          NHI: {nhi}
        </span>
      </div>

      {/* Divider */}
      <div
        className="hidden h-10 w-px bg-primary-500 sm:block"
        aria-hidden="true"
      />

      <BannerField label="Date of Birth" value={dob} />
      <BannerField label="Gender" value={gender} />
      {address && <BannerField label="Address" value={address} />}
      {gpName && <BannerField label="GP / Usual Doctor" value={gpName} />}

      {/* Sensitive record indicator */}
      <div className="ml-auto flex items-center gap-1.5 rounded bg-primary-800 px-2 py-1 text-xs text-primary-200">
        <svg
          className="h-3.5 w-3.5"
          viewBox="0 0 20 20"
          fill="currentColor"
          aria-hidden="true"
        >
          <path
            fillRule="evenodd"
            d="M10 1a4.5 4.5 0 00-4.5 4.5V9H5a2 2 0 00-2 2v6a2 2 0 002 2h10a2 2 0 002-2v-6a2 2 0 00-2-2h-.5V5.5A4.5 4.5 0 0010 1zm3 8V5.5a3 3 0 10-6 0V9h6z"
            clipRule="evenodd"
          />
        </svg>
        Health record
      </div>
    </div>
  );
}
