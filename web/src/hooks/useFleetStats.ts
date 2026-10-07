import { createContext, useContext } from "react";
import type { ContainerStats } from "../types/domain";

export interface FleetStats {
  /** Latest sample per running container. */
  statsById: Record<string, ContainerStats>;
  /** Rolling CPU% history per container, oldest first. */
  cpuSeriesById: Record<string, number[]>;
  /** Rolling fleet totals, oldest first. */
  totals: { cpu: number[]; memoryBytes: number[] };
  error: Error | undefined;
  loading: boolean;
}

export const FleetStatsContext = createContext<FleetStats>({
  statsById: {},
  cpuSeriesById: {},
  totals: { cpu: [], memoryBytes: [] },
  error: undefined,
  loading: true,
});

/**
 * Live resource usage for every running container. One provider polls
 * GET /api/stats for the whole app, so the table, the drawer, and the
 * overview charts all read the same samples instead of each polling.
 */
export function useFleetStats(): FleetStats {
  return useContext(FleetStatsContext);
}
