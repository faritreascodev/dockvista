import { useEffect, useRef } from "react";
import type { DockerEvent } from "../types/domain";

/**
 * Subscribes to the backend's Docker event feed over SSE (one connection
 * for the whole app — mount this once near the root) and invokes `onEvent`
 * for each message as it arrives, near-instantly after the daemon reports
 * it. This is what drives real-time refresh; the interval polling elsewhere
 * (useContainers, useEngineInfo) is only a safety net in case an event is
 * ever missed.
 */
export function useLiveEvents(onEvent: (event: DockerEvent) => void) {
  const handlerRef = useRef(onEvent);
  handlerRef.current = onEvent;

  useEffect(() => {
    const source = new EventSource("/api/events");
    source.onmessage = (e) => {
      handlerRef.current(JSON.parse(e.data) as DockerEvent);
    };
    return () => source.close();
  }, []);
}
