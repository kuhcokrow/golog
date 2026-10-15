package golog

import "context"

// RecordCache logs a cache operation. Value may be nil, e.g. for a miss or delete.
func (g *Golog) RecordCache(ctx context.Context, action CacheAction, key string, value any) {
	g.Record(ctx, TypeCache, CacheContent{Action: action, Key: key, Value: value})
}
