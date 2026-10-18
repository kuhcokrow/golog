// Package auth protects the dashboard with a single registered user and
// cookie sessions.
package auth

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kuhcokrow/golog/internal/store"
)

const (
	cookieName  = "golog_session"
	sessionTTL  = 7 * 24 * time.Hour
	iterations  = 600_000
	saltSize    = 16
	keySize     = 32
	MinPassword = 8
	MaxUsername = 64
)

var (
	ErrUserExists         = errors.New("a user is already registered")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrNoUser             = errors.New("no user is registered yet")
)

type State string

const (
	StateSetup State = "setup"
	StateLogin State = "login"
	StateOK    State = "ok"
)

// Manager keeps sessions in memory, so users log in again after a restart.
type Manager struct {
	users    store.UserStore
	path     string
	mu       sync.Mutex
	sessions map[string]time.Time
}

func New(users store.UserStore, path string) *Manager {
	return &Manager{users: users, path: path, sessions: map[string]time.Time{}}
}

func (m *Manager) State(r *http.Request) (State, string, error) {
	u, ok, err := m.users.User(r.Context())
	if err != nil {
		return "", "", err
	}
	if !ok {
		return StateSetup, "", nil
	}
	if !m.Authenticated(r) {
		return StateLogin, "", nil
	}
	return StateOK, u.Username, nil
}

func (m *Manager) Authenticated(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.sessions[c.Value]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(m.sessions, c.Value)
		return false
	}
	return true
}

func (m *Manager) Register(ctx context.Context, w http.ResponseWriter, r *http.Request, username, password string) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	created, err := m.users.CreateUser(ctx, store.User{Username: username, PasswordHash: hash})
	if err != nil {
		return err
	}
	if !created {
		return ErrUserExists
	}
	m.startSession(w, r)
	return nil
}

// Login always verifies the password, even for a wrong username, so the
// response time does not reveal which one was wrong.
func (m *Manager) Login(ctx context.Context, w http.ResponseWriter, r *http.Request, username, password string) error {
	u, ok, err := m.users.User(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoUser
	}
	passOK := verifyPassword(u.PasswordHash, password)
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(u.Username)) == 1
	if !passOK || !userOK {
		return ErrInvalidCredentials
	}
	m.startSession(w, r)
	return nil
}

func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, c.Value)
		m.mu.Unlock()
	}
	http.SetCookie(w, m.cookie(r, "", -1))
}

func (m *Manager) startSession(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	now := time.Now()
	m.mu.Lock()
	for t, exp := range m.sessions {
		if now.After(exp) {
			delete(m.sessions, t)
		}
	}
	m.sessions[token] = now.Add(sessionTTL)
	m.mu.Unlock()

	http.SetCookie(w, m.cookie(r, token, int(sessionTTL/time.Second)))
}

func (m *Manager) cookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     m.path + "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteStrictMode,
	}
}

// hashPassword encodes the result as pbkdf2-sha256$iterations$salt$key.
func hashPassword(password string) (string, error) {
	salt := make([]byte, saltSize)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, password, salt, iterations, keySize)
	if err != nil {
		return "", err
	}
	enc := base64.RawStdEncoding
	return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s", iterations, enc.EncodeToString(salt), enc.EncodeToString(key)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter <= 0 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := enc.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
