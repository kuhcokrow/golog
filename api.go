package golog

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/kuhcokrow/golog/internal/dashboard"
)

// Handler serves the dashboard and its API. Mount it at Config.Path, e.g.
// mux.Handle("/golog/", g.Handler()).
func (g *Golog) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/entries", g.listEntries)
	mux.HandleFunc("GET /api/entries/{id}", g.getEntry)
	mux.HandleFunc("DELETE /api/entries", g.clearEntries)
	mux.Handle("GET /", http.FileServerFS(dashboard.FS()))

	h := http.StripPrefix(g.cfg.Path, mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.cfg.Authorize != nil && !g.cfg.Authorize(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (g *Golog) listEntries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))

	entries, err := g.cfg.Store.List(r.Context(), Filter{
		Type:      EntryType(q.Get("type")),
		RequestID: q.Get("requestId"),
		Limit:     limit,
		Offset:    max(offset, 0),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, entries)
}

func (g *Golog) getEntry(w http.ResponseWriter, r *http.Request) {
	e, ok, err := g.cfg.Store.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, e)
}

func (g *Golog) clearEntries(w http.ResponseWriter, r *http.Request) {
	if err := g.cfg.Store.Clear(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
