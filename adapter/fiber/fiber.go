// Package gologfiber connects golog to Fiber v3.
package gologfiber

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"github.com/kuhcokrow/golog"
)

// Middleware records every request. Queries and logs are linked to the
// request only when they use c.Context(). Errors returned by handlers are
// passed to the app's ErrorHandler here, so the error response is recorded too.
func Middleware(g *golog.Golog) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Fiber reuses its buffers after the handler returns, so every value
		// kept in an entry must be copied.
		path := strings.Clone(c.Path())
		if path == g.Path() || strings.HasPrefix(path, g.Path()+"/") {
			return c.Next()
		}

		start := time.Now()
		ctx := golog.WithRequestID(c.Context(), golog.NewRequestID())
		c.SetContext(ctx)
		reqBody := string(c.Request().Body())

		defer func() {
			p := recover()
			status := c.Response().StatusCode()
			if p != nil {
				g.Record(ctx, golog.TypeException, golog.ExceptionContent{
					Message: fmt.Sprint(p),
					Stack:   string(debug.Stack()),
				})
				status = http.StatusInternalServerError
			}

			g.RecordRequest(ctx, start, golog.RequestContent{
				Method:          strings.Clone(c.Method()),
				Path:            path,
				Query:           string(c.Request().URI().QueryString()),
				IP:              strings.Clone(c.IP()),
				Status:          status,
				Headers:         cloneHeaders(c.GetReqHeaders()),
				Body:            reqBody,
				ResponseHeaders: cloneHeaders(c.GetRespHeaders()),
				ResponseBody:    string(c.Response().Body()),
			})

			if p != nil {
				panic(p)
			}
		}()

		if err := c.Next(); err != nil {
			return c.App().Config().ErrorHandler(c, err)
		}
		return nil
	}
}

// Handler serves the dashboard, e.g. app.All("/golog/*", gologfiber.Handler(g)).
func Handler(g *golog.Golog) fiber.Handler {
	return adaptor.HTTPHandler(g.Handler())
}

func cloneHeaders(h map[string][]string) http.Header {
	c := make(http.Header, len(h))
	for k, vs := range h {
		cloned := make([]string, len(vs))
		for i, v := range vs {
			cloned[i] = strings.Clone(v)
		}
		c[strings.Clone(k)] = cloned
	}
	return c
}
