package identity

import (
	"context"
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
	ErrRoleNotAllowed  = errs.New(errs.Invalid, "role must be student or instructor")
	ErrInvalidPassword = errs.New(errs.Invalid, "password must be at least 8 characters and at most 72 bytes")
)

// Service manages user accounts; sessions live in AuthService.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

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

// GetByID hides the email unless the actor may see it (see policy.go).
func (s *Service) GetByID(ctx context.Context, actor auth.Actor, id uuid.UUID) (*User, error) {
	u, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canSeeEmail(actor, u) {
		u.Email = ""
	}
	return u, nil
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

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
