// Package gologecho connects golog to Echo v4.
package gologecho

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/kuhcokrow/golog"
)

// Middleware records every request. Errors returned by handlers are passed to
// Echo's HTTPErrorHandler here, so the error response is recorded too.
func Middleware(g *golog.Golog) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			res := c.Response()
			original := res.Writer
			defer func() { res.Writer = original }()

			g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.SetRequest(r)
				res.Writer = w
				if err := next(c); err != nil {
					c.Error(err)
				}
			})).ServeHTTP(original, c.Request())
			return nil
		}
	}
}

// Handler serves the dashboard, e.g. e.Any("/golog/*", gologecho.Handler(g)).
func Handler(g *golog.Golog) echo.HandlerFunc {
	return echo.WrapHandler(g.Handler())
}
