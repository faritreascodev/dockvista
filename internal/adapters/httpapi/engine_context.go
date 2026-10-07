package httpapi

import (
	"context"
	"net/http"

	"dockvista/internal/core/domain"
)

const envCookieName = "dockvista_env"
const envCtxKey ctxKey = 100

// BoundEngine is one daemon's services. The hub fills this per request.
type BoundEngine struct {
	Containers containerService
	Images     imageService
	Volumes    volumeService
	Networks   networkService
	System     systemService
	Storage    storageService
	Events     eventSubscriber
	Stacks     stackService
}

type EngineResolver interface {
	Resolve(ctx context.Context, id string) (BoundEngine, error)
	List(ctx context.Context) ([]domain.Environment, error)
	Create(ctx context.Context, spec domain.EnvironmentSpec) (domain.Environment, error)
	Delete(ctx context.Context, id string) error
}

func envIDFrom(r *http.Request) string {
	c, err := r.Cookie(envCookieName)
	if err != nil || c.Value == "" {
		return domain.LocalEnvironmentID
	}
	return c.Value
}

func bindEngine(res EngineResolver) middleware {
	return func(next http.Handler) http.Handler {
		if res == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			eng, err := res.Resolve(r.Context(), envIDFrom(r))
			if err != nil {
				writeServiceError(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), envCtxKey, eng)))
		})
	}
}

func boundFrom(ctx context.Context) (BoundEngine, bool) {
	e, ok := ctx.Value(envCtxKey).(BoundEngine)
	return e, ok
}

func (h *handlers) c(r *http.Request) containerService {
	if e, ok := boundFrom(r.Context()); ok && e.Containers != nil {
		return e.Containers
	}
	return h.svc
}

func (h *handlers) ev(r *http.Request) eventSubscriber {
	if e, ok := boundFrom(r.Context()); ok && e.Events != nil {
		return e.Events
	}
	return h.events
}

func (h *imageHandlers) s(r *http.Request) imageService {
	if e, ok := boundFrom(r.Context()); ok && e.Images != nil {
		return e.Images
	}
	return h.svc
}

func (h *volumeHandlers) s(r *http.Request) volumeService {
	if e, ok := boundFrom(r.Context()); ok && e.Volumes != nil {
		return e.Volumes
	}
	return h.svc
}

func (h *networkHandlers) s(r *http.Request) networkService {
	if e, ok := boundFrom(r.Context()); ok && e.Networks != nil {
		return e.Networks
	}
	return h.svc
}

func (h *systemHandlers) s(r *http.Request) systemService {
	if e, ok := boundFrom(r.Context()); ok && e.System != nil {
		return e.System
	}
	return h.svc
}

func (h *storageHandlers) s(r *http.Request) storageService {
	if e, ok := boundFrom(r.Context()); ok && e.Storage != nil {
		return e.Storage
	}
	return h.svc
}
