package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/kuhcokrow/golog"
	"github.com/kuhcokrow/golog/postgres"
)

// These tests need a running PostgreSQL, e.g.
// GOLOG_TEST_POSTGRES_DSN=postgres://user:pass@localhost:5432/db go test ./tests/postgres/
func openDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("GOLOG_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOLOG_TEST_POSTGRES_DSN is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	table := fmt.Sprintf("golog_test_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS " + table + ", " + table + "_settings")
		db.Close()
	})
	return db, table
}

func newStore(t *testing.T, db *sql.DB, opts postgres.Options) *postgres.Store {
	t.Helper()
	s, err := postgres.New(db, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func ids(t *testing.T, s golog.Store, f golog.Filter) []string {
	t.Helper()
	entries, err := s.List(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
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

func TestStoreSaveListGet(t *testing.T) {
	db, table := openDB(t)
	s := newStore(t, db, postgres.Options{Table: table})
	ctx := context.Background()
	base := time.Now().Add(-time.Hour).Truncate(time.Second)

	s.Save(ctx, golog.Entry{ID: "1", Type: golog.TypeRequest, RequestID: "r1", CreatedAt: base,
		Content: golog.RequestContent{Method: "GET", Path: "/users"}})
	s.Save(ctx, golog.Entry{ID: "2", Type: golog.TypeQuery, RequestID: "r1", CreatedAt: base.Add(time.Minute),
		Content: golog.QueryContent{SQL: "SELECT * FROM Users WHERE name LIKE 'a%'"}})
	s.Save(ctx, golog.Entry{ID: "3", Type: golog.TypeLog, CreatedAt: base.Add(2 * time.Minute),
		Content: golog.LogContent{Level: "INFO", Message: "100% done"}})
	s.Close()

	tests := []struct {
		name   string
		filter golog.Filter
		want   []string
	}{
		{"all newest first", golog.Filter{}, []string{"3", "2", "1"}},
		{"by type", golog.Filter{Type: golog.TypeQuery}, []string{"2"}},
		{"by request", golog.Filter{RequestID: "r1"}, []string{"2", "1"}},
		{"limit and offset", golog.Filter{Limit: 1, Offset: 1}, []string{"2"}},
		{"search case-insensitive", golog.Filter{Search: "users"}, []string{"2", "1"}},
		{"search escapes percent", golog.Filter{Search: "100%"}, []string{"3"}},
		{"search escapes underscore", golog.Filter{Search: "a_"}, []string{}},
		{"from inclusive", golog.Filter{From: base.Add(time.Minute)}, []string{"3", "2"}},
		{"to inclusive", golog.Filter{To: base.Add(time.Minute)}, []string{"2", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ids(t, s, tt.filter); !equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}

	e, ok, err := s.Get(ctx, "1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	var c golog.RequestContent
	if err := json.Unmarshal(e.Content.(json.RawMessage), &c); err != nil {
		t.Fatal(err)
	}
	if e.RequestID != "r1" || c.Path != "/users" || !e.CreatedAt.Equal(base) {
		t.Fatalf("unexpected entry: %+v %+v", e, c)
	}
	if _, ok, _ := s.Get(ctx, "missing"); ok {
		t.Fatal("missing entry should not be found")
	}
	if e, _, _ := s.Get(ctx, "3"); e.RequestID != "" {
		t.Fatalf("entry without request should have empty request ID, got %q", e.RequestID)
	}
}

func TestStoreFlushesInBackground(t *testing.T) {
	db, table := openDB(t)
	s := newStore(t, db, postgres.Options{Table: table})

	s.Save(context.Background(), golog.Entry{ID: "a", Type: golog.TypeLog, CreatedAt: time.Now(), Content: golog.LogContent{}})

	deadline := time.Now().Add(3 * time.Second)
	for len(ids(t, s, golog.Filter{})) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("entry was not flushed without Close")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestStoreRetentionAndClear(t *testing.T) {
	db, table := openDB(t)
	s := newStore(t, db, postgres.Options{Table: table})
	s.Save(context.Background(), golog.Entry{ID: "old", Type: golog.TypeLog, CreatedAt: time.Now().Add(-48 * time.Hour), Content: golog.LogContent{}})
	s.Save(context.Background(), golog.Entry{ID: "new", Type: golog.TypeLog, CreatedAt: time.Now(), Content: golog.LogContent{}})
	s.Close()

	s = newStore(t, db, postgres.Options{Table: table, Retention: 24 * time.Hour})
	s.Close()
	if got := ids(t, s, golog.Filter{}); !equal(got, []string{"new"}) {
		t.Fatalf("old entry should be pruned on start, got %v", got)
	}

	if err := s.Clear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := ids(t, s, golog.Filter{}); len(got) != 0 {
		t.Fatalf("expected empty table after Clear, got %v", got)
	}
}

func TestStoreSetRetention(t *testing.T) {
	db, table := openDB(t)
	ctx := context.Background()
	s := newStore(t, db, postgres.Options{Table: table, Retention: 24 * time.Hour})
	s.Save(ctx, golog.Entry{ID: "old", Type: golog.TypeLog, CreatedAt: time.Now().Add(-3 * time.Hour), Content: golog.LogContent{}})
	s.Save(ctx, golog.Entry{ID: "new", Type: golog.TypeLog, CreatedAt: time.Now(), Content: golog.LogContent{}})
	s.Close()

	if d, _ := s.Retention(ctx); d != 24*time.Hour {
		t.Fatalf("initial retention should come from Options, got %v", d)
	}
	if err := s.SetRetention(ctx, time.Hour); err != nil {
		t.Fatal(err)
	}
	if got := ids(t, s, golog.Filter{}); !equal(got, []string{"new"}) {
		t.Fatalf("SetRetention should prune right away, got %v", got)
	}

	s = newStore(t, db, postgres.Options{Table: table, Retention: 7 * 24 * time.Hour})
	if d, _ := s.Retention(ctx); d != time.Hour {
		t.Fatalf("saved retention should win over Options after restart, got %v", d)
	}

	if err := s.SetRetention(ctx, 0); err != nil {
		t.Fatal(err)
	}
	s.Save(ctx, golog.Entry{ID: "ancient", Type: golog.TypeLog, CreatedAt: time.Now().Add(-365 * 24 * time.Hour), Content: golog.LogContent{}})
	s.Close()
	s = newStore(t, db, postgres.Options{Table: table})
	s.Close()
	if got := ids(t, s, golog.Filter{}); !equal(got, []string{"new", "ancient"}) {
		t.Fatalf("retention 0 should disable pruning, got %v", got)
	}

	if err := s.SetRetention(ctx, -time.Hour); err == nil {
		t.Fatal("negative retention should be rejected")
	}
}

func TestNewRejectsInvalidTable(t *testing.T) {
	if _, err := postgres.New(nil, postgres.Options{Table: "entries; DROP TABLE users"}); err == nil {
		t.Fatal("expected error for invalid table name")
	}
}
