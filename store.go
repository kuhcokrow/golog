package golog

import (
	"context"
	"sync"
)

type Filter struct {
	Type      EntryType
	RequestID string
	Limit     int
	Offset    int
}

// Store results from List must be ordered newest first.
type Store interface {
	Save(ctx context.Context, e Entry) error
	List(ctx context.Context, f Filter) ([]Entry, error)
	Get(ctx context.Context, id string) (Entry, bool, error)
	Clear(ctx context.Context) error
}

type MemoryStore struct {
	mu      sync.RWMutex
	entries []Entry
	next    int
	full    bool
}

func NewMemoryStore(max int) *MemoryStore {
	return &MemoryStore{entries: make([]Entry, max)}
}

func (s *MemoryStore) Save(_ context.Context, e Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[s.next] = e
	s.next = (s.next + 1) % len(s.entries)
	if s.next == 0 {
		s.full = true
	}
	return nil
}

func (s *MemoryStore) each(fn func(Entry) bool) {
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

func (s *MemoryStore) List(_ context.Context, f Filter) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []Entry{}
	skipped := 0
	s.each(func(e Entry) bool {
		if (f.Type != "" && e.Type != f.Type) || (f.RequestID != "" && e.RequestID != f.RequestID) {
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

func (s *MemoryStore) Get(_ context.Context, id string) (Entry, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found Entry
	ok := false
	s.each(func(e Entry) bool {
		if e.ID == id {
			found, ok = e, true
			return false
		}
		return true
	})
	return found, ok, nil
}

func (s *MemoryStore) Clear(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make([]Entry, len(s.entries))
	s.next, s.full = 0, false
	return nil
}
