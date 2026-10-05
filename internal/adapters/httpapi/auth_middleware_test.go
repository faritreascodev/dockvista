package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeAuth accepts exactly one token for one user.
type fakeAuth struct {
	token string
	user  string
}

func (f *fakeAuth) IsInitialized() (bool, error)                      { return true, nil }
func (f *fakeAuth) Setup(username, password, setupToken string) error { return nil }
func (f *fakeAuth) Login(u, p string) (string, time.Time, error)      { return "", time.Time{}, nil }
func (f *fakeAuth) RevokeSessions() error                             { return nil }
func (f *fakeAuth) VerifySession(token string) (string, error) {
	if token == f.token {
		return f.user, nil
	}
	return "", errors.New("bad token")
}

func TestRequireAuth(t *testing.T) {
	auth := &fakeAuth{token: "good", user: "admin"}
	var seenUser string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser, _ = r.Context().Value(usernameCtxKey).(string)
		w.WriteHeader(http.StatusNoContent)
	})
	h := requireAuth(auth)(next)

	cases := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
	}{
		{"no cookie", nil, http.StatusUnauthorized},
		{"bad token", &http.Cookie{Name: sessionCookieName, Value: "bad"}, http.StatusUnauthorized},
		{"wrong cookie name", &http.Cookie{Name: "other", Value: "good"}, http.StatusUnauthorized},
		{"valid token", &http.Cookie{Name: sessionCookieName, Value: "good"}, http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenUser = ""
			req := httptest.NewRequest(http.MethodGet, "/api/containers", nil)
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusNoContent && seenUser != "admin" {
				t.Fatalf("username in context = %q, want admin", seenUser)
			}
		})
	}
}
