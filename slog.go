package golog

import (
	"context"
	"log/slog"
	"maps"
)

// SlogHandler records every log entry and then passes it to next.
func (g *Golog) SlogHandler(next slog.Handler) slog.Handler {
	return &slogHandler{g: g, next: next, attrs: map[string]any{}}
}

type slogHandler struct {
	g      *Golog
	next   slog.Handler
	attrs  map[string]any
	prefix string
}

func (h *slogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *slogHandler) Handle(ctx context.Context, r slog.Record) error {
	attrs := maps.Clone(h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		addAttr(attrs, h.prefix, a)
		return true
	})
	h.g.Record(ctx, TypeLog, LogContent{
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   attrs,
	})
	return h.next.Handle(ctx, r)
}

func (h *slogHandler) WithAttrs(as []slog.Attr) slog.Handler {
	attrs := maps.Clone(h.attrs)
	for _, a := range as {
		addAttr(attrs, h.prefix, a)
	}
	return &slogHandler{g: h.g, next: h.next.WithAttrs(as), attrs: attrs, prefix: h.prefix}
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &slogHandler{g: h.g, next: h.next.WithGroup(name), attrs: h.attrs, prefix: h.prefix + name + "."}
}

func addAttr(m map[string]any, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, sub := range a.Value.Group() {
			addAttr(m, prefix, sub)
		}
		return
	}
	v := a.Value.Any()
	if err, ok := v.(error); ok {
		v = err.Error()
	}
	m[prefix+a.Key] = v
}
