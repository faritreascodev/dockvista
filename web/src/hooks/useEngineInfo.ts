import { getEngine } from "../api/client";
import { usePolling } from "./usePolling";
import { useResourceRefreshSignal } from "./resourceRefresh";

const POLL_INTERVAL_MS = 20000;

export function useEngineInfo() {
  // Engine info includes a container count, so container events are the
  // relevant refresh trigger here too.
  const refreshSignal = useResourceRefreshSignal("container");
  return usePolling(getEngine, POLL_INTERVAL_MS, [refreshSignal]);
}
