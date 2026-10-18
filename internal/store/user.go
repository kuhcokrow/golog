package store

import (
	"context"
	"sync"
)

// User is the single dashboard user. PasswordHash is never the plain password.
type User struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
}

// UserStore keeps the dashboard user, so it can survive restarts when the
// store is persistent. CreateUser must return false, without error, when a
// user already exists, so only one user can ever be registered.
type UserStore interface {
	User(ctx context.Context) (User, bool, error)
	CreateUser(ctx context.Context, u User) (bool, error)
}

// NewMemoryUsers keeps the user in memory, so it is lost on restart.
func NewMemoryUsers() UserStore {
	return &memoryUsers{}
}

type memoryUsers struct {
	mu   sync.RWMutex
	user *User
}

func (m *memoryUsers) User(context.Context) (User, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.user == nil {
		return User{}, false, nil
	}
	return *m.user, true, nil
}

func (m *memoryUsers) CreateUser(_ context.Context, u User) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.user != nil {
		return false, nil
	}
	m.user = &u
	return true, nil
}
