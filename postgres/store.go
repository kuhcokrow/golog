// Package postgres provides a golog.Store backed by PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/kuhcokrow/golog/internal/entry"
	"github.com/kuhcokrow/golog/internal/store"
)

const (
	queueSize     = 1000
	batchSize     = 100
	flushInterval = 500 * time.Millisecond
	pruneInterval = time.Minute
)

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Options struct {
	// Table defaults to "golog_entries". It is created if it does not exist.
	Table string
	// Retention is how long entries are kept. Defaults to 24 hours.
	Retention time.Duration
}

// Store writes entries asynchronously in batches, so saving never blocks a
// request. Entries become visible to List shortly after they are saved, and
// are dropped if the write queue is full.
type Store struct {
	db        *sql.DB
	table     string
	retention time.Duration
	queue     chan entry.Entry
	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// New creates the table if needed and starts the background writer. Pass a
// *sql.DB that is not wrapped by golog, otherwise golog records its own
// queries. Call Close before the program exits to flush pending entries.
func New(db *sql.DB, opts Options) (*Store, error) {
	if opts.Table == "" {
		opts.Table = "golog_entries"
	}
	if !identifier.MatchString(opts.Table) {
		return nil, fmt.Errorf("postgres: invalid table name %q", opts.Table)
	}
	if opts.Retention <= 0 {
		opts.Retention = 24 * time.Hour
	}

	s := &Store{
		db:        db,
		table:     opts.Table,
		retention: opts.Retention,
		queue:     make(chan entry.Entry, queueSize),
		done:      make(chan struct{}),
	}
	if err := s.migrate(context.Background()); err != nil {
		return nil, err
	}

	s.wg.Add(1)
	go s.run()
	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id         TEXT PRIMARY KEY,
			type       TEXT NOT NULL,
			request_id TEXT,
			created_at TIMESTAMPTZ NOT NULL,
			content    JSONB NOT NULL
		)`, s.table),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %[1]s_created_at_idx ON %[1]s (created_at DESC)`, s.table),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %[1]s_type_created_at_idx ON %[1]s (type, created_at DESC)`, s.table),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %[1]s_request_id_idx ON %[1]s (request_id)`, s.table),
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("postgres: migrate: %w", err)
		}
	}
	return nil
}

// Close flushes pending entries and stops the background writer. It does not
// close the *sql.DB.
func (s *Store) Close() error {
	s.closeOnce.Do(func() { close(s.done) })
	s.wg.Wait()
	return nil
}

func (s *Store) Save(_ context.Context, e entry.Entry) error {
	select {
	case s.queue <- e:
	default:
	}
	return nil
}

func (s *Store) run() {
	defer s.wg.Done()
	flushTicker := time.NewTicker(flushInterval)
	defer flushTicker.Stop()
	pruneTicker := time.NewTicker(pruneInterval)
	defer pruneTicker.Stop()

	batch := make([]entry.Entry, 0, batchSize)
	add := func(e entry.Entry) {
		batch = append(batch, e)
		if len(batch) >= batchSize {
			s.insert(batch)
			batch = batch[:0]
		}
	}

	s.prune()
	for {
		select {
		case e := <-s.queue:
			add(e)
		case <-flushTicker.C:
			s.insert(batch)
			batch = batch[:0]
		case <-pruneTicker.C:
			s.prune()
		case <-s.done:
			for {
				select {
				case e := <-s.queue:
					add(e)
				default:
					s.insert(batch)
					return
				}
			}
		}
	}
}

// Write errors are dropped on purpose, like everywhere else in golog.
func (s *Store) insert(batch []entry.Entry) {
	if len(batch) == 0 {
		return
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "INSERT INTO %s (id, type, request_id, created_at, content) VALUES ", s.table)
	args := make([]any, 0, len(batch)*5)
	for i, e := range batch {
		content, err := json.Marshal(e.Content)
		if err != nil {
			content = []byte("null")
		}
		var requestID any
		if e.RequestID != "" {
			requestID = e.RequestID
		}
		if i > 0 {
			sb.WriteString(", ")
		}
		n := i * 5
		fmt.Fprintf(&sb, "($%d, $%d, $%d, $%d, $%d)", n+1, n+2, n+3, n+4, n+5)
		args = append(args, e.ID, string(e.Type), requestID, e.CreatedAt, string(content))
	}
	sb.WriteString(" ON CONFLICT (id) DO NOTHING")
	s.db.Exec(sb.String(), args...)
}

func (s *Store) prune() {
	s.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE created_at < $1", s.table), time.Now().Add(-s.retention))
}

func (s *Store) List(ctx context.Context, f store.Filter) ([]entry.Entry, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(cond, len(args)))
	}
	if f.Type != "" {
		add("type = $%d", string(f.Type))
	}
	if f.RequestID != "" {
		add("request_id = $%d", f.RequestID)
	}
	if !f.From.IsZero() {
		add("created_at >= $%d", f.From)
	}
	if !f.To.IsZero() {
		add("created_at <= $%d", f.To)
	}
	if f.Search != "" {
		add("content::text ILIKE $%d", "%"+escapeLike(f.Search)+"%")
	}

	query := fmt.Sprintf("SELECT id, type, request_id, created_at, content FROM %s", s.table)
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY created_at DESC, id DESC"
	if f.Limit > 0 {
		args = append(args, f.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if f.Offset > 0 {
		args = append(args, f.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []entry.Entry{}
	for rows.Next() {
		e, err := scan(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (entry.Entry, bool, error) {
	row := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT id, type, request_id, created_at, content FROM %s WHERE id = $1", s.table), id)
	e, err := scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return entry.Entry{}, false, nil
	}
	if err != nil {
		return entry.Entry{}, false, err
	}
	return e, true, nil
}

func (s *Store) Clear(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", s.table))
	return err
}

func scan(row interface{ Scan(...any) error }) (entry.Entry, error) {
	var e entry.Entry
	var requestID sql.NullString
	var content []byte
	if err := row.Scan(&e.ID, &e.Type, &requestID, &e.CreatedAt, &content); err != nil {
		return entry.Entry{}, err
	}
	e.RequestID = requestID.String
	e.Content = json.RawMessage(content)
	return e, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
