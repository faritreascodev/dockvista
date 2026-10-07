import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { RotateCw, ShieldAlert, SquareTerminal } from "lucide-react";
import { openExecSession, type ExecConnectionState } from "../hooks/useContainerExec";
import { useSession } from "../hooks/useSession";
import { useTheme } from "../hooks/useTheme";
import { Button } from "./ui/Button";

const DARK_XTERM_THEME = {
  background: "#08090a",
  foreground: "#e8e6e1",
  cursor: "#f5a623",
  cursorAccent: "#08090a",
  selectionBackground: "#383b42",
};

const LIGHT_XTERM_THEME = {
  background: "#ece9e2",
  foreground: "#18181b",
  cursor: "#c46204",
  cursorAccent: "#ece9e2",
  selectionBackground: "#c7c2b7",
};

interface TerminalPanelProps {
  containerId: string;
  running: boolean;
}

const STATE_LABEL: Record<ExecConnectionState, string> = {
  connecting: "Connecting…",
  open: "Connected · shell",
  closed: "Session ended",
  error: "Connection lost",
};

export function TerminalPanel({ containerId, running }: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [state, setState] = useState<ExecConnectionState>("connecting");
  const [attempt, setAttempt] = useState(0);
  const { theme } = useTheme();
  const { readOnly } = useSession();

  useEffect(() => {
    const host = containerRef.current;
    if (!running || readOnly || !host) return;

    const term = new Terminal({
      fontSize: 13,
      lineHeight: 1.2,
      fontFamily: '"IBM Plex Mono", ui-monospace, SFMono-Regular, Menlo, monospace',
      theme: theme === "dark" ? DARK_XTERM_THEME : LIGHT_XTERM_THEME,
      cursorBlink: true,
      scrollback: 5000,
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(host);
    fitAddon.fit();

    const session = openExecSession(containerId, {
      onData: (chunk) => term.write(chunk),
      onStateChange: (next, reason) => {
        setState(next);
        if (next === "closed" || next === "error") {
          term.write(`\r\n\x1b[2m[${reason ?? (next === "closed" ? "session ended" : "connection lost")}]\x1b[0m\r\n`);
        }
      },
      onOpen: () => {
        fitAddon.fit();
        session.sendResize(term.cols, term.rows);
        term.focus();
      },
    });

    // Ctrl+C with a selection copies instead of sending SIGINT, and
    // Ctrl+Shift+C always copies, matching desktop terminal habits.
    term.attachCustomKeyEventHandler((event) => {
      if (event.type !== "keydown") return true;
      const isCopy = (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "c";
      if (isCopy && (event.shiftKey || term.hasSelection())) {
        void navigator.clipboard?.writeText(term.getSelection());
        term.clearSelection();
        event.preventDefault();
        return false;
      }
      return true;
    });

    const dataDisposable = term.onData((data) => session.sendInput(data));
    const resizeDisposable = term.onResize(({ cols, rows }) => session.sendResize(cols, rows));

    // The drawer resizes without a window resize event, so watch the box.
    const observer = new ResizeObserver(() => {
      try {
        fitAddon.fit();
      } catch {
        // fit() throws while the element is detached mid-unmount.
      }
    });
    observer.observe(host);

    return () => {
      observer.disconnect();
      dataDisposable.dispose();
      resizeDisposable.dispose();
      session.socket.close(1000);
      term.dispose();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [containerId, running, readOnly, attempt]);

  if (readOnly) {
    return (
      <Placeholder icon={ShieldAlert} text="Interactive shells are disabled on this read-only instance." />
    );
  }

  if (!running) {
    return <Placeholder icon={SquareTerminal} text="Start the container to open a terminal session." />;
  }

  const ended = state === "closed" || state === "error";

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between gap-3 border-b border-edge px-4 py-1.5 font-mono text-[11px] text-ink-muted">
        <span className="flex items-center gap-2">
          <span
            className={`h-1.5 w-1.5 rounded-full ${state === "open" ? "bg-ok live-dot" : state === "error" ? "bg-bad" : "bg-ink-faint"}`}
          />
          {STATE_LABEL[state]}
        </span>
        {ended ? (
          <Button size="sm" variant="ghost" onClick={() => setAttempt((n) => n + 1)}>
            <RotateCw className="h-3.5 w-3.5" /> Reconnect
          </Button>
        ) : (
          <span className="hidden text-ink-faint sm:inline">Ctrl+Shift+C copy · Ctrl+V paste</span>
        )}
      </div>
      <div ref={containerRef} className="min-h-0 flex-1 overflow-hidden bg-sunken px-2 py-2" />
    </div>
  );
}

function Placeholder({ icon: Icon, text }: { icon: typeof SquareTerminal; text: string }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 px-6 py-16 text-center">
      <Icon className="h-5 w-5 text-ink-faint" />
      <p className="text-sm text-ink-muted">{text}</p>
    </div>
  );
}
