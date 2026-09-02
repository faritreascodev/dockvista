import type { ContainerState } from "../types/domain";

const DOT_STYLES: Record<ContainerState, string> = {
  running: "bg-emerald-500",
  paused: "bg-amber-500",
  restarting: "bg-amber-500 animate-pulse",
  created: "bg-sky-500",
  removing: "bg-rose-500",
  exited: "bg-ink-faint",
  dead: "bg-rose-500",
};

const TEXT_STYLES: Record<ContainerState, string> = {
  running: "text-emerald-600 dark:text-emerald-400",
  paused: "text-amber-600 dark:text-amber-400",
  restarting: "text-amber-600 dark:text-amber-400",
  created: "text-sky-600 dark:text-sky-400",
  removing: "text-rose-600 dark:text-rose-400",
  exited: "text-ink-muted",
  dead: "text-rose-600 dark:text-rose-400",
};

export function StatusBadge({ state }: { state: ContainerState }) {
  return (
    <span className={`inline-flex items-center gap-1.5 text-xs font-medium capitalize ${TEXT_STYLES[state]}`}>
      <span className={`h-2 w-2 rounded-full ${DOT_STYLES[state]}`} />
      {state}
    </span>
  );
}
