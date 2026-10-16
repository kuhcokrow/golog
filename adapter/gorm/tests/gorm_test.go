package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/kuhcokrow/golog"
	gologgorm "github.com/kuhcokrow/golog/adapter/gorm"
)

// Needs a running PostgreSQL, e.g.
// GOLOG_TEST_POSTGRES_DSN=postgres://user:pass@localhost:5432/db go test ./tests/
func TestPluginRecordsQueries(t *testing.T) {
	dsn := os.Getenv("GOLOG_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GOLOG_TEST_POSTGRES_DSN is not set")
	}

	store := golog.NewMemoryStore(100)
	g := golog.New(golog.Config{Store: store})
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Use(gologgorm.New(g)); err != nil {
		t.Fatal(err)
	}

	table := fmt.Sprintf("golog_gorm_test_%d", time.Now().UnixNano())
	type User struct {
		ID   uint
		Name string
	}
	tdb := db.Table(table)
	if err := tdb.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Migrator().DropTable(table) })

	ctx := golog.WithRequestID(context.Background(), "req-1")
	q := db.WithContext(ctx).Table(table)
	q.Create(&User{Name: "budi"})
	var u User
	q.Where("name = ?", "budi").First(&u)
	db.WithContext(ctx).Table(table).Where("name = ?", "nobody").First(&User{})
	db.WithContext(ctx).Exec("SELEC 1")

	entries, _ := store.List(context.Background(), golog.Filter{Type: golog.TypeQuery, RequestID: "req-1"})
	if len(entries) != 4 {
		t.Fatalf("expected 4 linked queries, got %d", len(entries))
	}

	var insert, notFound, broken *golog.QueryContent
	for i := range entries {
		c := entries[i].Content.(golog.QueryContent)
		switch {
		case strings.HasPrefix(c.SQL, "INSERT"):
			insert = &c
		case strings.Contains(c.SQL, "SELEC 1"):
			broken = &c
		case len(c.Bindings) > 0 && c.Bindings[0] == "nobody":
			notFound = &c
		}
	}
	if insert == nil || len(insert.Bindings) == 0 || insert.Bindings[0] != "budi" {
		t.Fatalf("insert should be recorded with bindings, got %+v", insert)
	}
	if notFound == nil || notFound.Error != "" {
		t.Fatalf("record not found should not be recorded as an error, got %+v", notFound)
	}
	if broken == nil || broken.Error == "" {
		t.Fatalf("broken query should be recorded with its error, got %+v", broken)
	}
}
