package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kuhcokrow/golog"
)

func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func TestSettingsAPI(t *testing.T) {
	g, store := newGolog(golog.Config{})
	h := g.Handler()

	if rec := call(h, "GET", "/golog/api/settings", ""); strings.TrimSpace(rec.Body.String()) != `{"retention":0}` {
		t.Fatalf("unexpected settings %d %s", rec.Code, rec.Body.String())
	}

	rec := call(h, "PUT", "/golog/api/settings", `{"retention":3600}`)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"retention":3600}` {
		t.Fatalf("unexpected update response %d %s", rec.Code, rec.Body.String())
	}
	if d, _ := store.(golog.RetentionStore).Retention(context.Background()); d != time.Hour {
		t.Fatalf("store retention should be 1h, got %v", d)
	}

	for _, body := range []string{`{"retention":30}`, `{"retention":-60}`, `{}`, `nope`} {
		if rec := call(h, "PUT", "/golog/api/settings", body); rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: expected 400, got %d", body, rec.Code)
		}
	}
}

type plainStore struct{ golog.Store }

func TestSettingsAPIWithoutRetentionSupport(t *testing.T) {
	g := golog.New(golog.Config{Store: plainStore{golog.NewMemoryStore(10)}})
	h := g.Handler()

	if rec := call(h, "GET", "/golog/api/settings", ""); strings.TrimSpace(rec.Body.String()) != `{"retention":null}` {
		t.Fatalf("unsupported store should report null retention, got %s", rec.Body.String())
	}
	if rec := call(h, "PUT", "/golog/api/settings", `{"retention":3600}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSettingsAPIRequiresAuthorization(t *testing.T) {
	g, _ := newGolog(golog.Config{Authorize: func(*http.Request) bool { return false }})

	if rec := call(g.Handler(), "PUT", "/golog/api/settings", `{"retention":3600}`); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestBasicAuth(t *testing.T) {
	g, _ := newGolog(golog.Config{BasicAuth: &golog.BasicAuth{Username: "admin", Password: "secret"}})
	h := g.Handler()

	tests := []struct {
		name       string
		user, pass string
		setAuth    bool
		want       int
	}{
		{"no credentials", "", "", false, http.StatusUnauthorized},
		{"wrong password", "admin", "nope", true, http.StatusUnauthorized},
		{"wrong username", "root", "secret", true, http.StatusUnauthorized},
		{"correct", "admin", "secret", true, http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/golog/api/entries", nil)
			if tt.setAuth {
				req.SetBasicAuth(tt.user, tt.pass)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, rec.Code)
			}
			if tt.want == http.StatusUnauthorized && !strings.HasPrefix(rec.Header().Get("WWW-Authenticate"), "Basic ") {
				t.Fatal("401 should ask the browser for basic auth credentials")
			}
		})
	}
}

func TestBasicAuthWithAuthorize(t *testing.T) {
	g, _ := newGolog(golog.Config{
		BasicAuth: &golog.BasicAuth{Username: "admin", Password: "secret"},
		Authorize: func(r *http.Request) bool { return r.Header.Get("X-Team") == "dev" },
	})
	h := g.Handler()

	req := httptest.NewRequest("GET", "/golog/api/entries", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("passing basic auth alone should not be enough, got %d", rec.Code)
	}

	req.Header.Set("X-Team", "dev")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("passing both should be allowed, got %d", rec.Code)
	}
}

func TestBasicAuthRejectsEmptyPassword(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("New should panic when BasicAuth.Password is empty")
		}
	}()
	golog.New(golog.Config{BasicAuth: &golog.BasicAuth{Username: "admin"}})
}
