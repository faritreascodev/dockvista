import { useCallback, useEffect, useState } from "react";
import {
  getAuthStatus,
  getMe,
  login as apiLogin,
  logout as apiLogout,
  setupAdmin,
  type UserRole,
} from "../api/client";

type AuthState =
  | { phase: "loading" }
  | { phase: "needs-setup" }
  | { phase: "needs-login" }
  | { phase: "authenticated"; username: string; role: UserRole; readOnly: boolean; instanceReadOnly: boolean };

function fromUser(user: { username: string; role: UserRole; readOnly: boolean; instanceReadOnly: boolean }): AuthState {
  return {
    phase: "authenticated",
    username: user.username,
    role: user.role,
    readOnly: user.readOnly,
    instanceReadOnly: user.instanceReadOnly,
  };
}

export function useAuth() {
  const [state, setState] = useState<AuthState>({ phase: "loading" });

  const refresh = useCallback(async () => {
    try {
      const me = await getMe();
      setState(fromUser(me));
      return;
    } catch {
      // Not logged in (401) or a transient failure — either way, fall
      // through and resolve a definite state from /api/auth/status rather
      // than spinning forever.
    }

    try {
      const status = await getAuthStatus();
      setState(status.initialized ? { phase: "needs-login" } : { phase: "needs-setup" });
    } catch {
      setState({ phase: "needs-login" });
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const setup = useCallback(
    async (username: string, password: string, setupToken = "") => {
      await setupAdmin(username, password, setupToken);
      try {
        const user = await apiLogin(username, password);
        setState(fromUser(user));
      } catch {
        await refresh();
      }
    },
    [refresh],
  );

  const login = useCallback(async (username: string, password: string) => {
    const user = await apiLogin(username, password);
    setState(fromUser(user));
  }, []);

  const logout = useCallback(async () => {
    await apiLogout();
    setState({ phase: "needs-login" });
  }, []);

  return { state, setup, login, logout };
}
