package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"dockvista/internal/core/domain"
	"dockvista/pkg/httpjson"
)

type stackService interface {
	List() ([]domain.Stack, error)
	Get(id string) (domain.Stack, error)
	Create(name, yamlBody string) (domain.Stack, error)
	CreateFromGit(ctx context.Context, name, gitURL, gitRef, composeFile, username, token string) (domain.Stack, error)
	Update(id, yamlBody string) (domain.Stack, error)
	Sync(ctx context.Context, id string) (domain.Stack, error)
	Delete(id string) error
	Up(ctx context.Context, id string) error
	Down(ctx context.Context, id string, volumes bool) error
}

type stackDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	YAML        string `json:"yaml,omitempty"`
	GitURL      string `json:"gitUrl,omitempty"`
	GitRef      string `json:"gitRef,omitempty"`
	ComposeFile string `json:"composeFile,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

func toStackDTO(s domain.Stack, includeYAML bool) stackDTO {
	dto := stackDTO{
		ID: s.ID, Name: s.Name, GitURL: s.GitURL, GitRef: s.GitRef,
		ComposeFile: s.ComposeFile, CreatedAt: s.CreatedAt.Unix(), UpdatedAt: s.UpdatedAt.Unix(),
	}
	if includeYAML {
		dto.YAML = s.YAML
	}
	return dto
}

func stacksOf(r *http.Request) stackService {
	if e, ok := boundFrom(r.Context()); ok {
		return e.Stacks
	}
	return nil
}

func (h *stackHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		httpjson.Write(w, http.StatusOK, []stackDTO{})
		return
	}
	list, err := svc.List()
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]stackDTO, 0, len(list))
	for _, s := range list {
		out = append(out, toStackDTO(s, false))
	}
	httpjson.Write(w, http.StatusOK, out)
}

func (h *stackHandlers) handleGet(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}
	st, err := svc.Get(r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toStackDTO(st, true))
}

type stackWriteRequest struct {
	Name        string `json:"name"`
	YAML        string `json:"yaml"`
	GitURL      string `json:"gitUrl"`
	GitRef      string `json:"gitRef"`
	ComposeFile string `json:"composeFile"`
	GitUsername string `json:"gitUsername"`
	GitToken    string `json:"gitToken"`
}

func (h *stackHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		writeError(w, http.StatusNotImplemented, "stacks are not available")
		return
	}
	var req stackWriteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, domain.MaxStackYAML+4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.GitURL != "" && req.YAML != "" {
		writeError(w, http.StatusBadRequest, "provide yaml or gitUrl, not both")
		return
	}
	var (
		st  domain.Stack
		err error
	)
	if req.GitURL != "" {
		st, err = svc.CreateFromGit(r.Context(), req.Name, req.GitURL, req.GitRef, req.ComposeFile, req.GitUsername, req.GitToken)
	} else {
		st, err = svc.Create(req.Name, req.YAML)
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, toStackDTO(st, true))
}

func (h *stackHandlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}
	var req stackWriteRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, domain.MaxStackYAML+4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	st, err := svc.Update(r.PathValue("id"), req.YAML)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toStackDTO(st, true))
}

func (h *stackHandlers) handleSync(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}
	st, err := svc.Sync(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toStackDTO(st, false))
}

func (h *stackHandlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}
	if err := svc.Delete(r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *stackHandlers) handleUp(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}
	if err := svc.Up(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *stackHandlers) handleDown(w http.ResponseWriter, r *http.Request) {
	svc := stacksOf(r)
	if svc == nil {
		writeError(w, http.StatusNotFound, "stack not found")
		return
	}
	volumes := r.URL.Query().Get("volumes") == "true"
	if err := svc.Down(r.Context(), r.PathValue("id"), volumes); err != nil {
		writeServiceError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

type stackHandlers struct{}
