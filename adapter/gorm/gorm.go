// Package gologgorm records GORM queries in golog.
package gologgorm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/kuhcokrow/golog"
)

const startKey = "golog:start"

// Plugin records every query run through GORM. Do not combine it with a
// *sql.DB wrapped by golog.WrapDriver, or each query is recorded twice.
// Queries are linked to a request only when run with db.WithContext(ctx).
type Plugin struct {
	g *golog.Golog
}

func New(g *golog.Golog) *Plugin {
	return &Plugin{g: g}
}

func (p *Plugin) Name() string {
	return "golog"
}

func (p *Plugin) Initialize(db *gorm.DB) error {
	cb := db.Callback()
	return errors.Join(
		cb.Create().Before("gorm:create").Register("golog:before", before),
		cb.Create().After("gorm:create").Register("golog:after", p.after),
		cb.Query().Before("gorm:query").Register("golog:before", before),
		cb.Query().After("gorm:query").Register("golog:after", p.after),
		cb.Update().Before("gorm:update").Register("golog:before", before),
		cb.Update().After("gorm:update").Register("golog:after", p.after),
		cb.Delete().Before("gorm:delete").Register("golog:before", before),
		cb.Delete().After("gorm:delete").Register("golog:after", p.after),
		cb.Row().Before("gorm:row").Register("golog:before", before),
		cb.Row().After("gorm:row").Register("golog:after", p.after),
		cb.Raw().Before("gorm:raw").Register("golog:before", before),
		cb.Raw().After("gorm:raw").Register("golog:after", p.after),
	)
}

func before(db *gorm.DB) {
	db.InstanceSet(startKey, time.Now())
}

func (p *Plugin) after(db *gorm.DB) {
	v, ok := db.InstanceGet(startKey)
	if !ok {
		return
	}
	start := v.(time.Time)

	c := golog.QueryContent{
		SQL:        db.Statement.SQL.String(),
		Bindings:   db.Statement.Vars,
		DurationMs: float64(time.Since(start).Microseconds()) / 1000,
	}
	if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
		c.Error = db.Error.Error()
	}

	ctx := db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	p.g.Record(ctx, golog.TypeQuery, c)
}
