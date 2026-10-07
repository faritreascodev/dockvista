import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { listEnvironments, selectEnvironment } from "../api/client";
import type { Environment } from "../types/domain";

interface EnvState {
  environments: Environment[];
  active: Environment | undefined;
  generation: number;
  refresh: () => Promise<void>;
  select: (id: string) => Promise<void>;
}

const EnvContext = createContext<EnvState | null>(null);

export function EnvironmentProvider({ children }: { children: ReactNode }) {
  const [environments, setEnvironments] = useState<Environment[]>([]);
  const [generation, setGeneration] = useState(0);

  const refresh = useCallback(async () => {
    setEnvironments(await listEnvironments());
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const select = useCallback(
    async (id: string) => {
      await selectEnvironment(id);
      await refresh();
      setGeneration((n) => n + 1);
    },
    [refresh],
  );

  const active = environments.find((e) => e.active);
  const value = useMemo(
    () => ({ environments, active, generation, refresh, select }),
    [environments, active, generation, refresh, select],
  );
  return <EnvContext.Provider value={value}>{children}</EnvContext.Provider>;
}

export function useEnvironments(): EnvState {
  const ctx = useContext(EnvContext);
  if (!ctx) throw new Error("useEnvironments must be used inside EnvironmentProvider");
  return ctx;
}
