package tests

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"

	"github.com/kuhcokrow/golog"
)

type fakeDriver struct{}
type fakeConn struct{}
type fakeStmt struct{}
type fakeTx struct{}
type fakeRows struct{ done bool }

func (fakeDriver) Open(string) (driver.Conn, error)  { return fakeConn{}, nil }
func (fakeConn) Prepare(string) (driver.Stmt, error) { return fakeStmt{}, nil }
func (fakeConn) Close() error                        { return nil }
func (fakeConn) Begin() (driver.Tx, error)           { return fakeTx{}, nil }

func (fakeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if query == "BAD" {
		return nil, errors.New("syntax error")
	}
	return &fakeRows{}, nil
}

func (fakeStmt) Close() error                               { return nil }
func (fakeStmt) NumInput() int                              { return -1 }
func (fakeStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(1), nil }
func (fakeStmt) Query([]driver.Value) (driver.Rows, error)  { return &fakeRows{}, nil }
func (fakeTx) Commit() error                                { return nil }
func (fakeTx) Rollback() error                              { return nil }
func (*fakeRows) Columns() []string                         { return []string{"n"} }
func (*fakeRows) Close() error                              { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(1)
	return nil
}

func TestWrapDriverRecordsQueries(t *testing.T) {
	g, store := newGolog(golog.Config{})
	db := sql.OpenDB(g.WrapDriver(fakeDriver{}, ""))
	defer db.Close()
	ctx := golog.WithRequestID(context.Background(), "req-1")

	var n int
	if err := db.QueryRowContext(ctx, "SELECT n FROM t WHERE id = $1", 7).Scan(&n); err != nil {
		t.Fatal(err)
	}
	db.QueryContext(ctx, "BAD")
	if _, err := db.ExecContext(ctx, "UPDATE t SET name = $1", []byte("budi")); err != nil {
		t.Fatal(err)
	}
	tx, _ := db.BeginTx(ctx, nil)
	tx.ExecContext(ctx, "DELETE FROM t")
	tx.Commit()

	entries := list(t, store, golog.Filter{Type: golog.TypeQuery})
	if len(entries) != 4 {
		t.Fatalf("expected 4 queries, got %d", len(entries))
	}
	byQuery := map[string]golog.QueryContent{}
	for _, e := range entries {
		if e.RequestID != "req-1" {
			t.Fatalf("query should be linked to req-1, got %q", e.RequestID)
		}
		c := e.Content.(golog.QueryContent)
		byQuery[c.SQL] = c
	}

	if b := byQuery["SELECT n FROM t WHERE id = $1"].Bindings; len(b) != 1 || b[0] != int64(7) {
		t.Fatalf("unexpected bindings %v", b)
	}
	if e := byQuery["BAD"].Error; e != "syntax error" {
		t.Fatalf("expected error to be recorded, got %q", e)
	}
	if b := byQuery["UPDATE t SET name = $1"].Bindings; len(b) != 1 || b[0] != "budi" {
		t.Fatalf("[]byte binding should be stored as string, got %v", b)
	}
	if _, ok := byQuery["DELETE FROM t"]; !ok {
		t.Fatal("query inside transaction should be recorded")
	}
}

func TestWrapDriverWithoutRequest(t *testing.T) {
	g, store := newGolog(golog.Config{})
	db := sql.OpenDB(g.WrapDriver(fakeDriver{}, ""))
	defer db.Close()

	rows, err := db.Query("SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()

	entries := list(t, store, golog.Filter{Type: golog.TypeQuery})
	if len(entries) != 1 || entries[0].RequestID != "" {
		t.Fatalf("expected 1 unlinked query, got %+v", entries)
	}
}
