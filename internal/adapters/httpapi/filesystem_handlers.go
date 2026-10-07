package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"unicode"

	"dockvista/pkg/httpjson"
)

func (h *handlers) handleContainerFilesystem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}
	fs, err := h.c(r).Filesystem(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, fs)
}

func (h *handlers) handleContainerFiles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}
	listing, err := h.c(r).ListFiles(r.Context(), id, r.URL.Query().Get("path"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, listing)
}

func (h *handlers) handleContainerFileStat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}
	entry, err := h.c(r).FileStat(r.Context(), id, r.URL.Query().Get("path"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, entry)
}

func (h *handlers) handleContainerFileContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}
	body, entry, err := h.c(r).FileContent(r.Context(), id, r.URL.Query().Get("path"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	defer body.Close()

	filename := downloadFilename(entry.Name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if entry.SizeBytes > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(entry.SizeBytes, 10))
	}
	if _, err := io.Copy(w, body); err != nil {
		h.log.Warn("file download aborted", "container", id, "path", entry.Path, "error", err)
	}
}

func (h *handlers) handleContainerChanges(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isValidContainerID(id) {
		writeError(w, http.StatusBadRequest, "invalid container id")
		return
	}
	list, err := h.c(r).Changes(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, list)
}

func downloadFilename(name string) string {
	name = path.Base(name)
	if name == "" || name == "." || name == ".." || name == "/" {
		return "file"
	}
	var b strings.Builder
	for _, r := range name {
		if r == '"' || r == '\\' || r == '/' || r == '\n' || r == '\r' || unicode.IsControl(r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "file"
	}
	return b.String()
}
