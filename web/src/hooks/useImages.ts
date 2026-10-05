import { listImages } from "../api/client";
import { usePolling } from "./usePolling";
import { useResourceRefreshSignal } from "./resourceRefresh";

const POLL_INTERVAL_MS = 20000;

export function useImages() {
  const refreshSignal = useResourceRefreshSignal("image");
  return usePolling(listImages, POLL_INTERVAL_MS, [refreshSignal]);
}
