package golog

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	// Path is where the dashboard is mounted. Defaults to "/golog".
	Path string
	// Store defaults to an in-memory store holding the latest 1000 entries.
	Store Store
	// Authorize guards the dashboard. When nil, everyone can access it.
	Authorize func(r *http.Request) bool
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
	return &Golog{cfg: cfg}
}

// Record saves an entry linked to the request ID found in ctx, if any.
func (g *Golog) Record(ctx context.Context, t EntryType, content any) {
	g.save(Entry{
		ID:        newID(),
		Type:      t,
		RequestID: RequestIDFrom(ctx),
		CreatedAt: time.Now(),
		Content:   content,
	})
}

// Errors are dropped on purpose: logging them could feed back into golog itself.
func (g *Golog) save(e Entry) {
	g.cfg.Store.Save(context.Background(), e)
}
