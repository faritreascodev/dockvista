// Package audit appends a JSON line per security-relevant event under the
// data directory. Stdout still gets the same event via slog so operators
// watching the process see it without opening the file.
package audit

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	fileName    = "audit.log"
	rotateBytes = 2 << 20
)

type Logger struct {
	mu   sync.Mutex
	path string
	log  *slog.Logger
}

func New(dataDir string, log *slog.Logger) (*Logger, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("audit: create data dir: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Logger{path: filepath.Join(dataDir, fileName), log: log}, nil
}

func (l *Logger) Record(action string, fields map[string]string) {
	if l == nil {
		return
	}
	row := map[string]string{
		"ts":     time.Now().UTC().Format(time.RFC3339Nano),
		"action": action,
	}
	attrs := []any{"audit", true, "action", action}
	for k, v := range fields {
		if v == "" {
			continue
		}
		row[k] = v
		attrs = append(attrs, k, v)
	}
	l.log.Info("audit", attrs...)

	data, err := json.Marshal(row)
	if err != nil {
		return
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	l.rotateIfNeeded()
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		l.log.Warn("audit write failed", "error", err)
		return
	}
	_, _ = f.Write(data)
	_ = f.Close()
}

func (l *Logger) rotateIfNeeded() {
	info, err := os.Stat(l.path)
	if err != nil || info.Size() < rotateBytes {
		return
	}
	_ = os.Rename(l.path, l.path+".1")
}
