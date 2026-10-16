package entry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

type EntryType string

const (
	TypeRequest   EntryType = "request"
	TypeQuery     EntryType = "query"
	TypeLog       EntryType = "log"
	TypeException EntryType = "exception"
	TypeCache     EntryType = "cache"
)

type Entry struct {
	ID        string    `json:"id"`
	Type      EntryType `json:"type"`
	RequestID string    `json:"requestId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Content   any       `json:"content"`
}

type RequestContent struct {
	Method          string      `json:"method"`
	Path            string      `json:"path"`
	Query           string      `json:"query,omitempty"`
	IP              string      `json:"ip"`
	Status          int         `json:"status"`
	DurationMs      float64     `json:"durationMs"`
	Headers         http.Header `json:"headers"`
	Body            string      `json:"body,omitempty"`
	ResponseHeaders http.Header `json:"responseHeaders"`
	ResponseBody    string      `json:"responseBody,omitempty"`
}

type ExceptionContent struct {
	Message string `json:"message"`
	Stack   string `json:"stack"`
}

// New builds an entry linked to the request ID found in ctx, if any.
func New(ctx context.Context, t EntryType, content any) Entry {
	return Entry{
		ID:        NewID(),
		Type:      t,
		RequestID: RequestIDFrom(ctx),
		CreatedAt: time.Now(),
		Content:   content,
	}
}

func NewID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type QueryContent struct {
	SQL        string  `json:"sql"`
	Bindings   []any   `json:"bindings"`
	DurationMs float64 `json:"durationMs"`
	Error      string  `json:"error,omitempty"`
}

type LogContent struct {
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
}

type CacheAction string

const (
	CacheHit    CacheAction = "hit"
	CacheMiss   CacheAction = "miss"
	CacheSet    CacheAction = "set"
	CacheDelete CacheAction = "delete"
)

type CacheContent struct {
	Action     CacheAction `json:"action"`
	Key        string      `json:"key"`
	Value      any         `json:"value,omitempty"`
	DurationMs float64     `json:"durationMs,omitempty"`
}
