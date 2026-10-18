package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kuhcokrow/golog"
)

type client struct {
	t      *testing.T
	h      http.Handler
	cookie *http.Cookie
}

func (c *client) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, "/golog"+path, strings.NewReader(body))
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "golog_session" {
			if ck.MaxAge < 0 {
				c.cookie = nil
			} else {
				c.cookie = ck
			}
		}
	}
	return rec
}

func (c *client) state() string {
	c.t.Helper()
	var s struct {
		Enabled  bool   `json:"enabled"`
		State    string `json:"state"`
		Username string `json:"username"`
	}
	json.Unmarshal(c.do("GET", "/api/auth", "").Body.Bytes(), &s)
	if !s.Enabled {
		return "disabled"
	}
	return s.State + ":" + s.Username
}

func TestAuthDisabledByDefault(t *testing.T) {
	g, _ := newGolog(golog.Config{})
	c := &client{t: t, h: g.Handler()}

	if s := c.state(); s != "disabled" {
		t.Fatalf("expected auth to be disabled, got %s", s)
	}
	if rec := c.do("GET", "/api/entries", ""); rec.Code != http.StatusOK {
		t.Fatalf("API should be open without auth, got %d", rec.Code)
	}
	if rec := c.do("POST", "/api/auth/register", `{"username":"a","password":"12345678","confirmPassword":"12345678"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("register should be unavailable when auth is disabled, got %d", rec.Code)
	}
}

func TestAuthRegisterLoginLogout(t *testing.T) {
	g, _ := newGolog(golog.Config{Auth: true})
	c := &client{t: t, h: g.Handler()}

	if s := c.state(); s != "setup:" {
		t.Fatalf("expected setup state, got %s", s)
	}
	if rec := c.do("GET", "/api/entries", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("API should need a session, got %d", rec.Code)
	}
	if rec := c.do("GET", "/", ""); rec.Code != http.StatusOK {
		t.Fatalf("UI should stay reachable to show the login page, got %d", rec.Code)
	}

	invalid := []string{
		`{"username":" ","password":"12345678","confirmPassword":"12345678"}`,
		`{"username":"admin","password":"short","confirmPassword":"short"}`,
		`{"username":"admin","password":"12345678","confirmPassword":"87654321"}`,
		`nope`,
	}
	for _, body := range invalid {
		if rec := c.do("POST", "/api/auth/register", body); rec.Code != http.StatusBadRequest {
			t.Errorf("register %s: expected 400, got %d", body, rec.Code)
		}
	}

	if rec := c.do("POST", "/api/auth/register", `{"username":" admin ","password":"secret123","confirmPassword":"secret123"}`); rec.Code != http.StatusOK {
		t.Fatalf("register failed: %d %s", rec.Code, rec.Body.String())
	}
	if c.cookie == nil || !c.cookie.HttpOnly || c.cookie.SameSite != http.SameSiteStrictMode || c.cookie.Path != "/golog/" {
		t.Fatalf("unexpected session cookie %+v", c.cookie)
	}
	if s := c.state(); s != "ok:admin" {
		t.Fatalf("register should log in, got %s", s)
	}
	if rec := c.do("GET", "/api/entries", ""); rec.Code != http.StatusOK {
		t.Fatalf("API should be open with a session, got %d", rec.Code)
	}

	other := &client{t: t, h: g.Handler()}
	if rec := other.do("POST", "/api/auth/register", `{"username":"evil","password":"secret123","confirmPassword":"secret123"}`); rec.Code != http.StatusConflict {
		t.Fatalf("only one user may register, got %d", rec.Code)
	}
	if s := other.state(); s != "login:" {
		t.Fatalf("expected login state for a new visitor, got %s", s)
	}
	for _, body := range []string{
		`{"username":"admin","password":"wrongpass"}`,
		`{"username":"root","password":"secret123"}`,
	} {
		if rec := other.do("POST", "/api/auth/login", body); rec.Code != http.StatusUnauthorized {
			t.Errorf("login %s: expected 401, got %d", body, rec.Code)
		}
	}
	if rec := other.do("POST", "/api/auth/login", `{"username":"admin","password":"secret123"}`); rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := other.do("GET", "/api/entries", ""); rec.Code != http.StatusOK {
		t.Fatalf("API should be open after login, got %d", rec.Code)
	}

	stale := c.cookie
	c.do("POST", "/api/auth/logout", "")
	if c.cookie != nil {
		t.Fatal("logout should clear the cookie")
	}
	c.cookie = stale
	if rec := c.do("GET", "/api/entries", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("session should be invalid after logout, got %d", rec.Code)
	}
	if rec := other.do("GET", "/api/entries", ""); rec.Code != http.StatusOK {
		t.Fatalf("other sessions should survive a logout, got %d", rec.Code)
	}
}

func TestAuthSessionsSharedAcrossHandlers(t *testing.T) {
	g, _ := newGolog(golog.Config{Auth: true})
	c := &client{t: t, h: g.Handler()}
	c.do("POST", "/api/auth/register", `{"username":"admin","password":"secret123","confirmPassword":"secret123"}`)

	c.h = g.Handler()
	if rec := c.do("GET", "/api/entries", ""); rec.Code != http.StatusOK {
		t.Fatalf("a session should work with every Handler of the same Golog, got %d", rec.Code)
	}
}

func TestAuthWithAuthorize(t *testing.T) {
	g, _ := newGolog(golog.Config{
		Auth:      true,
		Authorize: func(r *http.Request) bool { return r.Header.Get("X-Team") == "dev" },
	})
	c := &client{t: t, h: g.Handler()}

	if rec := c.do("POST", "/api/auth/register", `{"username":"admin","password":"secret123","confirmPassword":"secret123"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("Authorize should also guard registration, got %d", rec.Code)
	}
}

type plainUserlessStore struct{ golog.Store }

func TestAuthWithStoreWithoutUserSupport(t *testing.T) {
	g := golog.New(golog.Config{Auth: true, Store: plainUserlessStore{golog.NewMemoryStore(10)}})
	c := &client{t: t, h: g.Handler()}

	if rec := c.do("POST", "/api/auth/register", `{"username":"admin","password":"secret123","confirmPassword":"secret123"}`); rec.Code != http.StatusOK {
		t.Fatalf("should fall back to keeping the user in memory, got %d", rec.Code)
	}
	if s := c.state(); s != "ok:admin" {
		t.Fatalf("expected ok:admin, got %s", s)
	}
}
