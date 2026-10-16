package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/kuhcokrow/golog"
	gologfiber "github.com/kuhcokrow/golog/adapter/fiber"
)

func setup() (*fiber.App, *golog.Golog, golog.Store) {
	store := golog.NewMemoryStore(100)
	g := golog.New(golog.Config{Store: store})

	app := fiber.New()
	app.Use(recover.New(), gologfiber.Middleware(g))
	app.All("/golog/*", gologfiber.Handler(g))
	return app, g, store
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
	app, g, store := setup()
	app.Post("/users/:id", func(c fiber.Ctx) error {
		g.Record(c.Context(), golog.TypeLog, golog.LogContent{Message: "inside"})
		return c.Status(http.StatusCreated).SendString("hi " + string(c.Body()))
	})

	req := httptest.NewRequest("POST", "/users/7?x=1", strings.NewReader("budi"))
	req.Header.Set("Authorization", "Bearer secret")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", res.StatusCode)
	}

	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	c := reqs[0].Content.(golog.RequestContent)
	if c.Method != "POST" || c.Path != "/users/7" || c.Query != "x=1" || c.Status != http.StatusCreated {
		t.Fatalf("unexpected content %+v", c)
	}
	if c.Body != "budi" || c.ResponseBody != "hi budi" {
		t.Fatalf("unexpected bodies %q / %q", c.Body, c.ResponseBody)
	}
	if got := c.Headers.Get("Authorization"); got != "********" {
		t.Fatalf("Authorization should be masked, got %q", got)
	}

	logs := list(t, store, golog.Filter{Type: golog.TypeLog})
	if len(logs) != 1 || logs[0].RequestID != reqs[0].ID {
		t.Fatal("log should be linked to the request through c.Context()")
	}
}

func TestMiddlewareRecordsReturnedError(t *testing.T) {
	app, _, store := setup()
	app.Get("/missing", func(fiber.Ctx) error {
		return fiber.NewError(http.StatusNotFound, "user not found")
	})

	res, err := app.Test(httptest.NewRequest("GET", "/missing", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.StatusCode)
	}

	reqs := list(t, store, golog.Filter{Type: golog.TypeRequest})
	if len(reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(reqs))
	}
	c := reqs[0].Content.(golog.RequestContent)
	if c.Status != http.StatusNotFound || c.ResponseBody != "user not found" {
		t.Fatalf("error response should be recorded, got %d %q", c.Status, c.ResponseBody)
	}
}

func TestMiddlewareRecordsPanic(t *testing.T) {
	app, _, store := setup()
	app.Get("/panic", func(fiber.Ctx) error { panic("boom") })

	res, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("recover should respond 500, got %d", res.StatusCode)
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
	app, _, store := setup()

	res, err := app.Test(httptest.NewRequest("GET", "/golog/api/entries", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if n := len(list(t, store, golog.Filter{})); n != 0 {
		t.Fatalf("dashboard requests should not be recorded, got %d entries", n)
	}
}
