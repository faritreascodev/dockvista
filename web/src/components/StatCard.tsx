import type { ReactNode } from "react";
import { Sparkline } from "./Sparkline";

export type Tone = "accent" | "ok" | "warn" | "bad" | "info" | "muted";

const TONE_TEXT: Record<Tone, string> = {
  accent: "text-accent",
  ok: "text-ok",
  warn: "text-warn",
  bad: "text-bad",
  info: "text-info",
  muted: "text-ink-faint",
};

interface StatCardProps {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  tone?: Tone;
  /** Optional history rendered under the value. */
  series?: number[];
  seriesMax?: number;
}

/** A readout tile: caps label, large mono value, optional trend line. */
export function StatCard({ label, value, hint, tone = "muted", series, seriesMax }: StatCardProps) {
  return (
    <div className="relative overflow-hidden rounded-lg border border-edge bg-panel px-4 pb-3 pt-3.5">
      <div className="flex items-center gap-2">
        <span className={`h-1.5 w-1.5 rounded-[1px] bg-current ${TONE_TEXT[tone]}`} />
        <p className="label-caps">{label}</p>
      </div>
      <p className="mt-2 font-mono text-2xl font-medium tabular-nums tracking-tight text-ink">{value}</p>
      {hint && <p className="mt-0.5 truncate text-xs text-ink-faint">{hint}</p>}
      {series && (
        <Sparkline values={series} max={seriesMax} tone={TONE_TEXT[tone]} className="-mx-4 -mb-3 mt-2 block h-9 w-[calc(100%+2rem)]" />
      )}
    </div>
  );
}
