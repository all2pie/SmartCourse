package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
)

type Service struct {
	repo       Repository
	tokens     *auth.Tokens
	accessTTL  time.Duration
	refreshTTL time.Duration
	// dummyHash is compared against on unknown emails so login timing doesn't reveal which emails exist.
	dummyHash []byte
	now       func() time.Time
}

func NewService(repo Repository, tokens *auth.Tokens, accessTTL, refreshTTL time.Duration) (*Service, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("generating dummy hash: %w", err)
	}

	return &Service{
		repo:       repo,
		tokens:     tokens,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		dummyHash:  dummy,
		now:        func() time.Time { return time.Now().UTC() },
	}, nil
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
