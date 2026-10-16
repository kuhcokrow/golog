package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kuhcokrow/golog"
	gologredis "github.com/kuhcokrow/golog/adapter/redis"
)

// Needs a running Redis, e.g.
// GOLOG_TEST_REDIS_ADDR=localhost:6379 go test ./tests/
func TestHookRecordsCommands(t *testing.T) {
	addr := os.Getenv("GOLOG_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("GOLOG_TEST_REDIS_ADDR is not set")
	}

	store := golog.NewMemoryStore(100)
	g := golog.New(golog.Config{Store: store})
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	rdb.AddHook(gologredis.New(g))
	t.Cleanup(func() { rdb.Close() })

	key := fmt.Sprintf("golog:test:%d", time.Now().UnixNano())
	missing := key + ":missing"
	ctx := golog.WithRequestID(context.Background(), "req-1")

	rdb.Set(ctx, key, "budi", time.Minute)
	rdb.Get(ctx, key)
	rdb.Get(ctx, missing)
	pipe := rdb.Pipeline()
	pipe.Incr(ctx, key+":n")
	pipe.Del(ctx, key, key+":n")
	pipe.Exec(ctx)

	entries, _ := store.List(context.Background(), golog.Filter{Type: golog.TypeCache, RequestID: "req-1"})
	got := make([]string, len(entries))
	for i, e := range entries {
		c := e.Content.(golog.CacheContent)
		got[len(entries)-1-i] = fmt.Sprintf("%s %s %v", c.Action, c.Key, c.Value)
	}

	want := []string{
		"set " + key + " budi",
		"hit " + key + " budi",
		"miss " + missing + " <nil>",
		"incr " + key + ":n <nil>",
		"delete " + key + " <nil>",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
