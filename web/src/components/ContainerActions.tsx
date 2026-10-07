import { useState } from "react";
import { Pause, Play, RotateCw, Square } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useSession } from "../hooks/useSession";
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

const HOVER: Record<ContainerAction, string> = {
  start: "hover:border-ok/40 hover:text-ok",
  unpause: "hover:border-ok/40 hover:text-ok",
  stop: "hover:border-bad/40 hover:text-bad",
  pause: "hover:border-warn/40 hover:text-warn",
  restart: "hover:border-accent/40 hover:text-accent",
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
  const { readOnly } = useSession();
  if (readOnly) return null;

  const dim = size === "sm" ? "h-3.5 w-3.5" : "h-4 w-4";
  const box = size === "sm" ? "h-7 w-7" : "h-8 w-8";

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
            aria-label={`${action} ${container.name}`}
            disabled={pending !== null}
            onClick={(e) => {
              e.stopPropagation();
              void run(action);
            }}
            className={`flex items-center justify-center rounded-md border border-edge bg-panel text-ink-muted transition disabled:opacity-40 ${box} ${HOVER[action]}`}
          >
            <Icon className={`${dim} ${pending === action ? "animate-pulse" : ""}`} />
          </button>
        );
      })}
    </div>
  );
}
