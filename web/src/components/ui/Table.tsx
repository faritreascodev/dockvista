import type { ReactNode } from "react";
import { Trash2 } from "lucide-react";

export function TableFrame({ children }: { children: ReactNode }) {
  return <div className="overflow-x-auto rounded-lg border border-edge bg-panel">{children}</div>;
}

export function DataTable({ columns, children }: { columns: string[]; children: ReactNode }) {
  return (
    <table className="w-full min-w-[720px] border-collapse text-left">
      <thead>
        <tr className="border-b border-edge bg-panel-2/60">
          {columns.map((col, i) => (
            <th
              key={i}
              className={`label-caps py-2.5 pr-4 font-medium ${i === 0 ? "pl-4" : ""} ${i === columns.length - 1 ? "text-right" : ""}`}
            >
              {col}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className="divide-y divide-edge text-sm">{children}</tbody>
    </table>
  );
}

export function Chip({ children, tone = "muted" }: { children: ReactNode; tone?: "muted" | "accent" | "ok" | "warn" }) {
  const tones = {
    muted: "border-edge bg-panel-2 text-ink-muted",
    accent: "border-accent/30 bg-accent/10 text-accent",
    ok: "border-ok/30 bg-ok/10 text-ok",
    warn: "border-warn/30 bg-warn/10 text-warn",
  };
  return (
    <span className={`inline-flex items-center rounded border px-1.5 py-px font-mono text-[10.5px] ${tones[tone]}`}>{children}</span>
  );
}

export function RemoveButton({
  onClick,
  title,
  disabled,
}: {
  onClick: () => void;
  title: string;
  disabled?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      title={title}
      aria-label={title}
      disabled={disabled}
      className="inline-flex h-7 w-7 items-center justify-center rounded-md text-ink-faint transition hover:bg-bad/10 hover:text-bad disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-ink-faint"
    >
      <Trash2 className="h-3.5 w-3.5" />
    </button>
  );
}
