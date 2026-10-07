package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"dockvista/internal/core/domain"
)

const (
	listExecTimeout = 8 * time.Second
	copyTimeout     = 30 * time.Second
	inspectFSTimeout = 20 * time.Second
	maxLSBytes      = 1 << 20
)

func (c *Client) InspectFilesystem(ctx context.Context, id string) (domain.ContainerFS, error) {
	ctx, cancel := context.WithTimeout(ctx, inspectFSTimeout)
	defer cancel()

	info, _, err := c.sdk.ContainerInspectWithRaw(ctx, id, true)
	if err != nil {
		// Size calculation can fail on some drivers; inspect without it.
		info2, err2 := c.sdk.ContainerInspect(ctx, id)
		if err2 != nil {
			return domain.ContainerFS{}, wrapErr("inspect filesystem", err)
		}
		return mapFilesystem(info2, false), nil
	}
	return mapFilesystem(info, true), nil
}

func mapFilesystem(info dockertypes.ContainerJSON, sizeKnown bool) domain.ContainerFS {
	fs := domain.ContainerFS{
		Running:   info.State != nil && info.State.Running,
		SizeKnown: sizeKnown,
		Mounts:    mapMounts(info.Mounts),
	}
	if info.HostConfig != nil {
		fs.Privileged = info.HostConfig.Privileged
		fs.ReadonlyRootfs = info.HostConfig.ReadonlyRootfs
	}
	if sizeKnown {
		if info.SizeRw != nil {
			fs.SizeRw = *info.SizeRw
		}
		if info.SizeRootFs != nil {
			fs.SizeRootFs = *info.SizeRootFs
		}
	}
	return fs
}

func mapMounts(mounts []dockertypes.MountPoint) []domain.ContainerMount {
	out := make([]domain.ContainerMount, 0, len(mounts))
	for _, m := range mounts {
		out = append(out, domain.ContainerMount{
			Type:        string(m.Type),
			Name:        m.Name,
			Source:      m.Source,
			Destination: m.Destination,
			Driver:      m.Driver,
			Mode:        m.Mode,
			RW:          m.RW,
			Propagation: string(m.Propagation),
		})
	}
	return out
}

func (c *Client) StatContainerPath(ctx context.Context, id, path string) (domain.FSEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	stat, err := c.sdk.ContainerStatPath(ctx, id, path)
	if err != nil {
		return domain.FSEntry{}, wrapErr("stat container path", err)
	}
	return fsEntryFromStat(path, stat), nil
}

func fsEntryFromStat(path string, stat dockertypes.ContainerPathStat) domain.FSEntry {
	name := stat.Name
	if name == "" {
		name = baseName(path)
	}
	return domain.FSEntry{
		Name:       name,
		Path:       path,
		Dir:        stat.Mode.IsDir(),
		SizeBytes:  stat.Size,
		Mode:       stat.Mode.String(),
		ModTime:    stat.Mtime.UTC(),
		LinkTarget: stat.LinkTarget,
	}
}

func (c *Client) ListContainerDir(ctx context.Context, id, path string) (domain.DirListing, error) {
	ctx, cancel := context.WithTimeout(ctx, listExecTimeout)
	defer cancel()

	info, err := c.sdk.ContainerInspect(ctx, id)
	if err != nil {
		return domain.DirListing{}, wrapErr("inspect for file list", err)
	}
	running := info.State != nil && info.State.Running
	mounts := mapMounts(info.Mounts)

	stat, err := c.sdk.ContainerStatPath(ctx, id, path)
	if err != nil {
		return domain.DirListing{}, wrapErr("stat container path", err)
	}
	entry := fsEntryFromStat(path, stat)
	annotateMount(&entry, mounts)

	out := domain.DirListing{
		Path:    path,
		Running: running,
		Dir:     entry.Dir,
		Entry:   entry,
	}
	if !entry.Dir {
		return out, nil
	}
	if !running {
		out.Reason = domain.ListReasonNotRunning
		return out, nil
	}

	raw, lsErr := c.lsDir(ctx, id, path)
	if lsErr != nil {
		if wrapIsNoShell(lsErr) {
			out.Reason = domain.ListReasonNoShell
			return out, nil
		}
		return domain.DirListing{}, lsErr
	}
	entries, truncated := parseLS(path, raw)
	annotateMounts(entries, mounts)
	out.Entries = entries
	out.Truncated = truncated
	return out, nil
}

func (c *Client) lsDir(ctx context.Context, id, path string) (string, error) {
	cmds := [][]string{
		{"/bin/ls", "-1A", "-p", path},
		{"ls", "-1A", "-p", path},
	}
	var last error
	for _, cmd := range cmds {
		out, code, err := c.execOnce(ctx, id, cmd)
		if err != nil {
			last = err
			if wrapIsNoShell(err) {
				continue
			}
			return "", err
		}
		if code == 127 {
			last = domain.ErrNoShell
			continue
		}
		if code != 0 {
			if strings.Contains(strings.ToLower(out), "no such file") {
				return "", fmt.Errorf("docker: ls: %w", domain.ErrNotFound)
			}
			return "", fmt.Errorf("docker: ls exited %d: %w", code, domain.ErrEngine)
		}
		return out, nil
	}
	if last == nil {
		last = domain.ErrNoShell
	}
	return "", last
}

func wrapIsNoShell(err error) bool {
	if err == nil {
		return false
	}
	if err == domain.ErrNoShell {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "executable file not found") ||
		(strings.Contains(msg, "no such file or directory") && strings.Contains(msg, "create exec"))
}

