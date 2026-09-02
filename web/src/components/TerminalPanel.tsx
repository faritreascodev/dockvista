import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { openExecSession, type ExecConnectionState } from "../hooks/useContainerExec";
import { useTheme } from "../hooks/useTheme";

const DARK_XTERM_THEME = {
  background: "#0b0f17",
  foreground: "#e2e8f0",
  cursor: "#38bdf8",
  selectionBackground: "#334155",
};

const LIGHT_XTERM_THEME = {
  background: "#ffffff",
  foreground: "#0f172a",
  cursor: "#0ea5e9",
  selectionBackground: "#cbd5e1",
};

interface TerminalPanelProps {
  containerId: string;
  running: boolean;
}

export function TerminalPanel({ containerId, running }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [state, setState] = useState<ExecConnectionState>("connecting");
  const { theme } = useTheme();

  useEffect(() => {
    if (!running || !containerRef.current) return;

    const term = new Terminal({
      convertEol: true,
      fontSize: 13,
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
      theme: theme === "dark" ? DARK_XTERM_THEME : LIGHT_XTERM_THEME,
      cursorBlink: true,
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(containerRef.current);
    fitAddon.fit();

    const session = openExecSession(containerId, (chunk) => term.write(chunk), setState);

    const dataDisposable = term.onData((data) => {
      if (session.socket.readyState === WebSocket.OPEN) {
        session.socket.send(data);
      }
    });

    const resizeDisposable = term.onResize(({ cols, rows }) => session.sendResize(cols, rows));

    const handleWindowResize = () => fitAddon.fit();
    window.addEventListener("resize", handleWindowResize);

    // Fit once more after the WS opens — the panel's flex layout can settle
    // to a slightly different size than at mount time.
    const openFitTimer = window.setTimeout(() => fitAddon.fit(), 50);

    return () => {
      window.clearTimeout(openFitTimer);
      window.removeEventListener("resize", handleWindowResize);
      dataDisposable.dispose();
      resizeDisposable.dispose();
      session.socket.close();
      term.dispose();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [containerId, running]);

  if (!running) {
    return (
      <div className="flex h-full items-center justify-center px-6 text-center text-sm text-ink-muted">
        Start the container to open a terminal session.
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-edge px-4 py-2 text-xs text-ink-muted">
        <span>
          {state === "open"
            ? "Connected — /bin/sh"
            : state === "connecting"
              ? "Connecting…"
              : state === "error"
                ? "Connection error"
                : "Disconnected"}
        </span>
        <span
          className={`h-1.5 w-1.5 rounded-full ${state === "open" ? "bg-emerald-500" : state === "error" ? "bg-rose-500" : "bg-ink-faint"}`}
        />
      </div>
      <div ref={containerRef} className="min-h-0 flex-1 overflow-hidden px-2 py-2" />
    </div>
  );
}
