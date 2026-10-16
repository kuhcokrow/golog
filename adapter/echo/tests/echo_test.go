package tests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/kuhcokrow/golog"
	gologecho "github.com/kuhcokrow/golog/adapter/echo"
)

func setup() (*echo.Echo, *golog.Golog, golog.Store) {
	store := golog.NewMemoryStore(100)
	g := golog.New(golog.Config{Store: store})

	e := echo.New()
	e.Use(middleware.Recover(), gologecho.Middleware(g))
	e.Any("/golog/*", gologecho.Handler(g))
	return e, g, store
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
	e, g, store := setup()
	e.POST("/users/:id", func(c echo.Context) error {
		b, _ := io.ReadAll(c.Request().Body)
		g.Record(c.Request().Context(), golog.TypeLog, golog.LogContent{Message: "inside"})
		return c.String(http.StatusCreated, "hi "+string(b))
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("POST", "/users/7?x=1", strings.NewReader("budi")))
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
		t.Fatal("log should be linked to the request through c.Request().Context()")
	}
}

func TestMiddlewareRecordsReturnedError(t *testing.T) {
	e, _, store := setup()
	e.GET("/missing", func(echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	c := reqs[0].Content.(golog.RequestContent)
	if c.Status != http.StatusNotFound || !strings.Contains(c.ResponseBody, "user not found") {
		t.Fatalf("error response should be recorded, got %d %q", c.Status, c.ResponseBody)
	}
}

func TestMiddlewareRecordsPanic(t *testing.T) {
	e, _, store := setup()
	e.GET("/panic", func(echo.Context) error { panic("boom") })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("Recover should respond 500, got %d", rec.Code)
	}

	excs := list(t, store, golog.Filter{Type: golog.TypeException})
	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(excs) != 1 || len(reqs) != 1 || excs[0].RequestID != reqs[0].ID {
		t.Fatalf("expected a linked exception and request, got %d and %d", len(excs), len(reqs))
	}
}

func TestHandlerServesDashboard(t *testing.T) {
	e, _, store := setup()

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest("GET", "/golog/api/entries", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if n := len(list(t, store, golog.Filter{})); n != 0 {
		t.Fatalf("dashboard requests should not be recorded, got %d entries", n)
	}
}
