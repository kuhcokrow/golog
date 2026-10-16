package tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kuhcokrow/golog"
)

func TestMiddlewareCapturesRequest(t *testing.T) {
	g, store := newGolog(golog.Config{})
	h := g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("got " + string(b)))
	}))

	req := httptest.NewRequest("POST", "/users?x=1", strings.NewReader("hello"))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Body.String() != "got hello" {
		t.Fatalf("handler should still read the full body, got %q", rec.Body.String())
	}

	entries := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(entries) != 1 {
		t.Fatalf("expected 1 request entry, got %d", len(entries))
	}
	c := entries[0].Content.(golog.RequestContent)
	if c.Method != "POST" || c.Path != "/users" || c.Query != "x=1" || c.Status != http.StatusCreated {
		t.Fatalf("unexpected content: %+v", c)
	}
	if c.Body != "hello" || c.ResponseBody != "got hello" {
		t.Fatalf("unexpected bodies: %q / %q", c.Body, c.ResponseBody)
	}
	if got := c.Headers.Get("Authorization"); got != "********" {
		t.Fatalf("Authorization should be masked, got %q", got)
	}
	if entries[0].RequestID != entries[0].ID {
		t.Fatal("request entry should carry its own ID as request ID")
	}
}

func TestMiddlewareRecordsPanic(t *testing.T) {
	g, store := newGolog(golog.Config{})
	h := g.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	func() {
		defer func() {
			if p := recover(); p != "boom" {
				t.Fatalf("panic should be re-raised, got %v", p)
			}
		}()
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()

	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	excs := list(t, store, golog.Filter{Type: golog.TypeException})
	if len(reqs) != 1 || len(excs) != 1 {
		t.Fatalf("expected 1 request and 1 exception, got %d and %d", len(reqs), len(excs))
	}
	if s := reqs[0].Content.(golog.RequestContent).Status; s != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", s)
	}
	if excs[0].RequestID != reqs[0].ID {
		t.Fatal("exception should be linked to the request")
	}
	if m := excs[0].Content.(golog.ExceptionContent).Message; m != "boom" {
		t.Fatalf("unexpected message %q", m)
	}
}

func TestMiddlewareSkipsDashboard(t *testing.T) {
	g, store := newGolog(golog.Config{Path: "/debug"})
	h := g.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	for _, p := range []string{"/debug", "/debug/api/entries"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/debugger", nil))

	entries := list(t, store, golog.Filter{})
	if len(entries) != 1 || entries[0].Content.(golog.RequestContent).Path != "/debugger" {
		t.Fatalf("only /debugger should be recorded, got %+v", entries)
	}
}
