import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "danger" | "ghost";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  loading?: boolean;
}

const VARIANT_CLASSES: Record<Variant, string> = {
  primary: "bg-accent text-accent-ink hover:opacity-90 disabled:hover:opacity-100",
  secondary: "bg-panel-2 text-ink hover:bg-edge disabled:hover:bg-panel-2",
  danger:
    "bg-rose-600 text-white hover:bg-rose-500 disabled:hover:bg-rose-600 dark:bg-rose-600 dark:hover:bg-rose-500",
  ghost: "bg-transparent text-ink-muted hover:bg-panel-2 disabled:hover:bg-transparent",
};

export function Button({
  variant = "secondary",
  loading = false,
  disabled,
  className = "",
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      disabled={disabled || loading}
      className={`inline-flex items-center justify-center gap-1.5 rounded-lg px-3 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60 ${VARIANT_CLASSES[variant]} ${className}`}
      {...rest}
    >
      {loading && (
        <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" />
      )}
      {children}
    </button>
  );
}
