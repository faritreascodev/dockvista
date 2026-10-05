import { listNetworks } from "../api/client";
import { usePolling } from "./usePolling";
import { useResourceRefreshSignal } from "./resourceRefresh";

const POLL_INTERVAL_MS = 20000;

export function useNetworks() {
  const refreshSignal = useResourceRefreshSignal("network");
  return usePolling(listNetworks, POLL_INTERVAL_MS, [refreshSignal]);
}
