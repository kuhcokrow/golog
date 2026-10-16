package tests

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/kuhcokrow/golog"
)

func TestSlogHandler(t *testing.T) {
	g, store := newGolog(golog.Config{})
	var out bytes.Buffer
	logger := slog.New(g.SlogHandler(slog.NewTextHandler(&out, nil))).
		With("svc", "api").
		WithGroup("req")

	ctx := golog.WithRequestID(context.Background(), "req-1")
	logger.WarnContext(ctx, "slow", "ms", 120, "err", errors.New("timeout"), slog.Group("db", "rows", 3))
	logger.Debug("hidden")

	if !strings.Contains(out.String(), "msg=slow") {
		t.Fatalf("log should be passed to the next handler, got %q", out.String())
	}

	entries := list(t, store, golog.Filter{Type: golog.TypeLog})
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry (debug is disabled), got %d", len(entries))
	}
	e := entries[0]
	c := e.Content.(golog.LogContent)
	if e.RequestID != "req-1" || c.Level != "WARN" || c.Message != "slow" {
		t.Fatalf("unexpected entry: %+v", e)
	}

	want := map[string]any{"svc": "api", "req.ms": int64(120), "req.err": "timeout", "req.db.rows": int64(3)}
	for k, v := range want {
		if c.Attrs[k] != v {
			t.Errorf("attr %s = %#v, want %#v", k, c.Attrs[k], v)
		}
	}
}
