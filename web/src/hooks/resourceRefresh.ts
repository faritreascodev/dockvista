import { createContext, useContext } from "react";

export type ResourceType = "container" | "image" | "volume" | "network";

export type EventCounters = Record<ResourceType, number>;

export const ZERO_COUNTERS: EventCounters = { container: 0, image: 0, volume: 0, network: 0 };

export const EventsContext = createContext<EventCounters>(ZERO_COUNTERS);

export function useResourceRefreshSignal(type: ResourceType): number {
  return useContext(EventsContext)[type];
}
