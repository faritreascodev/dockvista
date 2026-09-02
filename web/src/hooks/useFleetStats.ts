import { getContainerStats } from "../api/client";
import type { ContainerStats } from "../types/domain";
import { usePolling } from "./usePolling";

const POLL_INTERVAL_MS = 3000;

/**
 * Polls live stats for every given (running) container in parallel, once
 * per interval, and returns them keyed by container ID. Centralizing this
 * in one poller — rather than one per table row — means the table and the
 * detail drawer share a single set of requests instead of duplicating them.
 */
export function useFleetStats(runningIds: string[]) {
  const idsKey = [...runningIds].sort().join(",");

  const { data, error, loading } = usePolling(
    async () => {
      const entries = await Promise.all(
        runningIds.map(async (id): Promise<[string, ContainerStats] | null> => {
          try {
            return [id, await getContainerStats(id)];
          } catch {
            return null;
          }
        }),
      );
      const byId: Record<string, ContainerStats> = {};
      for (const entry of entries) {
        if (entry) byId[entry[0]] = entry[1];
      }
      return byId;
    },
    runningIds.length > 0 ? POLL_INTERVAL_MS : null,
    [idsKey],
  );

  return { statsById: data ?? {}, error, loading };
}
