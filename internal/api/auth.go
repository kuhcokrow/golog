package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/kuhcokrow/golog/internal/auth"
)

const maxPassword = 1024

type authStatus struct {
	Enabled  bool       `json:"enabled"`
	State    auth.State `json:"state"`
	Username string     `json:"username,omitempty"`
}

type credentials struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (g *handler) authStatus(w http.ResponseWriter, r *http.Request) {
	if g.auth == nil {
		writeJSON(w, authStatus{State: auth.StateOK})
		return
	}
	state, username, err := g.auth.State(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, authStatus{Enabled: true, State: state, Username: username})
}

func (g *handler) register(w http.ResponseWriter, r *http.Request) {
	c, ok := g.readCredentials(w, r)
	if !ok {
		return
	}
	c.Username = strings.TrimSpace(c.Username)
	switch {
	case c.Username == "":
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	case utf8.RuneCountInString(c.Username) > auth.MaxUsername:
		http.Error(w, "username is too long", http.StatusBadRequest)
		return
	case utf8.RuneCountInString(c.Password) < auth.MinPassword:
		http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	case c.Password != c.ConfirmPassword:
		http.Error(w, "passwords do not match", http.StatusBadRequest)
		return
	}

	err := g.auth.Register(r.Context(), w, r, c.Username, c.Password)
	if errors.Is(err, auth.ErrUserExists) {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, authStatus{Enabled: true, State: auth.StateOK, Username: c.Username})
}

func (g *handler) login(w http.ResponseWriter, r *http.Request) {
	c, ok := g.readCredentials(w, r)
	if !ok {
		return
	}
	err := g.auth.Login(r.Context(), w, r, c.Username, c.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	case errors.Is(err, auth.ErrNoUser):
		http.Error(w, err.Error(), http.StatusConflict)
		return
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, authStatus{Enabled: true, State: auth.StateOK, Username: c.Username})
}

func (g *handler) logout(w http.ResponseWriter, r *http.Request) {
	if g.auth != nil {
		g.auth.Logout(w, r)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (g *handler) readCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	var c credentials
	if g.auth == nil {
		http.Error(w, "login is disabled", http.StatusNotFound)
		return c, false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&c); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return c, false
	}
	if len(c.Password) > maxPassword {
		http.Error(w, "password is too long", http.StatusBadRequest)
		return c, false
	}
	return c, true
}