func (c *Client) execOnce(ctx context.Context, containerID string, cmd []string) (string, int, error) {
	resp, err := c.sdk.ContainerExecCreate(ctx, containerID, dockertypes.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		Cmd:          cmd,
	})
	if err != nil {
		return "", 0, wrapErr("create exec", err)
	}
	hj, err := c.sdk.ContainerExecAttach(ctx, resp.ID, dockertypes.ExecStartCheck{})
	if err != nil {
		return "", 0, wrapErr("attach exec", err)
	}
	defer hj.Close()

	var stdout, stderr bytes.Buffer
	_, copyErr := stdcopy.StdCopy(&stdout, &stderr, hj.Reader)
	if copyErr != nil && copyErr != io.EOF && stdout.Len() == 0 {
		return "", 0, wrapErr("read exec", copyErr)
	}
	if stdout.Len() > maxLSBytes {
		stdout.Truncate(maxLSBytes)
	}
	inspect, err := c.sdk.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		return "", 0, wrapErr("inspect exec", err)
	}
	combined := stdout.String()
	if inspect.ExitCode != 0 && stderr.Len() > 0 {
		combined = stderr.String()
	}
	return combined, inspect.ExitCode, nil
}

func parseLS(dir, raw string) ([]domain.FSEntry, bool) {
	entries := make([]domain.FSEntry, 0, 16)
	sc := bufio.NewScanner(strings.NewReader(raw))
	truncated := false
	for sc.Scan() {
		name := strings.TrimRight(sc.Text(), "\r")
		if name == "" || name == "." || name == ".." {
			continue
		}
		dirent := strings.HasSuffix(name, "/")
		name = strings.TrimSuffix(name, "/")
		if name == "" {
			continue
		}
		entries = append(entries, domain.FSEntry{
			Name: name,
			Path: joinContainerPath(dir, name),
			Dir:  dirent,
		})
		if len(entries) >= domain.MaxDirEntries {
			truncated = true
			break
		}
	}
	return entries, truncated
}

func joinContainerPath(dir, name string) string {
	if dir == "/" {
		return "/" + name
	}
	return dir + "/" + name
}

func baseName(p string) string {
	if p == "/" {
		return "/"
	}
	if i := strings.LastIndex(p, "/"); i >= 0 && i < len(p)-1 {
		return p[i+1:]
	}
	return p
}

func annotateMount(entry *domain.FSEntry, mounts []domain.ContainerMount) {
	for _, m := range mounts {
		if m.Destination == entry.Path {
			entry.Mount = true
			entry.MountType = m.Type
			return
		}
	}
}

func annotateMounts(entries []domain.FSEntry, mounts []domain.ContainerMount) {
	byDest := make(map[string]domain.ContainerMount, len(mounts))
	for _, m := range mounts {
		byDest[m.Destination] = m
	}
	for i := range entries {
		if m, ok := byDest[entries[i].Path]; ok {
			entries[i].Mount = true
			entries[i].MountType = m.Type
		}
	}
}

func (c *Client) CopyContainerFile(ctx context.Context, id, path string) (io.ReadCloser, domain.FSEntry, error) {
	ctx, cancel := context.WithTimeout(ctx, copyTimeout)

	r, stat, err := c.sdk.CopyFromContainer(ctx, id, path)
	if err != nil {
		cancel()
		return nil, domain.FSEntry{}, wrapErr("copy from container", err)
	}
	entry := fsEntryFromStat(path, stat)
	if stat.Mode.IsDir() || stat.Mode&os.ModeDir != 0 {
		r.Close()
		cancel()
		return nil, entry, fmt.Errorf("docker: copy: %w", domain.ErrIsDirectory)
	}

	pr, pw := io.Pipe()
	go func() {
		defer cancel()
		defer r.Close()
		tr := tar.NewReader(r)
		hdr, nextErr := tr.Next()
		if nextErr != nil {
			pw.CloseWithError(nextErr)
			return
		}
		if hdr.Typeflag == tar.TypeDir {
			pw.CloseWithError(domain.ErrIsDirectory)
			return
		}
		n, copyErr := io.Copy(pw, io.LimitReader(tr, domain.MaxFileDownloadBytes+1))
		if n > domain.MaxFileDownloadBytes {
			pw.CloseWithError(domain.ErrTooLarge)
			return
		}
		pw.CloseWithError(copyErr)
	}()
	return pr, entry, nil
}

func (c *Client) ContainerChanges(ctx context.Context, id string) (domain.FSChangeList, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultCallTimeout)
	defer cancel()

	raw, err := c.sdk.ContainerDiff(ctx, id)
	if err != nil {
		return domain.FSChangeList{}, wrapErr("container diff", err)
	}
	return mapChanges(raw), nil
}

func mapChanges(raw []container.FilesystemChange) domain.FSChangeList {
	out := domain.FSChangeList{
		Changes: make([]domain.FSChange, 0, min(len(raw), domain.MaxFSChanges)),
	}
	for _, ch := range raw {
		kind := ch.Kind.String()
		switch kind {
		case "A":
			out.Added++
		case "D":
			out.Deleted++
		default:
			kind = "C"
			out.Modified++
		}
		if len(out.Changes) < domain.MaxFSChanges {
			out.Changes = append(out.Changes, domain.FSChange{Path: ch.Path, Kind: kind})
		} else {
			out.Truncated = true
		}
	}
	return out
}
