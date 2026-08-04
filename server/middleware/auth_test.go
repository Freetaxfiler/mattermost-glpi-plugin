package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuthAuthenticated(t *testing.T) {
	mw := New(func(r *http.Request) (*CurrentUser, error) {
		return &CurrentUser{
			UserID:        "user1",
			Username:      "alice",
			Email:         "alice@example.com",
			IsSystemAdmin: true,
		}, nil
	})

	var got *CurrentUser
	h := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromRequest(r)
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/user", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got == nil || got.UserID != "user1" {
		t.Fatalf("expected CurrentUser user1 in context, got %+v", got)
	}
	if got.IsSystemAdmin != true {
		t.Fatal("expected IsSystemAdmin to be true")
	}
}

func TestRequireAuthRejectsMissingSession(t *testing.T) {
	mw := New(func(r *http.Request) (*CurrentUser, error) {
		return nil, errNotAuthenticated
	})

	called := false
	h := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/tickets", nil))

	if called {
		t.Fatal("handler must not be invoked for unauthenticated requests")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected structured JSON error, got %q: %v", rec.Body.String(), err)
	}
	if body["status"] != "error" || body["error"] == "" {
		t.Fatalf("expected {status:error, error:...}, got %v", body)
	}
}

func TestRequireAuthRejectsNilUser(t *testing.T) {
	mw := New(func(r *http.Request) (*CurrentUser, error) {
		return nil, nil
	})

	called := false
	h := mw.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/user", nil))

	if called {
		t.Fatal("handler must not be invoked when authenticator returns nil user")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCurrentUserHasRole(t *testing.T) {
	cu := &CurrentUser{Roles: "system_user system_admin"}
	if !cu.HasRole("system_admin") {
		t.Fatal("expected HasRole(system_admin) to be true")
	}
	if cu.HasRole("team_admin") {
		t.Fatal("expected HasRole(team_admin) to be false")
	}
}

var errNotAuthenticated = &authError{msg: "not authenticated"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }
