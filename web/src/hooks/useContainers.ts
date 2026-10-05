import { listContainers } from "../api/client";
import { usePolling } from "./usePolling";
import { useResourceRefreshSignal } from "./resourceRefresh";

// Safety net only — the shared event feed (see useEventsContext) triggers an
// immediate refetch whenever the daemon reports a container change.
const POLL_INTERVAL_MS = 20000;

export function useContainers() {
  const refreshSignal = useResourceRefreshSignal("container");
  return usePolling(listContainers, POLL_INTERVAL_MS, [refreshSignal]);
}
