import type { ReactNode } from "react";

export function FieldLabel({ htmlFor, children, hint }: { htmlFor?: string; children: ReactNode; hint?: ReactNode }) {
  return (
    <label htmlFor={htmlFor} className="mb-1.5 flex items-baseline justify-between gap-2 text-xs font-medium text-ink-muted">
      <span>{children}</span>
      {hint && <span className="font-normal text-ink-faint">{hint}</span>}
    </label>
  );
}

export function FormError({ children }: { children: ReactNode }) {
  return (
    <div role="alert" className="mt-3 rounded-md border border-bad/30 bg-bad/10 px-3 py-2 text-sm text-bad">
      {children}
    </div>
  );
}
