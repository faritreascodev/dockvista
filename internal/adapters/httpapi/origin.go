package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

// requireSameOrigin rejects cookie-authenticated state changes that did not
// come from this origin. SameSite=Strict is not enough behind some proxies
// (the cookie can still be attached); browsers always send Origin on POST.
func requireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !needsOrigin(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !originMatchesHost(r) {
			writeError(w, http.StatusForbidden, "cross-origin request rejected")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func needsOrigin(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func originMatchesHost(r *http.Request) bool {
	raw := r.Header.Get("Origin")
	if raw == "" || raw == "null" {
		raw = r.Header.Get("Referer")
	}
	if raw == "" || raw == "null" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func setSameOrigin(r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	r.Header.Set("Origin", scheme+"://"+r.Host)
}
