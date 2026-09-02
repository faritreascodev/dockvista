import { useEffect, useRef, useState, type DependencyList } from "react";

interface PollingState<T> {
  data: T | undefined;
  error: Error | undefined;
  loading: boolean;
}

/**
 * Polls `fetcher` every `intervalMs`, starting immediately. Pass `null` for
 * `intervalMs` to pause polling (e.g. while a drawer is closed) without
 * unmounting the hook. Errors don't clear the last-known-good data, so a
 * transient failure doesn't blank the UI mid-poll.
 */
export function usePolling<T>(
  fetcher: () => Promise<T>,
  intervalMs: number | null,
  deps: DependencyList,
): PollingState<T> {
  const [data, setData] = useState<T>();
  const [error, setError] = useState<Error>();
  const [loading, setLoading] = useState(true);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  useEffect(() => {
    if (intervalMs === null) return;

    let cancelled = false;
    setLoading(true);

    const tick = () => {
      fetcherRef
        .current()
        .then((result) => {
          if (cancelled) return;
          setData(result);
          setError(undefined);
        })
        .catch((err: unknown) => {
          if (cancelled) return;
          setError(err instanceof Error ? err : new Error(String(err)));
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    };

    tick();
    const id = window.setInterval(tick, intervalMs);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [intervalMs, ...deps]);

  return { data, error, loading };
}
