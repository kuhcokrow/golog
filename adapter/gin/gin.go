// Package gologgin connects golog to Gin.
package gologgin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kuhcokrow/golog"
)

// Middleware records every request. Register it after gin.Recovery so panics
// are recorded before Recovery turns them into a 500 response.
func Middleware(g *golog.Golog) gin.HandlerFunc {
	return func(c *gin.Context) {
		original := c.Writer
		defer func() { c.Writer = original }()

		g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Request = r
			c.Writer = &writer{ResponseWriter: original, w: w}
			c.Next()
		})).ServeHTTP(original, c.Request)
	}
}

// Handler serves the dashboard, e.g. r.Any("/golog/*any", gologgin.Handler(g)).
func Handler(g *golog.Golog) gin.HandlerFunc {
	return gin.WrapH(g.Handler())
}

// writer sends the body through golog's recorder while keeping Gin's own
// writer for everything else.
type writer struct {
	gin.ResponseWriter
	w http.ResponseWriter
}

func (w *writer) WriteHeader(code int) {
	w.w.WriteHeader(code)
}

func (w *writer) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w *writer) WriteString(s string) (int, error) {
	return w.w.Write([]byte(s))
}
