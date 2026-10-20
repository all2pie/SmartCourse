package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *gormRepository) CreateRefreshToken(ctx context.Context, t *RefreshToken) error {
	if err := r.db.WithContext(ctx).Create(t).Error; err != nil {
		return fmt.Errorf("creating refresh token: %w", err)
	}
	return nil
}

func (r *gormRepository) GetRefreshTokenForUpdate(ctx context.Context, hash []byte) (*RefreshToken, error) {
	var t RefreshToken
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate}).
		First(&t, "token_hash = ?", hash).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, fmt.Errorf("fetching refresh token: %w", err)
	}
	return &t, nil
}

func (r *gormRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID, at time.Time) error {
	err := r.db.WithContext(ctx).Model(&RefreshToken{}).
		Where("id = ?", id).
		Update("revoked_at", at).Error
	if err != nil {
		return fmt.Errorf("revoking refresh token: %w", err)
	}
	return nil
}

func (r *gormRepository) RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error {
	err := r.db.WithContext(ctx).Model(&RefreshToken{}).
		Where("family_id = ? AND revoked_at IS NULL", familyID).
		Update("revoked_at", at).Error
	if err != nil {
		return fmt.Errorf("revoking refresh token family: %w", err)
	}
	return nil
}
