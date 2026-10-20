package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Redis layout (hash = hex SHA-256 of the refresh token):
//
//	identity:rt:<hash>       HASH {user, family}  immutable record of every issued token, TTL = refresh TTL
//	identity:rtfam:<family>  STRING <hash>        the family's only currently valid token, TTL = refresh TTL
//
// A token is valid only while its family points at it. Rotation moves the pointer;
// presenting any other token of the family deletes the pointer (reuse detection).
const (
	tokenKeyPrefix  = "identity:rt:"
	familyKeyPrefix = "identity:rtfam:"
)

type TokenStore interface {
	Create(ctx context.Context, hash string, t RefreshToken, ttl time.Duration) error
	Lookup(ctx context.Context, hash string) (RefreshToken, error)
	// Rotate atomically replaces oldHash with newHash as the family's current token.
	// It returns false, after revoking the family, if oldHash was not current.
	Rotate(ctx context.Context, t RefreshToken, oldHash, newHash string, ttl time.Duration) (bool, error)
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error
}

type redisTokenStore struct {
	rdb *redis.Client
}

func NewTokenStore(rdb *redis.Client) TokenStore {
	return &redisTokenStore{rdb: rdb}
}

func (s *redisTokenStore) Create(ctx context.Context, hash string, t RefreshToken, ttl time.Duration) error {
	_, err := s.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, tokenKeyPrefix+hash, "user", t.UserID.String(), "family", t.FamilyID.String())
		p.Expire(ctx, tokenKeyPrefix+hash, ttl)
		p.Set(ctx, familyKeyPrefix+t.FamilyID.String(), hash, ttl)
		return nil
	})
	if err != nil {
		return fmt.Errorf("storing refresh token: %w", err)
	}
	return nil
}

func (s *redisTokenStore) Lookup(ctx context.Context, hash string) (RefreshToken, error) {
	fields, err := s.rdb.HGetAll(ctx, tokenKeyPrefix+hash).Result()
	if err != nil {
		return RefreshToken{}, fmt.Errorf("looking up refresh token: %w", err)
	}
	if len(fields) == 0 {
		return RefreshToken{}, ErrInvalidRefreshToken
	}

	userID, errU := uuid.Parse(fields["user"])
	familyID, errF := uuid.Parse(fields["family"])
	if err := errors.Join(errU, errF); err != nil {
		return RefreshToken{}, fmt.Errorf("corrupt refresh token record: %w", err)
	}
	return RefreshToken{UserID: userID, FamilyID: familyID}, nil
}

// KEYS[1] family pointer, KEYS[2] new token record.
// ARGV: presented hash, new hash, user id, family id, ttl seconds.
var rotateScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then
  redis.call('DEL', KEYS[1])
  return 0
end
redis.call('HSET', KEYS[2], 'user', ARGV[3], 'family', ARGV[4])
redis.call('EXPIRE', KEYS[2], ARGV[5])
redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[5])
return 1
`)

func (s *redisTokenStore) Rotate(ctx context.Context, t RefreshToken, oldHash, newHash string, ttl time.Duration) (bool, error) {
	ok, err := rotateScript.Run(ctx, s.rdb,
		[]string{familyKeyPrefix + t.FamilyID.String(), tokenKeyPrefix + newHash},
		oldHash, newHash, t.UserID.String(), t.FamilyID.String(), int(ttl.Seconds()),
	).Int()
	if err != nil {
		return false, fmt.Errorf("rotating refresh token: %w", err)
	}
	return ok == 1, nil
}

func (s *redisTokenStore) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	if err := s.rdb.Del(ctx, familyKeyPrefix+familyID.String()).Err(); err != nil {
		return fmt.Errorf("revoking refresh token family: %w", err)
	}
	return nil
}
