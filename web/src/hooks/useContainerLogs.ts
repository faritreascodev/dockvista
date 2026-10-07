import { useEffect, useRef, useState } from "react";
import { logsUrl, type LogQuery } from "../api/client";
import type { LogLine } from "../types/domain";

const MAX_LINES = 1000;

export type LogConnectionState = "connecting" | "open" | "closed" | "error";

/**
 * Subscribes to a container's live log stream over SSE. The EventSource is
 * torn down automatically on unmount or when `id` / query options change,
 * which in turn cancels the request context on the Go server (see
 * internal/adapters/httpapi/sse.go) — no server-side goroutine outlives the
 * client connection.
 */
export function useContainerLogs(id: string | undefined, opts: LogQuery = {}) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [state, setState] = useState<LogConnectionState>("connecting");
  const sourceRef = useRef<EventSource>();
  const tail = opts.tail ?? "200";
  const timestamps = !!opts.timestamps;
  const since = opts.since ?? "";

  useEffect(() => {
    setLines([]);
    if (!id) {
      setState("closed");
      return;
    }

    setState("connecting");
    const source = new EventSource(logsUrl(id, { tail, timestamps, ...(since ? { since } : {}) }));
    sourceRef.current = source;

    source.onopen = () => setState("open");
    source.onerror = () => setState("error");
    source.onmessage = (event) => {
      const line = JSON.parse(event.data) as LogLine;
      setLines((prev) => {
        const next = [...prev, line];
        return next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next;
      });
    };

    return () => {
      source.close();
      setState("closed");
    };
  }, [id, tail, timestamps, since]);

  return { lines, state, clear: () => setLines([]) };
}
