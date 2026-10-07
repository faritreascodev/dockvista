import { useEffect, useMemo, useState, type ReactNode } from "react";
import { getFleetStats } from "../api/client";
import type { ContainerStats } from "../types/domain";
import { FleetStatsContext, type FleetStats } from "./useFleetStats";
import { usePageVisible } from "./usePageVisible";
import { usePolling } from "./usePolling";

const POLL_INTERVAL_MS = 3000;
const TOTALS_HISTORY = 60; // 3 minutes at the poll interval
const PER_CONTAINER_HISTORY = 24;

interface History {
  cpuSeriesById: Record<string, number[]>;
  totals: { cpu: number[]; memoryBytes: number[] };
}

function push(series: number[] | undefined, value: number, limit: number): number[] {
  const next = series ? series.slice(-(limit - 1)) : [];
  next.push(value);
  return next;
}

function appendSample(prev: History, samples: ContainerStats[]): History {
  // Containers missing from this sample (stopped, removed) drop out, so
  // the map never grows past the set of currently running containers.
  const cpuSeriesById: Record<string, number[]> = {};
  let cpu = 0;
  let memoryBytes = 0;
  for (const s of samples) {
    cpuSeriesById[s.containerId] = push(prev.cpuSeriesById[s.containerId], s.cpuPercent, PER_CONTAINER_HISTORY);
    cpu += s.cpuPercent;
    memoryBytes += s.memoryUsageBytes;
  }
  return {
    cpuSeriesById,
    totals: {
      cpu: push(prev.totals.cpu, cpu, TOTALS_HISTORY),
      memoryBytes: push(prev.totals.memoryBytes, memoryBytes, TOTALS_HISTORY),
    },
  };
}

export function FleetStatsProvider({ children }: { children: ReactNode }) {
  const visible = usePageVisible();
  const { data, error, loading } = usePolling(getFleetStats, visible ? POLL_INTERVAL_MS : null, []);
  const [history, setHistory] = useState<History>({ cpuSeriesById: {}, totals: { cpu: [], memoryBytes: [] } });

  useEffect(() => {
    if (data) setHistory((prev) => appendSample(prev, data));
  }, [data]);

  const value = useMemo<FleetStats>(() => {
    const statsById: Record<string, ContainerStats> = {};
    for (const s of data ?? []) statsById[s.containerId] = s;
    return { statsById, cpuSeriesById: history.cpuSeriesById, totals: history.totals, error, loading };
  }, [data, history, error, loading]);

  return <FleetStatsContext.Provider value={value}>{children}</FleetStatsContext.Provider>;
}
