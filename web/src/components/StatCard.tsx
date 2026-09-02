import type { LucideIcon } from "lucide-react";

interface StatCardProps {
  label: string;
  value: string;
  icon: LucideIcon;
  accent?: "default" | "emerald" | "sky" | "amber" | "rose";
}

const ACCENTS: Record<NonNullable<StatCardProps["accent"]>, string> = {
  default: "bg-panel-2 text-ink-muted",
  emerald: "bg-emerald-500/10 text-emerald-500 dark:text-emerald-400",
  sky: "bg-sky-500/10 text-sky-500 dark:text-sky-400",
  amber: "bg-amber-500/10 text-amber-500 dark:text-amber-400",
  rose: "bg-rose-500/10 text-rose-500 dark:text-rose-400",
};

export function StatCard({ label, value, icon: Icon, accent = "default" }: StatCardProps) {
  return (
    <div className="flex items-center gap-3 rounded-xl border border-edge bg-panel p-4">
      <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${ACCENTS[accent]}`}>
        <Icon className="h-5 w-5" />
      </div>
      <div>
        <p className="text-xs font-medium text-ink-muted">{label}</p>
        <p className="mt-0.5 text-xl font-semibold tabular-nums text-ink">{value}</p>
      </div>
    </div>
  );
}
