package tests

import (
	"context"
	"testing"
	"time"

	"github.com/kuhcokrow/golog"
)

func TestMemoryStoreOverwritesOldest(t *testing.T) {
	s := golog.NewMemoryStore(3)
	for _, id := range []string{"a", "b", "c", "d"} {
		s.Save(context.Background(), golog.Entry{ID: id})
	}

	got := ids(t, s, golog.Filter{})
	want := []string{"d", "c", "b"}
	if !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, ok, _ := s.Get(context.Background(), "a"); ok {
		t.Fatal("oldest entry should have been overwritten")
	}
}

func TestMemoryStoreFilter(t *testing.T) {
	s := golog.NewMemoryStore(10)
	s.Save(context.Background(), golog.Entry{ID: "1", Type: golog.TypeRequest, RequestID: "r1"})
	s.Save(context.Background(), golog.Entry{ID: "2", Type: golog.TypeQuery, RequestID: "r1"})
	s.Save(context.Background(), golog.Entry{ID: "3", Type: golog.TypeQuery, RequestID: "r2"})
	s.Save(context.Background(), golog.Entry{ID: "4", Type: golog.TypeQuery, RequestID: "r1"})

	tests := []struct {
		name   string
		filter golog.Filter
		want   []string
	}{
		{"by type", golog.Filter{Type: golog.TypeQuery}, []string{"4", "3", "2"}},
		{"by request", golog.Filter{RequestID: "r1"}, []string{"4", "2", "1"}},
		{"type and request", golog.Filter{Type: golog.TypeQuery, RequestID: "r1"}, []string{"4", "2"}},
		{"limit", golog.Filter{Limit: 2}, []string{"4", "3"}},
		{"offset", golog.Filter{Limit: 2, Offset: 2}, []string{"2", "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ids(t, s, tt.filter); !equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryStoreSearchAndTimeRange(t *testing.T) {
	s := golog.NewMemoryStore(10)
	base := time.Date(2026, 10, 16, 13, 0, 0, 0, time.UTC)
	s.Save(context.Background(), golog.Entry{ID: "1", CreatedAt: base, Content: golog.RequestContent{Path: "/users"}})
	s.Save(context.Background(), golog.Entry{ID: "2", CreatedAt: base.Add(30 * time.Minute), Content: golog.QueryContent{SQL: "SELECT * FROM Users"}})
	s.Save(context.Background(), golog.Entry{ID: "3", CreatedAt: base.Add(time.Hour), Content: golog.LogContent{Message: "order created"}})

	tests := []struct {
		name   string
		filter golog.Filter
		want   []string
	}{
		{"search is case-insensitive", golog.Filter{Search: "USERS"}, []string{"2", "1"}},
		{"search no match", golog.Filter{Search: "invoice"}, []string{}},
		{"from inclusive", golog.Filter{From: base.Add(30 * time.Minute)}, []string{"3", "2"}},
		{"to inclusive", golog.Filter{To: base.Add(30 * time.Minute)}, []string{"2", "1"}},
		{"range and search", golog.Filter{From: base.Add(time.Minute), Search: "users"}, []string{"2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ids(t, s, tt.filter); !equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMemoryStoreRetention(t *testing.T) {
	s := golog.NewMemoryStore(10)
	ctx := context.Background()
	s.Save(ctx, golog.Entry{ID: "old", CreatedAt: time.Now().Add(-2 * time.Hour)})
	s.Save(ctx, golog.Entry{ID: "new", CreatedAt: time.Now()})

	if d, _ := s.Retention(ctx); d != 0 {
		t.Fatalf("default retention should be 0, got %v", d)
	}
	s.SetRetention(ctx, time.Hour)
	if got := ids(t, s, golog.Filter{}); !equal(got, []string{"new"}) {
		t.Fatalf("expired entry should be hidden, got %v", got)
	}
	if _, ok, _ := s.Get(ctx, "old"); ok {
		t.Fatal("expired entry should not be found")
	}
	s.SetRetention(ctx, 0)
	if got := ids(t, s, golog.Filter{}); !equal(got, []string{"new", "old"}) {
		t.Fatalf("retention 0 should show every entry, got %v", got)
	}
	if err := s.SetRetention(ctx, -time.Hour); err == nil {
		t.Fatal("negative retention should be rejected")
	}
}

func TestMemoryStoreClear(t *testing.T) {
	s := golog.NewMemoryStore(3)
	s.Save(context.Background(), golog.Entry{ID: "a"})
	s.Clear(context.Background())

	if got := ids(t, s, golog.Filter{}); len(got) != 0 {
		t.Fatalf("expected no entries, got %v", got)
	}
}
