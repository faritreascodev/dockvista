import { getDiskUsage, getSystemInfo } from "../api/client";
import { usePolling } from "./usePolling";
import { useResourceRefreshSignal } from "./resourceRefresh";

export function useSystemInfo() {
  const containers = useResourceRefreshSignal("container");
  const images = useResourceRefreshSignal("image");
  return usePolling(getSystemInfo, 30000, [containers, images]);
}

/**
 * Disk usage is expensive for the daemon to compute, so the server caches
 * it; refetching on image/volume events just picks up its invalidation.
 */
export function useDiskUsage() {
  const images = useResourceRefreshSignal("image");
  const volumes = useResourceRefreshSignal("volume");
  return usePolling(getDiskUsage, 60000, [images, volumes]);
}
