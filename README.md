# Golog

A debugging dashboard for Go web apps, inspired by [Lens](https://lensjs.vercel.app/) and Laravel Telescope.

Golog records what happens inside your app and shows it in a dashboard served by the app itself:

- **Requests**: method, path, status, duration, headers, and request/response bodies
- **Queries**: SQL, bindings, duration, and errors (`database/sql` or GORM)
- **Logs**: every `log/slog` record with its attributes
- **Exceptions**: panics with their stack traces
- **Cache**: hits, misses, sets, and deletes (manual or go-redis)

Every entry is linked to the request that produced it. Open a request to see its queries, logs, exceptions, and cache operations in one place. The dashboard has search, a time range filter, and pagination, and refreshes automatically.

The UI is embedded in the Go binary, so there is nothing extra to deploy.

## Install

```bash
go get github.com/kuhcokrow/golog
```

Requires Go 1.25 or newer. Adapters are separate modules, so you only download what you use:

| Adapter | Install | Go |
| --- | --- | --- |
| Gin | `go get github.com/kuhcokrow/golog/adapter/gin` | 1.25 |
| Echo v4 | `go get github.com/kuhcokrow/golog/adapter/echo` | 1.25 |
| Fiber v3 | `go get github.com/kuhcokrow/golog/adapter/fiber` | 1.26 |
| GORM | `go get github.com/kuhcokrow/golog/adapter/gorm` | 1.25 |
| go-redis v9 | `go get github.com/kuhcokrow/golog/adapter/redis` | 1.26 |

## Quick start

```go
package main

import (
	"net/http"

	"github.com/kuhcokrow/golog"
)

func main() {
	g := golog.New(golog.Config{})

	mux := http.NewServeMux()
	mux.Handle("/golog/", g.Handler())
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	http.ListenAndServe(":8080", g.Middleware(mux))
}
```

Open http://localhost:8080/golog/ to see the dashboard.

## Watchers

### Linking entries to a request

`Middleware` stores a request ID in the request's `context.Context`. Queries, logs, and cache operations are linked to a request only when they receive that context. Use the `...Context` variants:

```go
db.QueryContext(r.Context(), "SELECT ...")   // linked
db.Query("SELECT ...")                       // recorded, but not linked

logger.InfoContext(r.Context(), "user loaded") // linked
logger.Info("user loaded")                     // recorded, but not linked
```

### Queries with `database/sql`

Wrap your driver and open the database with `sql.OpenDB`:

```go
import "github.com/jackc/pgx/v5/stdlib"

db := sql.OpenDB(g.WrapDriver(stdlib.GetDefaultDriver(), dsn))
```

Any `database/sql` driver works, for example `pq.Driver{}` or `&mysql.MySQLDriver{}`.

### Queries with GORM

```go
import gologgorm "github.com/kuhcokrow/golog/adapter/gorm"

db.Use(gologgorm.New(g))
db.WithContext(r.Context()).First(&user)
```

`gorm.ErrRecordNotFound` is not recorded as an error. Use either the GORM plugin or `WrapDriver`, not both, otherwise each query is recorded twice.

### Logs with `log/slog`

`SlogHandler` records each log record and then passes it on to your own handler, so your normal log output does not change:

```go
logger := slog.New(g.SlogHandler(slog.NewJSONHandler(os.Stdout, nil)))
slog.SetDefault(logger)
```

Grouped attributes are flattened with dots, for example `req.db.rows`.

### Cache

Record cache operations yourself:

```go
g.RecordCache(ctx, golog.CacheMiss, "user:7", nil)
g.RecordCache(ctx, golog.CacheSet, "user:7", user)
```

Or let the go-redis hook do it:

```go
import gologredis "github.com/kuhcokrow/golog/adapter/redis"

rdb.AddHook(gologredis.New(g))
```

The hook records every command that has a key, including pipelines. `GET` is recorded as `hit` or `miss`, `SET`/`SETNX`/`SETEX` as `set`, and `DEL`/`UNLINK` as `delete`. Other commands use their own name, for example `incr`.

### Exceptions

`Middleware` recovers panics, records them with their stack trace, and then re-panics. Your own recovery middleware, or `net/http`, still decides what the client receives.

### Custom entries

```go
g.Record(ctx, golog.TypeLog, golog.LogContent{Level: "INFO", Message: "job finished"})
```

## Framework adapters

### Gin

```go
import gologgin "github.com/kuhcokrow/golog/adapter/gin"

r := gin.New()
r.Use(gin.Recovery(), gologgin.Middleware(g))
r.Any("/golog/*any", gologgin.Handler(g))
```

Register the golog middleware after `gin.Recovery()` so panics are recorded before Recovery turns them into a 500 response. Use `c.Request.Context()` for queries and logs.

### Echo v4

```go
import gologecho "github.com/kuhcokrow/golog/adapter/echo"

e := echo.New()
e.Use(middleware.Recover(), gologecho.Middleware(g))
e.Any("/golog/*", gologecho.Handler(g))
```

Errors returned by handlers are passed to Echo's `HTTPErrorHandler` inside the middleware, so the error response is recorded as well. Use `c.Request().Context()` for queries and logs. Open the dashboard at `/golog/` (with the trailing slash).

### Fiber v3

```go
import gologfiber "github.com/kuhcokrow/golog/adapter/fiber"

app := fiber.New()
app.Use(recover.New(), gologfiber.Middleware(g))
app.All("/golog/*", gologfiber.Handler(g))
```

Use `c.Context()` for queries and logs. Errors returned by handlers are passed to the app's `ErrorHandler` inside the middleware. Open the dashboard at `/golog/` (with the trailing slash).

### Other frameworks

Any framework built on `net/http` (chi, gorilla/mux, and others) works with `g.Middleware` and `g.Handler` directly. For frameworks that are not, build the request yourself and record it with `g.RecordRequest`. See `adapter/fiber` for an example.

## Storage

### In memory (default)

Keeps the latest 1000 entries. When it is full, the oldest entries are overwritten. Entries are lost when the app restarts. There is no time limit unless you set one from the dashboard (see [Retention](#retention)).

```go
g := golog.New(golog.Config{Store: golog.NewMemoryStore(5000)})
```

### PostgreSQL

```go
import "github.com/kuhcokrow/golog/postgres"

db, _ := sql.Open("pgx", dsn)

store, err := postgres.New(db, postgres.Options{
	Table:     "golog_entries", // default
	Retention: 24 * time.Hour,  // initial value, default
})
if err != nil {
	log.Fatal(err)
}
defer store.Close()

g := golog.New(golog.Config{Store: store})
```

- The table and its indexes are created automatically if they do not exist.
- Entries are written asynchronously in batches (every 100 entries or 500 ms), so requests never wait for the database. If the write queue is full, new entries are dropped instead of slowing your app down.
- Entries older than the retention are deleted on start and then every minute.
- Retention changed from the dashboard is saved in a `<table>_settings` table and wins over `Options.Retention`, also after a restart.
- Call `Close` before your app exits to flush pending entries. It does not close your `*sql.DB`.
- Pass a `*sql.DB` that is **not** wrapped with `g.WrapDriver`, otherwise golog records its own queries.
- The package only uses `database/sql`, so use whichever PostgreSQL driver you like.

### Retention

Open **Settings** in the dashboard to choose how long entries are kept: no limit, 1 hour, 6 hours, 24 hours, 3 days, 7 days, or 30 days. The change applies immediately and older entries are removed right away.

Both built-in stores support this. The PostgreSQL store keeps the value across restarts; the in-memory store does not. You can also change it from code:

```go
store.SetRetention(ctx, 7*24*time.Hour) // 0 means no time limit
```

### Your own store

Implement `golog.Store`:

```go
type Store interface {
	Save(ctx context.Context, e Entry) error
	List(ctx context.Context, f Filter) ([]Entry, error) // newest first
	Get(ctx context.Context, id string) (Entry, bool, error)
	Clear(ctx context.Context) error
}
```

Also implement `Retention(ctx)` and `SetRetention(ctx, d)` (see `golog.RetentionStore`) to make retention configurable from the dashboard. Otherwise the setting is hidden.

## Configuration and security

```go
g := golog.New(golog.Config{
	Path:  "/debug", // where the dashboard lives, defaults to "/golog"
	Store: golog.NewMemoryStore(1000),
})
```

> [!WARNING]
> Golog records request bodies, response bodies, SQL bindings, and log attributes, which may contain personal data. Without protection, anyone who can reach the dashboard can read them, clear them, and change the retention. Always set `BasicAuth` or `Authorize`, or do not mount the dashboard, outside of local development.

### Basic auth

```go
g := golog.New(golog.Config{
	BasicAuth: &golog.BasicAuth{
		Username: "admin",
		Password: os.Getenv("GOLOG_PASSWORD"),
	},
})
```

The browser asks for the username and password when the dashboard is opened. `New` panics if the password is empty, so a missing environment variable never leaves the dashboard open.

Keep in mind that basic auth:

- sends credentials only base64 encoded, so use HTTPS outside of localhost;
- has no logout, the browser remembers the credentials until it is closed;
- supports a single username and password.

### Custom authorization

For anything else (several users, SSO, sessions, IP allow lists), use `Authorize`. Requests it rejects get a 403.

```go
g := golog.New(golog.Config{
	Authorize: func(r *http.Request) bool {
		return r.Header.Get("X-Debug-Token") == os.Getenv("DEBUG_TOKEN")
	},
})
```

When both `BasicAuth` and `Authorize` are set, a request must pass both.

### Built-in protections

- `Authorization`, `Cookie`, `Set-Cookie`, and `Proxy-Authorization` headers are masked as `********`.
- Request and response bodies are truncated to 64 KB.
- Requests to the dashboard itself are not recorded.

## Development

```
golog/
├── golog.go, types.go   public API (package golog)
├── internal/            implementation: capture, watcher, store, api, dashboard
├── postgres/            PostgreSQL store
├── adapter/             one module per framework or library
├── tests/               tests for the root module
├── examples/basic/      example app, also the backend for UI development
└── ui/                  dashboard source (React + Vite + TypeScript)
```

Set up a workspace so your editor and the Go tool see every module:

```bash
go work init . ./adapter/gin ./adapter/echo ./adapter/fiber ./adapter/gorm ./adapter/redis
```

### Tests

```bash
go test ./tests/...
for a in gin echo fiber gorm redis; do (cd adapter/$a && go test ./tests/); done
```

PostgreSQL and Redis tests are skipped unless these are set:

```bash
export GOLOG_TEST_POSTGRES_DSN='postgres://user:pass@localhost:5432/db?sslmode=disable'
export GOLOG_TEST_REDIS_ADDR=localhost:6379
```

### Example app

```bash
go run ./examples/basic
```

It listens on `:8080` with the dashboard at `/golog/`, and uses a fake database driver so it runs without any setup. Try `/users/7`, `POST /users`, `/broken-query`, and `/panic`.

### Dashboard UI

```bash
cd ui
pnpm install
pnpm dev     # http://localhost:5173, proxies /api to the example app on :8080
pnpm build   # writes to internal/dashboard/dist
```

Commit `internal/dashboard/dist` after building. It is embedded in the Go module, and people who `go get` golog do not run the build.

### Releasing

Tag the root module first, then point the adapters at it and tag them:

```bash
git tag v1.2.0 && git push origin v1.2.0

for a in gin echo fiber gorm redis; do
  (cd adapter/$a && go mod edit -require=github.com/kuhcokrow/golog@v1.2.0 && go mod tidy)
done
git commit -am "chore: require golog v1.2.0 in adapters" && git push

for a in gin echo fiber gorm redis; do git tag adapter/$a/v1.2.0; done
git push origin --tags
```

The `replace github.com/kuhcokrow/golog => ../..` lines in the adapters only apply during development. Go ignores them for people who depend on the adapters.

## License

[MIT](LICENSE)
