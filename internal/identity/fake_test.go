package identity

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type fakeUsers struct {
	mu    sync.Mutex
	users map[uuid.UUID]User
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{users: map[uuid.UUID]User{}}
}

func (r *fakeUsers) CreateUser(_ context.Context, u *User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if existing.Email == u.Email {
			return ErrEmailTaken
		}
	}
	u.ID = uuid.New()
	u.CreatedAt = time.Now().UTC()
	u.UpdatedAt = u.CreatedAt
	r.users[u.ID] = *u
	return nil
}

func (r *fakeUsers) GetUserByID(_ context.Context, id uuid.UUID) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &u, nil
}

func (r *fakeUsers) GetUserByEmail(_ context.Context, email string) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.users {
		if u.Email == email {
			return &u, nil
		}
	}
	return nil, ErrNotFound
}

func (r *fakeUsers) delete(id uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, id)
}

// fakeTokenStore mirrors the Redis store's semantics (see token_store.go) without TTLs.
type fakeTokenStore struct {
	mu       sync.Mutex
	tokens   map[string]RefreshToken
	families map[uuid.UUID]string
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{tokens: map[string]RefreshToken{}, families: map[uuid.UUID]string{}}
}

func (s *fakeTokenStore) Create(_ context.Context, hash string, t RefreshToken, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[hash] = t
	s.families[t.FamilyID] = hash
	return nil
}

func (s *fakeTokenStore) Lookup(_ context.Context, hash string) (RefreshToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tokens[hash]
	if !ok {
		return RefreshToken{}, ErrInvalidRefreshToken
	}
	return t, nil
}

func (s *fakeTokenStore) Rotate(_ context.Context, t RefreshToken, oldHash, newHash string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.families[t.FamilyID] != oldHash {
		delete(s.families, t.FamilyID)
		return false, nil
	}
	s.tokens[newHash] = t
	s.families[t.FamilyID] = newHash
	return true, nil
}

func (s *fakeTokenStore) RevokeFamily(_ context.Context, familyID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.families, familyID)
	return nil
}
