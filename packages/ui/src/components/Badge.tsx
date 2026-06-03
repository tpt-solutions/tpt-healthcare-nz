import React from "react";

export type BadgeVariant = "success" | "warning" | "error" | "info" | "neutral";

export interface BadgeProps {
  variant?: BadgeVariant;
  children: React.ReactNode;
  className?: string;
}

const variantClasses: Record<BadgeVariant, string> = {
  success: "bg-green-100 text-green-800 ring-green-600/20",
  warning: "bg-yellow-100 text-yellow-800 ring-yellow-600/20",
  error: "bg-red-100 text-red-800 ring-red-600/20",
  info: "bg-teal-100 text-teal-800 ring-teal-600/20",
  neutral: "bg-gray-100 text-gray-700 ring-gray-600/20",
};

const dotClasses: Record<BadgeVariant, string> = {
  success: "bg-green-500",
  warning: "bg-yellow-500",
  error: "bg-red-500",
  info: "bg-teal-500",
  neutral: "bg-gray-400",
};

export function Badge({
  variant = "neutral",
  children,
  className = "",
}: BadgeProps) {
  return (
    <span
      className={[
        "inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5",
        "text-xs font-medium ring-1 ring-inset",
        variantClasses[variant],
        className,
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <span
        className={[
          "h-1.5 w-1.5 rounded-full flex-shrink-0",
          dotClasses[variant],
        ].join(" ")}
        aria-hidden="true"
      />
      {children}
    </span>
  );
}
