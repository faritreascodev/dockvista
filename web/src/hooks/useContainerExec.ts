export type ExecConnectionState = "connecting" | "open" | "closed" | "error";

export interface ExecSession {
  socket: WebSocket;
  sendResize: (cols: number, rows: number) => void;
}

/**
 * Opens the exec WebSocket for a container and wires it up for xterm.js:
 * binary frames are raw terminal bytes in both directions; resize is sent
 * as a JSON text frame (see internal/adapters/httpapi/exec_handler.go).
 * Returns the raw session so the caller (TerminalPanel) can bind it
 * directly to a Terminal instance without an extra buffering layer.
 */
export function openExecSession(
  containerId: string,
  onData: (chunk: string) => void,
  onStateChange: (state: ExecConnectionState) => void,
): ExecSession {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${proto}//${window.location.host}/api/containers/${encodeURIComponent(containerId)}/exec`);
  socket.binaryType = "arraybuffer";

  onStateChange("connecting");
  const decoder = new TextDecoder();

  socket.onopen = () => onStateChange("open");
  socket.onclose = () => onStateChange("closed");
  socket.onerror = () => onStateChange("error");
  socket.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
      onData(decoder.decode(event.data));
    }
  };

  return {
    socket,
    sendResize: (cols, rows) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "resize", cols, rows }));
      }
    },
  };
}
