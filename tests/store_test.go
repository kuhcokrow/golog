package tests

import (
	"context"
	"testing"

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

func TestMemoryStoreClear(t *testing.T) {
	s := golog.NewMemoryStore(3)
	s.Save(context.Background(), golog.Entry{ID: "a"})
	s.Clear(context.Background())

	if got := ids(t, s, golog.Filter{}); len(got) != 0 {
		t.Fatalf("expected no entries, got %v", got)
	}
}
