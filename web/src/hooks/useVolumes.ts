import { listVolumes } from "../api/client";
import { usePolling } from "./usePolling";
import { useResourceRefreshSignal } from "./resourceRefresh";

const POLL_INTERVAL_MS = 20000;

export function useVolumes() {
  const refreshSignal = useResourceRefreshSignal("volume");
  return usePolling(listVolumes, POLL_INTERVAL_MS, [refreshSignal]);
}
