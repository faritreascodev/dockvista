package httpapi

import (
	"net/http"

	"dockvista/internal/core/domain"
)

// Auditor records security-relevant events. The file adapter in
// internal/adapters/audit is the production implementation; tests pass nil.
type Auditor interface {
	Record(action string, fields map[string]string)
}

func auditFields(r *http.Request, extra map[string]string) map[string]string {
	p := principalFrom(r.Context())
	fields := map[string]string{
		"ip":     clientKey(r),
		"method": r.Method,
		"path":   r.URL.Path,
	}
	if p.Username != "" {
		fields["user"] = p.Username
		fields["role"] = string(p.Role)
	}
	for k, v := range extra {
		fields[k] = v
	}
	return fields
}

func (h *authHandlers) record(action string, r *http.Request, extra map[string]string) {
	if h == nil || h.audit == nil {
		return
	}
	h.audit.Record(action, auditFields(r, extra))
}

func auditMutating(a Auditor) middleware {
	return func(next http.Handler) http.Handler {
		if a == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if rec.status >= 200 && rec.status < 300 {
				p := principalFrom(r.Context())
				if p.Role == domain.RoleViewer {
					return
				}
				a.Record("http.mutate", auditFields(r, nil))
			}
		})
	}
}
