import type { ContainerState } from "../types/domain";

const STYLES: Record<ContainerState, { text: string; dot: string }> = {
  running: { text: "text-ok", dot: "bg-ok live-dot" },
  paused: { text: "text-warn", dot: "bg-warn" },
  restarting: { text: "text-warn", dot: "bg-warn animate-pulse" },
  created: { text: "text-info", dot: "bg-info" },
  removing: { text: "text-bad", dot: "bg-bad animate-pulse" },
  exited: { text: "text-ink-faint", dot: "bg-ink-faint" },
  dead: { text: "text-bad", dot: "bg-bad" },
};

export function StatusBadge({ state }: { state: ContainerState }) {
  const style = STYLES[state];
  return (
    <span className={`inline-flex items-center gap-2 font-mono text-[11px] font-medium uppercase tracking-wider ${style.text}`}>
      <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${style.dot}`} />
      {state}
    </span>
  );
}
