package audit_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"dockvista/internal/adapters/audit"
)

func TestLogger_WritesJSONLine(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	l, err := audit.New(dir, log)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Record("auth.login.ok", map[string]string{"user": "farit", "ip": "127.0.0.1", "password": ""})

	raw, err := os.ReadFile(filepath.Join(dir, "audit.log"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var row map[string]string
	if err := json.Unmarshal(raw[:len(raw)-1], &row); err != nil {
		t.Fatalf("json: %v body %s", err, raw)
	}
	if row["action"] != "auth.login.ok" || row["user"] != "farit" {
		t.Fatalf("row = %v", row)
	}
	if _, ok := row["password"]; ok {
		t.Fatal("empty fields must be omitted")
	}
	if !bytes.Contains(buf.Bytes(), []byte("auth.login.ok")) {
		t.Fatalf("slog missing action: %s", buf.Bytes())
	}
}
