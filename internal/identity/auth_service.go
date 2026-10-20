package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

var (
	ErrInvalidCredentials  = errs.New(errs.Unauthenticated, "invalid email or password")
	ErrInvalidRefreshToken = errs.New(errs.Unauthenticated, "invalid refresh token")
)

// AuthService issues and rotates sessions: JWT access tokens plus opaque refresh tokens in the TokenStore.
type AuthService struct {
	users      Repository
	store      TokenStore
	tokens     *auth.Tokens
	accessTTL  time.Duration
	refreshTTL time.Duration
	// dummyHash is compared against on unknown emails so login timing doesn't reveal which emails exist.
	dummyHash []byte
}

func NewAuthService(users Repository, store TokenStore, tokens *auth.Tokens, accessTTL, refreshTTL time.Duration) (*AuthService, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("generating dummy hash: %w", err)
	}

	return &AuthService{
		users:      users,
		store:      store,
		tokens:     tokens,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		dummyHash:  dummy,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (TokenPair, error) {
	u, err := s.users.GetUserByEmail(ctx, normalizeEmail(email))
	if errors.Is(err, ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return TokenPair{}, ErrInvalidCredentials
	}
	if err != nil {
		return TokenPair{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	access, err := s.signAccess(u)
	if err != nil {
		return TokenPair{}, err
	}

	refresh := rand.Text()
	t := RefreshToken{UserID: u.ID, FamilyID: uuid.New()}
	if err := s.store.Create(ctx, hashToken(refresh), t, s.refreshTTL); err != nil {
		return TokenPair{}, err
	}

	return s.pair(access, refresh), nil
}

// Refresh rotates a refresh token. Presenting a token that is no longer its family's
// current one means it leaked, so the store revokes the whole family (that login session).
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	oldHash := hashToken(refreshToken)
	t, err := s.store.Lookup(ctx, oldHash)
	if err != nil {
		return TokenPair{}, err
	}

	// Re-read the user so role changes apply from the next access token.
	u, err := s.users.GetUserByID(ctx, t.UserID)
	if errors.Is(err, ErrNotFound) {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return TokenPair{}, err
	}

	access, err := s.signAccess(u)
	if err != nil {
		return TokenPair{}, err
	}

	refresh := rand.Text()
	ok, err := s.store.Rotate(ctx, t, oldHash, hashToken(refresh), s.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	if !ok {
		return TokenPair{}, ErrInvalidRefreshToken
	}

	return s.pair(access, refresh), nil
}

// Logout ends the session the refresh token belongs to. Unknown tokens are ignored so the call is idempotent.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	t, err := s.store.Lookup(ctx, hashToken(refreshToken))
	if errors.Is(err, ErrInvalidRefreshToken) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.store.RevokeFamily(ctx, t.FamilyID)
}

func (s *AuthService) signAccess(u *User) (string, error) {
	access, err := s.tokens.Sign(auth.Actor{ID: u.ID, Role: u.Role}, s.accessTTL, time.Now())
	if err != nil {
		return "", fmt.Errorf("signing access token: %w", err)
	}
	return access, nil
}

func (s *AuthService) pair(access, refresh string) TokenPair {
	return TokenPair{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: refresh,
	}
}

// Refresh tokens are 130-bit random values, so a fast hash is enough (no need for bcrypt).
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
