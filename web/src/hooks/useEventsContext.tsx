import { useRef, useState } from "react";
import { useLiveEvents } from "./useLiveEvents";
import type { DockerEvent } from "../types/domain";
import { EventsContext, ZERO_COUNTERS, type EventCounters, type ResourceType } from "./resourceRefresh";

const KNOWN_TYPES = new Set<ResourceType>(["container", "image", "volume", "network"]);

// Docker reports exec_create/exec_start/exec_die for every health-check
// probe (frequently, once per container per probe interval) and
// attach/detach/resize/top for other tooling — none of those change what
// the list endpoints return, so counting them as "refetch now" would floor
// the poll interval into a tight loop on any fleet with active health
// checks. Mirrors the same allow-list the backend uses to decide whether to
// refresh its own cache (see containerListActions in cmd/dockvista/main.go).
const RELEVANT_CONTAINER_ACTIONS = new Set([
  "create",
  "start",
  "stop",
  "die",
  "destroy",
  "pause",
  "unpause",
  "restart",
  "rename",
  "kill",
  "update",
]);

function isRelevantEvent(event: DockerEvent): boolean {
  if (event.type !== "container") return true; // images/volumes/networks: no noisy sub-events to filter
  return RELEVANT_CONTAINER_ACTIONS.has(event.action) || event.action.startsWith("health_status");
}

/**
 * Wraps the app once with a single SSE connection (see useLiveEvents) and
 * exposes a per-resource-type counter that increments on every relevant
 * daemon event. Pages read their own counter via useResourceRefreshSignal
 * and pass it into their polling hook's deps, turning "an event happened"
 * into "refetch now" without every page opening its own connection.
 */
// A burst of legitimate events close together (e.g. `docker compose up` on a
// multi-service stack) should still only trigger one refetch, not one per
// event — this coalesces same-type increments that land within the window
// into a single counter bump instead of firing on every one.
const COALESCE_WINDOW_MS = 500;

export function EventsProvider({ children }: { children: React.ReactNode }) {
  const [counters, setCounters] = useState<EventCounters>(ZERO_COUNTERS);
  const pendingRef = useRef<Partial<Record<ResourceType, number>>>({});

  useLiveEvents((event: DockerEvent) => {
    if (!KNOWN_TYPES.has(event.type as ResourceType) || !isRelevantEvent(event)) return;
    const type = event.type as ResourceType;

    if (pendingRef.current[type] === undefined) {
      pendingRef.current[type] = window.setTimeout(() => {
        delete pendingRef.current[type];
        setCounters((prev) => ({ ...prev, [type]: prev[type] + 1 }));
      }, COALESCE_WINDOW_MS);
    }
  });

  return <EventsContext.Provider value={counters}>{children}</EventsContext.Provider>;
}
