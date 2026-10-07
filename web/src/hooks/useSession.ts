import { createContext, useContext } from "react";
import type { UserRole } from "../api/client";

export interface Session {
  username: string;
  role: UserRole;
  readOnly: boolean;
  instanceReadOnly: boolean;
}

export const SessionContext = createContext<Session>({
  username: "",
  role: "viewer",
  readOnly: false,
  instanceReadOnly: false,
});

/** The signed-in user, their role, and whether mutations are forbidden. */
export function useSession(): Session {
  return useContext(SessionContext);
}
