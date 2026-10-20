package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	Issuer = "smartcourse-identity"
	// HS256 keys shorter than the hash output weaken the signature.
	minSecretBytes = 32
)

var ErrInvalidToken = errors.New("invalid access token")

type claims struct {
	Role Role `json:"role"`
	jwt.RegisteredClaims
}

// Tokens signs (Identity) and verifies (gateway) HS256 access tokens.
type Tokens struct {
	key []byte
}

func NewTokens(secret string) (*Tokens, error) {
	if len(secret) < minSecretBytes {
		return nil, fmt.Errorf("jwt secret must be at least %d bytes", minSecretBytes)
	}
	return &Tokens{key: []byte(secret)}, nil
}

func (t *Tokens) Sign(a Actor, ttl time.Duration, now time.Time) (string, error) {
	c := claims{
		Role: a.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   a.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.key)
}

func (t *Tokens) Verify(token string) (Actor, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return t.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(Issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Actor{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	id, err := uuid.Parse(c.Subject)
	if err != nil || !c.Role.Valid() {
		return Actor{}, ErrInvalidToken
	}
	return Actor{ID: id, Role: c.Role}, nil
}
