package golog

import (
	"context"
	"database/sql/driver"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kuhcokrow/golog/internal/api"
	"github.com/kuhcokrow/golog/internal/capture"
	"github.com/kuhcokrow/golog/internal/entry"
	"github.com/kuhcokrow/golog/internal/watcher"
)

type Config struct {
	// Path is where the dashboard is mounted. Defaults to "/golog".
	Path string
	// Store defaults to an in-memory store holding the latest 1000 entries.
	Store Store
	// BasicAuth makes the browser ask for a username and password before
	// showing the dashboard. Only use it over HTTPS outside of localhost.
	BasicAuth *BasicAuth
	// Authorize guards the dashboard. When both BasicAuth and Authorize are
	// set, a request must pass both. When neither is set, everyone can access
	// the dashboard.
	Authorize func(r *http.Request) bool
}

type BasicAuth struct {
	Username string
	// Password must not be empty.
	Password string
}

type Golog struct {
	cfg Config
}

func New(cfg Config) *Golog {
	cfg.Path = "/" + strings.Trim(cfg.Path, "/")
	if cfg.Path == "/" {
		cfg.Path = "/golog"
	}
	if cfg.Store == nil {
		cfg.Store = NewMemoryStore(1000)
	}
	// Failing loudly beats silently exposing the dashboard when, for example,
	// the environment variable holding the password is missing.
	if cfg.BasicAuth != nil && cfg.BasicAuth.Password == "" {
		panic("golog: BasicAuth.Password must not be empty")
	}
	return &Golog{cfg: cfg}
}

// Middleware records every request, except those to the dashboard itself.
func (g *Golog) Middleware(next http.Handler) http.Handler {
	return capture.Middleware(g.cfg.Path, g.save, next)
}

// Path is where the dashboard is mounted.
func (g *Golog) Path() string {
	return g.cfg.Path
}

// RecordRequest records a request handled outside of Middleware, for
// frameworks not built on net/http such as Fiber. ctx must carry the request
// ID (see WithRequestID). Headers are masked, bodies are truncated and the
// duration is measured from start, the same way Middleware does.
func (g *Golog) RecordRequest(ctx context.Context, start time.Time, c RequestContent) {
	g.save(capture.RequestEntry(RequestIDFrom(ctx), start, c))
}

// Handler serves the dashboard and its API. Mount it at Config.Path, e.g.
// mux.Handle("/golog/", g.Handler()).
func (g *Golog) Handler() http.Handler {
	access := api.Access{Authorize: g.cfg.Authorize}
	if g.cfg.BasicAuth != nil {
		access.Username = g.cfg.BasicAuth.Username
		access.Password = g.cfg.BasicAuth.Password
	}
	return api.Handler(g.cfg.Path, g.cfg.Store, access)
}

// WrapDriver returns a connector that records every query. Use it with
// sql.OpenDB. Queries are linked to a request only when run with a context,
// e.g. db.QueryContext(r.Context(), ...).
func (g *Golog) WrapDriver(d driver.Driver, dsn string) driver.Connector {
	return watcher.WrapDriver(g.save, d, dsn)
}

// SlogHandler records every log entry and then passes it to next.
func (g *Golog) SlogHandler(next slog.Handler) slog.Handler {
	return watcher.NewSlogHandler(g.save, next)
}

// Record saves an entry linked to the request ID found in ctx, if any.
func (g *Golog) Record(ctx context.Context, t EntryType, content any) {
	g.save(entry.New(ctx, t, content))
}

// RecordCache logs a cache operation. Value may be nil, e.g. for a miss or delete.
func (g *Golog) RecordCache(ctx context.Context, action CacheAction, key string, value any) {
	g.Record(ctx, TypeCache, CacheContent{Action: action, Key: key, Value: value})
}

// Errors are dropped on purpose: logging them could feed back into golog itself.
func (g *Golog) save(e Entry) {
	g.cfg.Store.Save(context.Background(), e)
}
