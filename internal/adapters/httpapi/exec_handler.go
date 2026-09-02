package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
)

type resizeMessage struct {
	Type string `json:"type"`
	Rows uint   `json:"rows"`
	Cols uint   `json:"cols"`
}

// handleExec upgrades to a WebSocket and bridges it to a /bin/sh session
// inside the container: binary frames carry raw terminal I/O in both
// directions, text frames carry a {"type":"resize","rows":N,"cols":N}
// control message. This is the most sensitive endpoint in the app — it's
// only reachable through the same requireAuth wall as everything else
// under /api, and websocket.Accept's default same-origin check (we never
// relax it) means no other site's page can open this socket even indirectly.
func (h *handlers) handleExec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return // Accept already wrote the appropriate HTTP error response.
	}
	defer func() { _ = conn.CloseNow() }()

	ctx := r.Context()

	execID, err := h.svc.CreateExec(ctx, id)
	if err != nil {
		h.log.Warn("exec create failed", "container", id, "error", err)
		_ = conn.Close(websocket.StatusInternalError, "failed to start shell")
		return
	}

	execConn, err := h.svc.AttachExec(ctx, execID)
	if err != nil {
		h.log.Warn("exec attach failed", "container", id, "error", err)
		_ = conn.Close(websocket.StatusInternalError, "failed to attach shell")
		return
	}
	defer execConn.Close()

	done := make(chan struct{}, 2)

	// container -> browser
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 4096)
		for {
			n, readErr := execConn.Read(buf)
			if n > 0 {
				if writeErr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	// browser -> container, plus resize control messages
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msgType, data, readErr := conn.Read(ctx)
			if readErr != nil {
				return
			}
			switch msgType {
			case websocket.MessageBinary:
				if _, err := execConn.Write(data); err != nil {
					return
				}
			case websocket.MessageText:
				var msg resizeMessage
				if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Rows > 0 && msg.Cols > 0 {
					if err := h.svc.ResizeExec(ctx, execID, msg.Rows, msg.Cols); err != nil {
						h.log.Warn("exec resize failed", "container", id, "error", err)
					}
				}
			}
		}
	}()

	<-done
	_ = conn.Close(websocket.StatusNormalClosure, "")
}
