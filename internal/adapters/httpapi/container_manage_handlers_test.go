package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dockvista/internal/core/domain"
)

// fakeCreateService embeds the interface so only Create needs an
// implementation; any other method call panics, which is the intent.
type fakeCreateService struct {
	containerService
	id      string
	started bool
	err     error
}

func (f *fakeCreateService) Create(ctx context.Context, spec domain.ContainerSpec) (string, bool, error) {
	return f.id, f.started, f.err
}

func postCreate(t *testing.T, svc containerService) *httptest.ResponseRecorder {
	t.Helper()
	h := &handlers{svc: svc, log: slog.Default()}
	req := httptest.NewRequest(http.MethodPost, "/api/containers", strings.NewReader(`{"image":"nginx:alpine"}`))
	rec := httptest.NewRecorder()
	h.handleCreateContainer(rec, req)
	return rec
}

func TestCreateContainer_Success(t *testing.T) {
	rec := postCreate(t, &fakeCreateService{id: "abc123", started: true})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var got createContainerResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "abc123" || !got.Started || got.Status != "ok" {
		t.Fatalf("body = %+v", got)
	}
}

// A container that was created but failed to start must come back with its
// id, so the client can tell the user what exists instead of hiding it.
func TestCreateContainer_CreatedButNotStarted(t *testing.T) {
	svc := &fakeCreateService{id: "abc123", started: false, err: errors.New("start failed")}
	rec := postCreate(t, svc)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var got createContainerResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "abc123" || got.Started || got.Status != "created" || got.Error == "" {
		t.Fatalf("body = %+v", got)
	}
}

func TestCreateContainer_NothingCreatedIsPlainError(t *testing.T) {
	rec := postCreate(t, &fakeCreateService{err: errors.New("image not found")})
	if rec.Code == http.StatusCreated {
		t.Fatalf("status = %d for a failed create", rec.Code)
	}
	// Must be the generic error body, not the created-container shape.
	if strings.Contains(rec.Body.String(), `"started"`) {
		t.Fatalf("error response carries container fields: %s", rec.Body.String())
	}
}
