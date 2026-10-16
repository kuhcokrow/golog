package store

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/kuhcokrow/golog/internal/entry"
)

type Filter struct {
	Type      entry.EntryType
	RequestID string
	// Search matches entry content case-insensitively.
	Search string
	// From and To bound CreatedAt inclusively. Zero values are ignored.
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
}

func (f Filter) matches(e entry.Entry) bool {
	if f.Type != "" && e.Type != f.Type {
		return false
	}
	if f.RequestID != "" && e.RequestID != f.RequestID {
		return false
	}
	if !f.From.IsZero() && e.CreatedAt.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && e.CreatedAt.After(f.To) {
		return false
	}
	if f.Search != "" {
		b, _ := json.Marshal(e.Content)
		return strings.Contains(strings.ToLower(string(b)), strings.ToLower(f.Search))
	}
	return true
}

// Store results from List must be ordered newest first.
type Store interface {
	Save(ctx context.Context, e entry.Entry) error
	List(ctx context.Context, f Filter) ([]entry.Entry, error)
	Get(ctx context.Context, id string) (entry.Entry, bool, error)
	Clear(ctx context.Context) error
}

type Memory struct {
	mu      sync.RWMutex
	entries []entry.Entry
	next    int
	full    bool
}

func NewMemory(max int) *Memory {
	return &Memory{entries: make([]entry.Entry, max)}
}

func (s *Memory) Save(_ context.Context, e entry.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[s.next] = e
	s.next = (s.next + 1) % len(s.entries)
	if s.next == 0 {
		s.full = true
	}
	return nil
}

func (s *Memory) each(fn func(entry.Entry) bool) {
	n := s.next
	if s.full {
		n = len(s.entries)
	}
	for i := 1; i <= n; i++ {
		idx := (s.next - i + len(s.entries)) % len(s.entries)
		if !fn(s.entries[idx]) {
			return
		}
	}
}

func (s *Memory) List(_ context.Context, f Filter) ([]entry.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []entry.Entry{}
	skipped := 0
	s.each(func(e entry.Entry) bool {
		if !f.matches(e) {
			return true
		}
		if skipped < f.Offset {
			skipped++
			return true
		}
		result = append(result, e)
		return f.Limit <= 0 || len(result) < f.Limit
	})
	return result, nil
}

func (s *Memory) Get(_ context.Context, id string) (entry.Entry, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found entry.Entry
	ok := false
	s.each(func(e entry.Entry) bool {
		if e.ID == id {
			found, ok = e, true
			return false
		}
		return true
	})
	return found, ok, nil
}

func (s *Memory) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make([]entry.Entry, len(s.entries))
	s.next, s.full = 0, false
	return nil
}
