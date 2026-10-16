package tests

import (
	"context"
	"testing"

	"github.com/kuhcokrow/golog"
)

func newGolog(cfg golog.Config) (*golog.Golog, golog.Store) {
	if cfg.Store == nil {
		cfg.Store = golog.NewMemoryStore(100)
	}
	return golog.New(cfg), cfg.Store
}

func list(t *testing.T, s golog.Store, f golog.Filter) []golog.Entry {
	t.Helper()
	entries, err := s.List(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func ids(t *testing.T, s golog.Store, f golog.Filter) []string {
	t.Helper()
	entries := list(t, s, f)
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
