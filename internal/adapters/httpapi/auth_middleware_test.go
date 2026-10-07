package httpapi

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dockvista/internal/core/domain"
)

// fakeAuth accepts exactly one token for one user.
type fakeAuth struct {
	token string
	user  string
	role  domain.Role
}

func (f *fakeAuth) principal() domain.Principal {
	role := f.role
	if role == "" {
		role = domain.RoleAdmin
	}
	return domain.Principal{Username: f.user, Role: role}
}

func (f *fakeAuth) IsInitialized() (bool, error)                      { return true, nil }
func (f *fakeAuth) Setup(username, password, setupToken string) error { return nil }
func (f *fakeAuth) Login(u, p string) (string, time.Time, error)      { return "", time.Time{}, nil }
func (f *fakeAuth) RevokeSessions(username string) error              { return nil }
func (f *fakeAuth) ChangePassword(u, cur, next string) (string, time.Time, error) {
	return "", time.Time{}, nil
}
func (f *fakeAuth) VerifySession(token string) (domain.Principal, error) {
	if token == f.token {
		return f.principal(), nil
	}
	return domain.Principal{}, errors.New("bad token")
}
func (f *fakeAuth) ListUsers() ([]domain.User, error)     { return nil, nil }
func (f *fakeAuth) ListInvites() ([]domain.Invite, error) { return nil, nil }
func (f *fakeAuth) CreateInvite(actor domain.Principal, role domain.Role) (domain.Invite, string, error) {
	return domain.Invite{}, "", domain.ErrForbidden
}
func (f *fakeAuth) PeekInvite(token string) (domain.Invite, error) {
	return domain.Invite{}, domain.ErrInviteInvalid
}
func (f *fakeAuth) AcceptInvite(token, username, password string) error {
	return domain.ErrInviteInvalid
}
func (f *fakeAuth) DeleteUser(actor domain.Principal, username string) error {
	return domain.ErrForbidden
}
func (f *fakeAuth) SetRole(actor domain.Principal, username string, role domain.Role) error {
	return domain.ErrForbidden
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
