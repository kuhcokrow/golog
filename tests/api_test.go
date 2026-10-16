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
