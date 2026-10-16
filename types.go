package golog

import (
	"context"

	"github.com/kuhcokrow/golog/internal/entry"
	"github.com/kuhcokrow/golog/internal/store"
)

type (
	Entry            = entry.Entry
	EntryType        = entry.EntryType
	RequestContent   = entry.RequestContent
	QueryContent     = entry.QueryContent
	LogContent       = entry.LogContent
	ExceptionContent = entry.ExceptionContent
	CacheContent     = entry.CacheContent
	CacheAction      = entry.CacheAction

	Store       = store.Store
	Filter      = store.Filter
	MemoryStore = store.Memory
)

const (
	TypeRequest   = entry.TypeRequest
	TypeQuery     = entry.TypeQuery
	TypeLog       = entry.TypeLog
	TypeException = entry.TypeException
	TypeCache     = entry.TypeCache

	CacheHit    = entry.CacheHit
	CacheMiss   = entry.CacheMiss
	CacheSet    = entry.CacheSet
	CacheDelete = entry.CacheDelete
)

func NewMemoryStore(max int) *MemoryStore {
	return store.NewMemory(max)
}

func NewRequestID() string {
	return entry.NewID()
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return entry.WithRequestID(ctx, id)
}

func RequestIDFrom(ctx context.Context) string {
	return entry.RequestIDFrom(ctx)
}
