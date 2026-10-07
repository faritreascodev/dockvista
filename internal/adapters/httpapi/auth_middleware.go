package httpapi

import "net/http"

// requireAuth rejects any request without a valid session cookie before it
// reaches the wrapped handler. It's applied to every /api/* route except the
// auth bootstrap endpoints themselves (see NewRouter).
func requireAuth(auth authService) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			principal, err := auth.VerifySession(cookie.Value)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
		})
	}
}
