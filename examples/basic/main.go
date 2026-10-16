package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/kuhcokrow/golog"
)

func main() {
	g := golog.New(golog.Config{Path: "/golog"})

	db := sql.OpenDB(g.WrapDriver(fakeDriver{}, ""))
	logger := slog.New(g.SlogHandler(slog.NewTextHandler(os.Stdout, nil)))
	var cache sync.Map

	mux := http.NewServeMux()
	mux.Handle("/golog/", g.Handler())

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.PathValue("id")
		key := "user:" + id

		if u, ok := cache.Load(key); ok {
			g.RecordCache(ctx, golog.CacheHit, key, u)
			writeJSON(w, u)
			return
		}
		g.RecordCache(ctx, golog.CacheMiss, key, nil)

		var name string
		if err := db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = $1", id).Scan(&name); err != nil {
			logger.ErrorContext(ctx, "query user failed", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		u := map[string]string{"id": id, "name": name}
		cache.Store(key, u)
		g.RecordCache(ctx, golog.CacheSet, key, u)
		logger.InfoContext(ctx, "user loaded", "id", id)
		writeJSON(w, u)
	})

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			logger.WarnContext(ctx, "invalid body", "err", err)
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		db.ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", body.Name)
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]string{"name": body.Name})
	})

	mux.HandleFunc("GET /broken-query", func(w http.ResponseWriter, r *http.Request) {
		_, err := db.QueryContext(r.Context(), "SELEC * FROM users")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})

	mux.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("something went terribly wrong")
	})

	log.Println("listening on http://localhost:8080, dashboard at http://localhost:8080/golog")
	log.Fatal(http.ListenAndServe(":8080", g.Middleware(mux)))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// fakeDriver stands in for a real database so the example runs without setup.
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
	if query == "SELEC * FROM users" {
		return nil, errors.New(`syntax error at or near "SELEC"`)
	}
	return &fakeRows{}, nil
}

func (fakeStmt) Close() error                               { return nil }
func (fakeStmt) NumInput() int                              { return -1 }
func (fakeStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(1), nil }
func (fakeStmt) Query([]driver.Value) (driver.Rows, error)  { return &fakeRows{}, nil }
func (fakeTx) Commit() error                                { return nil }
func (fakeTx) Rollback() error                              { return nil }
func (*fakeRows) Columns() []string                         { return []string{"name"} }
func (*fakeRows) Close() error                              { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "Budi"
	return nil
}
