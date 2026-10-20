package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

const (
	minPasswordChars = 8
	// bcrypt ignores input past 72 bytes.
	maxPasswordBytes = 72
)

var (
	ErrRoleNotAllowed      = errs.New(errs.Invalid, "role must be student or instructor")
	ErrInvalidPassword     = errs.New(errs.Invalid, "password must be at least 8 characters and at most 72 bytes")
	ErrInvalidCredentials  = errs.New(errs.Unauthenticated, "invalid email or password")
	ErrInvalidRefreshToken = errs.New(errs.Unauthenticated, "invalid refresh token")
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Role     auth.Role
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (*User, error) {
	if !canSelfRegisterAs(in.Role) {
		return nil, ErrRoleNotAllowed
	}
	return s.createUser(ctx, in)
}

// CreateAdmin is for the operator-run identity-admin command only; it is not exposed over HTTP.
func (s *Service) CreateAdmin(ctx context.Context, name, email, password string) (*User, error) {
	return s.createUser(ctx, RegisterInput{Name: name, Email: email, Password: password, Role: auth.RoleAdmin})
}

func (s *Service) createUser(ctx context.Context, in RegisterInput) (*User, error) {
	if utf8.RuneCountInString(in.Password) < minPasswordChars || len(in.Password) > maxPasswordBytes {
		return nil, ErrInvalidPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	u := &User{
		Name:         in.Name,
		Email:        normalizeEmail(in.Email),
		PasswordHash: string(hash),
		Role:         in.Role,
	}
	if err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (TokenPair, error) {
	u, err := s.repo.GetUserByEmail(ctx, normalizeEmail(email))
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

	return s.issueTokens(ctx, s.repo, u, uuid.New())
}

// Refresh rotates a refresh token. Presenting an already-rotated token means it leaked,
// so the whole family (that login session) is revoked.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	var (
		pair   TokenPair
		reused bool
	)

	err := s.repo.InTx(ctx, func(tx Repository) error {
		t, err := tx.GetRefreshTokenForUpdate(ctx, hashToken(refreshToken))
		if err != nil {
			return err
		}

		now := s.now()
		if t.RevokedAt != nil {
			// Commit the family revocation; the error is returned after the transaction.
			reused = true
			return tx.RevokeRefreshFamily(ctx, t.FamilyID, now)
		}
		if !now.Before(t.ExpiresAt) {
			return ErrInvalidRefreshToken
		}

		if err := tx.RevokeRefreshToken(ctx, t.ID, now); err != nil {
			return err
		}

		// Re-read the user so role changes apply from the next access token.
		u, err := tx.GetUserByID(ctx, t.UserID)
		if err != nil {
			return err
		}

		pair, err = s.issueTokens(ctx, tx, u, t.FamilyID)
		return err
	})
	if err != nil {
		return TokenPair{}, err
	}
	if reused {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	return pair, nil
}

// Logout ends the session the refresh token belongs to. Unknown tokens are ignored so the call is idempotent.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.InTx(ctx, func(tx Repository) error {
		t, err := tx.GetRefreshTokenForUpdate(ctx, hashToken(refreshToken))
		if errors.Is(err, ErrInvalidRefreshToken) {
			return nil
		}
		if err != nil {
			return err
		}
		return tx.RevokeRefreshFamily(ctx, t.FamilyID, s.now())
	})
}

func (s *Service) issueTokens(ctx context.Context, repo Repository, u *User, familyID uuid.UUID) (TokenPair, error) {
	now := s.now()

	access, err := s.tokens.Sign(auth.Actor{ID: u.ID, Role: u.Role}, s.accessTTL, now)
	if err != nil {
		return TokenPair{}, fmt.Errorf("signing access token: %w", err)
	}

	refresh := rand.Text()
	err = repo.CreateRefreshToken(ctx, &RefreshToken{
		UserID:    u.ID,
		FamilyID:  familyID,
		TokenHash: hashToken(refresh),
		ExpiresAt: now.Add(s.refreshTTL),
	})
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.accessTTL.Seconds()),
		RefreshToken: refresh,
	}, nil
}

// Refresh tokens are 130-bit random values, so a fast hash is enough (no need for bcrypt).
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
