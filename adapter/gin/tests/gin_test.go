package tests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kuhcokrow/golog"
	gologgin "github.com/kuhcokrow/golog/adapter/gin"
)

func setup() (*gin.Engine, *golog.Golog, golog.Store) {
	gin.SetMode(gin.TestMode)
	store := golog.NewMemoryStore(100)
	g := golog.New(golog.Config{Store: store})

	r := gin.New()
	r.Use(gin.Recovery(), gologgin.Middleware(g))
	r.Any("/golog/*any", gologgin.Handler(g))
	return r, g, store
}

func list(t *testing.T, s golog.Store, f golog.Filter) []golog.Entry {
	t.Helper()
	entries, err := s.List(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestMiddlewareRecordsRequest(t *testing.T) {
	r, g, store := setup()
	r.POST("/users/:id", func(c *gin.Context) {
		b, _ := io.ReadAll(c.Request.Body)
		g.Record(c.Request.Context(), golog.TypeLog, golog.LogContent{Message: "inside"})
		c.String(http.StatusCreated, "hi %s", b)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("POST", "/users/7?x=1", strings.NewReader("budi")))
	if rec.Code != http.StatusCreated || rec.Body.String() != "hi budi" {
		t.Fatalf("unexpected response %d %q", rec.Code, rec.Body.String())
	}

	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	c := reqs[0].Content.(golog.RequestContent)
	if c.Path != "/users/7" || c.Query != "x=1" || c.Status != http.StatusCreated || c.Body != "budi" || c.ResponseBody != "hi budi" {
		t.Fatalf("unexpected content %+v", c)
	}

	logs := list(t, store, golog.Filter{Type: golog.TypeLog})
	if len(logs) != 1 || logs[0].RequestID != reqs[0].ID {
		t.Fatal("log should be linked to the request through c.Request.Context()")
	}
}

func TestMiddlewareRecordsStatusWithoutBody(t *testing.T) {
	r, _, store := setup()
	r.GET("/gone", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/gone", nil))

	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(reqs) != 1 || reqs[0].Content.(golog.RequestContent).Status != http.StatusNoContent {
		t.Fatalf("expected status 204 to be recorded, got %+v", reqs)
	}
}

func TestMiddlewareRecordsPanic(t *testing.T) {
	r, _, store := setup()
	r.GET("/panic", func(*gin.Context) { panic("boom") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("gin.Recovery should respond 500, got %d", rec.Code)
	}

	excs := list(t, store, golog.Filter{Type: golog.TypeException})
	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(excs) != 1 || len(reqs) != 1 || excs[0].RequestID != reqs[0].ID {
		t.Fatalf("expected a linked exception and request, got %d and %d", len(excs), len(reqs))
	}
	if s := reqs[0].Content.(golog.RequestContent).Status; s != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", s)
	}
}

func TestHandlerServesDashboard(t *testing.T) {
	r, _, store := setup()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/golog/api/entries", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := len(list(t, store, golog.Filter{})); n != 0 {
		t.Fatalf("dashboard requests should not be recorded, got %d entries", n)
	}
}
