import { Outlet } from "react-router-dom";
import { TopBar } from "./TopBar";
import { useEngineInfo } from "../hooks/useEngineInfo";

interface ShellProps {
  username: string;
  onLogout: () => void;
}

/** App-wide layout: top bar + whichever page is routed into the outlet. */
export function Shell({ username, onLogout }: ShellProps) {
  const { data: engine } = useEngineInfo();

  return (
    <div className="flex h-screen flex-col bg-canvas text-ink">
      <TopBar engine={engine} username={username} onLogout={onLogout} />
      <Outlet />
    </div>
  );
}
