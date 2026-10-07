export type ExecConnectionState = "connecting" | "open" | "closed" | "error";

export interface ExecSession {
  socket: WebSocket;
  sendInput: (data: string) => void;
  sendResize: (cols: number, rows: number) => void;
}

interface ExecHandlers {
  onData: (chunk: string) => void;
  onStateChange: (state: ExecConnectionState, reason?: string) => void;
  onOpen?: () => void;
}

/**
 * Opens the exec WebSocket for a container. Keystrokes must travel as
 * binary frames: the server reads text frames only as JSON control messages
 * (resize), see internal/adapters/httpapi/exec_handler.go.
 */
export function openExecSession(containerId: string, handlers: ExecHandlers): ExecSession {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${proto}//${window.location.host}/api/containers/${encodeURIComponent(containerId)}/exec`);
  socket.binaryType = "arraybuffer";

  handlers.onStateChange("connecting");
  // stream: true keeps a multi-byte character that straddles two frames intact.
  const decoder = new TextDecoder();
  const encoder = new TextEncoder();

  socket.onopen = () => {
    handlers.onStateChange("open");
    handlers.onOpen?.();
  };
  socket.onclose = (event) => {
    handlers.onStateChange(event.code === 1000 ? "closed" : "error", event.reason || undefined);
  };
  socket.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
      handlers.onData(decoder.decode(event.data, { stream: true }));
    }
  };

  return {
    socket,
    sendInput: (data) => {
      if (socket.readyState === WebSocket.OPEN) socket.send(encoder.encode(data));
    },
    sendResize: (cols, rows) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "resize", cols, rows }));
      }
    },
  };
}
