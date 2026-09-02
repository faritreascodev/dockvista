// Package embedweb embeds the built frontend (web/dist, produced by
// `npm --prefix web run build`) into the Go binary so a single `go build`
// output is a fully self-contained deployable artifact.
//
// The dist directory is committed with a placeholder index.html so `go
// build` always compiles, even in a fresh checkout before the frontend has
// been built; Handler serves a clear message instead of a raw 404 in that
// case.
package embedweb

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

//go:embed all:dist
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded SPA build, with
// client-side-router fallback: any path without a matching static asset is
// served index.html so client-side routes survive a hard refresh.
func Handler() (http.Handler, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	return &spaHandler{fs: sub}, nil
}

type spaHandler struct {
	fs fs.FS
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	assetPath := strings.TrimPrefix(r.URL.Path, "/")

	if assetPath != "" && assetPath != "index.html" {
		if f, err := h.fs.Open(assetPath); err == nil {
			f.Close()
			http.FileServer(http.FS(h.fs)).ServeHTTP(w, r)
			return
		}
	}

	// No matching static asset: fall back to index.html for client-side
	// routing, or a friendly message if the frontend hasn't been built yet.
	index, err := fs.ReadFile(h.fs, "index.html")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("DockVista frontend is not built. Run `make frontend-build` (or `npm --prefix web run build`) and restart.\n"))
		return
	}

	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
}
