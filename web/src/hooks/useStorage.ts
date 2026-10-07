import { useEffect, useRef, useState } from "react";
import { getStorageInventory } from "../api/client";
import { usePolling } from "./usePolling";
import { useResourceRefreshSignal } from "./resourceRefresh";

/**
 * `docker system df` is expensive, so this poller is slower than the
 * container list and coalesces a burst of daemon events into one refetch.
 * The first event-counter snapshot is ignored so mount does not cancel
 * the initial request.
 */
export function useStorageInventory(nonce = 0) {
  const containers = useResourceRefreshSignal("container");
  const images = useResourceRefreshSignal("image");
  const volumes = useResourceRefreshSignal("volume");
  const [events, setEvents] = useState(0);
  const skip = useRef(true);

  useEffect(() => {
    if (skip.current) {
      skip.current = false;
      return;
    }
    const timer = window.setTimeout(() => setEvents((n) => n + 1), 2500);
    return () => window.clearTimeout(timer);
  }, [containers, images, volumes]);

  return usePolling(getStorageInventory, 60_000, [events, nonce]);
}
