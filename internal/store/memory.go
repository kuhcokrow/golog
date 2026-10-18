package store

import (
	"context"
	"encoding/json"
	"errors"
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

// RetentionStore is a Store whose retention can be changed at runtime, e.g.
// from the dashboard. A retention of 0 keeps entries without a time limit.
type RetentionStore interface {
	Store
	Retention(ctx context.Context) (time.Duration, error)
	SetRetention(ctx context.Context, d time.Duration) error
}

// Memory keeps the latest entries up to its capacity. Entries older than its
// retention are hidden and eventually overwritten.
type Memory struct {
	mu        sync.RWMutex
	entries   []entry.Entry
	next      int
	full      bool
	retention time.Duration
	memoryUsers
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
	var cutoff time.Time
	if s.retention > 0 {
		cutoff = time.Now().Add(-s.retention)
	}
	for i := 1; i <= n; i++ {
		e := s.entries[(s.next-i+len(s.entries))%len(s.entries)]
		if !cutoff.IsZero() && e.CreatedAt.Before(cutoff) {
			continue
		}
		if !fn(e) {
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

func (s *Memory) Retention(context.Context) (time.Duration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.retention, nil
}

func (s *Memory) SetRetention(_ context.Context, d time.Duration) error {
	if d < 0 {
		return errors.New("retention must not be negative")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retention = d
	return nil
}
