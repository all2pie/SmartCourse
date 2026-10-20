package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/all2pie/go-smart-course/internal/platform/errs"
)

var (
	ErrNotFound   = errs.New(errs.NotFound, "user not found")
	ErrEmailTaken = errs.New(errs.Conflict, "email already in use")
)

type Repository interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)

	CreateRefreshToken(ctx context.Context, t *RefreshToken) error
	// GetRefreshTokenForUpdate row-locks the token; call it inside InTx.
	GetRefreshTokenForUpdate(ctx context.Context, hash []byte) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error

	InTx(ctx context.Context, fn func(tx Repository) error) error
}

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) InTx(ctx context.Context, fn func(tx Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepository{db: tx})
	})
}

func (r *gormRepository) CreateUser(ctx context.Context, u *User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrEmailTaken
		}
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

func (r *gormRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return r.getUser(ctx, "id = ?", id)
}

func (r *gormRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return r.getUser(ctx, "email = ?", email)
}

func (r *gormRepository) getUser(ctx context.Context, where string, arg any) (*User, error) {
	var u User
	if err := r.db.WithContext(ctx).First(&u, where, arg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("fetching user: %w", err)
	}
	return &u, nil
}
