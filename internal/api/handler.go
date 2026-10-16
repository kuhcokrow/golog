package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/kuhcokrow/golog/internal/dashboard"
	"github.com/kuhcokrow/golog/internal/entry"
	"github.com/kuhcokrow/golog/internal/store"
)

type handler struct {
	store store.Store
}

// Handler serves the dashboard and its JSON API under path.
func Handler(path string, s store.Store, authorize func(*http.Request) bool) http.Handler {
	g := &handler{store: s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/entries", g.listEntries)
	mux.HandleFunc("GET /api/entries/{id}", g.getEntry)
	mux.HandleFunc("DELETE /api/entries", g.clearEntries)
	mux.HandleFunc("GET /api/settings", g.getSettings)
	mux.HandleFunc("PUT /api/settings", g.updateSettings)
	mux.Handle("GET /", http.FileServerFS(dashboard.FS()))

	h := http.StripPrefix(path, mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authorize != nil && !authorize(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (g *handler) listEntries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	from, err := parseTime(q.Get("from"))
	if err != nil {
		http.Error(w, "invalid from: "+err.Error(), http.StatusBadRequest)
		return
	}
	to, err := parseTime(q.Get("to"))
	if err != nil {
		http.Error(w, "invalid to: "+err.Error(), http.StatusBadRequest)
		return
	}

	entries, err := g.store.List(r.Context(), store.Filter{
		Type:      entry.EntryType(q.Get("type")),
		RequestID: q.Get("requestId"),
		Search:    q.Get("search"),
		From:      from,
		To:        to,
		Limit:     limit,
		Offset:    max(offset, 0),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, entries)
}

func (g *handler) getEntry(w http.ResponseWriter, r *http.Request) {
	e, ok, err := g.store.Get(r.Context(), r.PathValue("id"))
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

func (g *handler) clearEntries(w http.ResponseWriter, r *http.Request) {
	if err := g.store.Clear(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// settings.Retention is in seconds; nil means the store does not support it.
type settings struct {
	Retention *int64 `json:"retention"`
}

func (g *handler) getSettings(w http.ResponseWriter, r *http.Request) {
	rs, ok := g.store.(store.RetentionStore)
	if !ok {
		writeJSON(w, settings{})
		return
	}
	d, err := rs.Retention(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	secs := int64(d / time.Second)
	writeJSON(w, settings{Retention: &secs})
}

func (g *handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	rs, ok := g.store.(store.RetentionStore)
	if !ok {
		http.Error(w, "store does not support retention", http.StatusBadRequest)
		return
	}
	var body settings
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Retention == nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	d := time.Duration(*body.Retention) * time.Second
	if d != 0 && d < time.Minute {
		http.Error(w, "retention must be 0 or at least 60 seconds", http.StatusBadRequest)
		return
	}
	if err := rs.SetRetention(r.Context(), d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	g.getSettings(w, r)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
