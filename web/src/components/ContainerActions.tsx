import { useState } from "react";
import { Pause, Play, RotateCw, Square } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { Container, ContainerAction } from "../types/domain";

interface ContainerActionsProps {
  container: Container;
  onAction: (id: string, action: ContainerAction) => Promise<void>;
  size?: "sm" | "md";
}

const ICONS: Record<ContainerAction, LucideIcon> = {
  start: Play,
  stop: Square,
  pause: Pause,
  unpause: Play,
  restart: RotateCw,
};

function actionsFor(container: Container): ContainerAction[] {
  switch (container.state) {
    case "running":
      return ["pause", "restart", "stop"];
    case "paused":
      return ["unpause", "stop"];
    case "exited":
    case "created":
    case "dead":
      return ["start"];
    default:
      return [];
  }
}

export function ContainerActions({ container, onAction, size = "sm" }: ContainerActionsProps) {
  const [pending, setPending] = useState<ContainerAction | null>(null);
  const dim = size === "sm" ? "h-3.5 w-3.5" : "h-4 w-4";
  const pad = size === "sm" ? "p-1.5" : "p-2";

  const run = async (action: ContainerAction) => {
    setPending(action);
    try {
      await onAction(container.id, action);
    } finally {
      setPending(null);
    }
  };

  return (
    <div className="flex items-center gap-1">
      {actionsFor(container).map((action) => {
        const Icon = ICONS[action];
        return (
          <button
            key={action}
            title={action}
            disabled={pending !== null}
            onClick={(e) => {
              e.stopPropagation();
              void run(action);
            }}
            className={`rounded-md border border-edge bg-panel-2 text-ink-muted transition hover:bg-accent/10 hover:text-accent disabled:opacity-40 ${pad}`}
          >
            <Icon className={`${dim} ${pending === action ? "animate-pulse" : ""}`} />
          </button>
        );
      })}
    </div>
  );
}
