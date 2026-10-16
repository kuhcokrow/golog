// Package gologredis records go-redis v9 commands as golog cache entries.
package gologredis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kuhcokrow/golog"
)

var ignored = map[string]bool{"hello": true, "client": true, "auth": true, "select": true, "ping": true}

// Hook records every command that has a key. GET is recorded as a hit or
// miss, SET as set and DEL as delete; other commands use their own name.
// Use it with rdb.AddHook(gologredis.New(g)).
type Hook struct {
	g *golog.Golog
}

func New(g *golog.Golog) *Hook {
	return &Hook{g: g}
}

func (h *Hook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *Hook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		h.record(ctx, cmd, start)
		return err
	}
}

func (h *Hook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		for _, cmd := range cmds {
			h.record(ctx, cmd, start)
		}
		return err
	}
}

func (h *Hook) record(ctx context.Context, cmd redis.Cmder, start time.Time) {
	args := cmd.Args()
	name := cmd.Name()
	if len(args) < 2 || ignored[name] {
		return
	}

	c := golog.CacheContent{
		Key:        fmt.Sprint(args[1]),
		DurationMs: float64(time.Since(start).Microseconds()) / 1000,
	}
	switch name {
	case "get":
		if errors.Is(cmd.Err(), redis.Nil) {
			c.Action = golog.CacheMiss
		} else {
			c.Action = golog.CacheHit
			if s, ok := cmd.(*redis.StringCmd); ok {
				c.Value = s.Val()
			}
		}
	case "set", "setnx":
		c.Action = golog.CacheSet
		if len(args) > 2 {
			c.Value = value(args[2])
		}
	case "setex":
		c.Action = golog.CacheSet
		if len(args) > 3 {
			c.Value = value(args[3])
		}
	case "del", "unlink":
		c.Action = golog.CacheDelete
	default:
		c.Action = golog.CacheAction(name)
	}
	h.g.Record(ctx, golog.TypeCache, c)
}

func value(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}
