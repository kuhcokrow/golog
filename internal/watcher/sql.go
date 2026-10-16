package watcher

import (
	"context"
	"database/sql/driver"
	"errors"
	"time"

	"github.com/kuhcokrow/golog/internal/entry"
)

func WrapDriver(save func(entry.Entry), d driver.Driver, dsn string) driver.Connector {
	var c driver.Connector = dsnConnector{d: d, dsn: dsn}
	if dc, ok := d.(driver.DriverContext); ok {
		if oc, err := dc.OpenConnector(dsn); err == nil {
			c = oc
		}
	}
	return &connector{save: save, c: c, d: d}
}

type dsnConnector struct {
	d   driver.Driver
	dsn string
}

func (c dsnConnector) Connect(context.Context) (driver.Conn, error) { return c.d.Open(c.dsn) }
func (c dsnConnector) Driver() driver.Driver                        { return c.d }

type connector struct {
	save func(entry.Entry)
	c    driver.Connector
	d    driver.Driver
}

func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &wrappedConn{save: c.save, Conn: conn}, nil
}

func (c *connector) Driver() driver.Driver { return c.d }

func recordQuery(save func(entry.Entry), ctx context.Context, query string, args []driver.NamedValue, start time.Time, err error) {
	bindings := make([]any, len(args))
	for i, a := range args {
		if b, ok := a.Value.([]byte); ok {
			bindings[i] = string(b)
		} else {
			bindings[i] = a.Value
		}
	}
	c := entry.QueryContent{
		SQL:        query,
		Bindings:   bindings,
		DurationMs: float64(time.Since(start).Microseconds()) / 1000,
	}
	if err != nil {
		c.Error = err.Error()
	}
	save(entry.New(ctx, entry.TypeQuery, c))
}

type wrappedConn struct {
	driver.Conn
	save func(entry.Entry)
}

func (c *wrappedConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *wrappedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var stmt driver.Stmt
	var err error
	if p, ok := c.Conn.(driver.ConnPrepareContext); ok {
		stmt, err = p.PrepareContext(ctx, query)
	} else {
		stmt, err = c.Conn.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	return &wrappedStmt{Stmt: stmt, save: c.save, query: query}, nil
}

func (c *wrappedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if b, ok := c.Conn.(driver.ConnBeginTx); ok {
		return b.BeginTx(ctx, opts)
	}
	return c.Conn.Begin()
}

func (c *wrappedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	e, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	res, err := e.ExecContext(ctx, query, args)
	if !errors.Is(err, driver.ErrSkip) {
		recordQuery(c.save, ctx, query, args, start, err)
	}
	return res, err
}

func (c *wrappedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	q, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	rows, err := q.QueryContext(ctx, query, args)
	if !errors.Is(err, driver.ErrSkip) {
		recordQuery(c.save, ctx, query, args, start, err)
	}
	return rows, err
}

func (c *wrappedConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}

func (c *wrappedConn) ResetSession(ctx context.Context) error {
	if r, ok := c.Conn.(driver.SessionResetter); ok {
		return r.ResetSession(ctx)
	}
	return nil
}

func (c *wrappedConn) IsValid() bool {
	if v, ok := c.Conn.(driver.Validator); ok {
		return v.IsValid()
	}
	return true
}

func (c *wrappedConn) CheckNamedValue(nv *driver.NamedValue) error {
	if n, ok := c.Conn.(driver.NamedValueChecker); ok {
		return n.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

type wrappedStmt struct {
	driver.Stmt
	save  func(entry.Entry)
	query string
}

func (s *wrappedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	start := time.Now()
	var res driver.Result
	var err error
	if e, ok := s.Stmt.(driver.StmtExecContext); ok {
		res, err = e.ExecContext(ctx, args)
	} else {
		res, err = s.Stmt.Exec(toValues(args))
	}
	recordQuery(s.save, ctx, s.query, args, start, err)
	return res, err
}

func (s *wrappedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	start := time.Now()
	var rows driver.Rows
	var err error
	if q, ok := s.Stmt.(driver.StmtQueryContext); ok {
		rows, err = q.QueryContext(ctx, args)
	} else {
		rows, err = s.Stmt.Query(toValues(args))
	}
	recordQuery(s.save, ctx, s.query, args, start, err)
	return rows, err
}

func (s *wrappedStmt) CheckNamedValue(nv *driver.NamedValue) error {
	if n, ok := s.Stmt.(driver.NamedValueChecker); ok {
		return n.CheckNamedValue(nv)
	}
	return driver.ErrSkip
}

func toValues(args []driver.NamedValue) []driver.Value {
	v := make([]driver.Value, len(args))
	for i, a := range args {
		v[i] = a.Value
	}
	return v
}
