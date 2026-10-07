package docker

import (
	"context"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// shellCmd opens bash when the image has it and sh otherwise. Detection runs
// inside the container's own /bin/sh, so an image without one (FROM scratch,
// distroless) still fails with a clear error instead of a hang.
var shellCmd = []string{
	"/bin/sh", "-c",
	`if command -v bash >/dev/null 2>&1; then exec bash; fi; exec sh`,
}

// CreateExec starts an interactive shell exec session.
func (c *Client) CreateExec(ctx context.Context, containerID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	resp, err := c.sdk.ContainerExecCreate(ctx, containerID, dockertypes.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Env:          []string{"TERM=xterm-256color"},
		Cmd:          shellCmd,
	})
	if err != nil {
		return "", wrapErr("create exec", err)
	}
	return resp.ID, nil
}

// AttachExec hijacks the exec session's connection. Unlike the other
// methods here this isn't bounded by defaultCallTimeout — an interactive
// shell is meant to stay open until the caller closes it.
func (c *Client) AttachExec(ctx context.Context, execID string) (io.ReadWriteCloser, error) {
	hijacked, err := c.sdk.ContainerExecAttach(ctx, execID, dockertypes.ExecStartCheck{Tty: true})
	if err != nil {
		return nil, wrapErr("attach exec", err)
	}
	return &hijackedConn{hijacked}, nil
}

func (c *Client) ResizeExec(ctx context.Context, execID string, rows, cols uint) error {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	if err := c.sdk.ContainerExecResize(ctx, execID, container.ResizeOptions{Height: rows, Width: cols}); err != nil {
		return wrapErr("resize exec", err)
	}
	return nil
}

// hijackedConn adapts types.HijackedResponse (a Conn plus a separately
// buffered Reader — reads must go through Reader, since it may already hold
// bytes buffered during the connection handshake) into a plain
// io.ReadWriteCloser.
type hijackedConn struct {
	resp dockertypes.HijackedResponse
}

func (h *hijackedConn) Read(p []byte) (int, error)  { return h.resp.Reader.Read(p) }
func (h *hijackedConn) Write(p []byte) (int, error) { return h.resp.Conn.Write(p) }
func (h *hijackedConn) Close() error {
	h.resp.Close()
	return nil
}
