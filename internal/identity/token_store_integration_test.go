//go:build integration

package identity

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func TestRedisTokenStore(t *testing.T) {
	rdb := testutil.Redis(t)
	store := NewTokenStore(rdb)
	ctx := t.Context()
	const ttl = time.Hour

	newFamily := func(t *testing.T, hash string) RefreshToken {
		t.Helper()
		tok := RefreshToken{UserID: uuid.New(), FamilyID: uuid.New()}
		if err := store.Create(ctx, hash, tok, ttl); err != nil {
			t.Fatal(err)
		}
		return tok
	}

	t.Run("create, lookup and TTLs", func(t *testing.T) {
		tok := newFamily(t, "h-create")
		got, err := store.Lookup(ctx, "h-create")
		if err != nil {
			t.Fatal(err)
		}
		if got != tok {
			t.Fatalf("Lookup = %+v, want %+v", got, tok)
		}
		for _, key := range []string{tokenKeyPrefix + "h-create", familyKeyPrefix + tok.FamilyID.String()} {
			if d := rdb.TTL(ctx, key).Val(); d <= 0 || d > ttl {
				t.Errorf("TTL(%s) = %v, want (0, %v]", key, d, ttl)
			}
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		if _, err := store.Lookup(ctx, "missing"); !errors.Is(err, ErrInvalidRefreshToken) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("rotate, then reuse revokes the family", func(t *testing.T) {
		tok := newFamily(t, "h1")
		if ok, err := store.Rotate(ctx, tok, "h1", "h2", ttl); err != nil || !ok {
			t.Fatalf("first rotation = %v, %v", ok, err)
		}
		if d := rdb.TTL(ctx, tokenKeyPrefix+"h2").Val(); d <= 0 {
			t.Fatalf("rotated token has no TTL: %v", d)
		}
		if ok, _ := store.Rotate(ctx, tok, "h1", "h3", ttl); ok {
			t.Fatal("rotating an already rotated token succeeded")
		}
		if ok, _ := store.Rotate(ctx, tok, "h2", "h4", ttl); ok {
			t.Fatal("the current token still works after reuse was detected")
		}
	})

	t.Run("revoke family", func(t *testing.T) {
		tok := newFamily(t, "h-revoke")
		if err := store.RevokeFamily(ctx, tok.FamilyID); err != nil {
			t.Fatal(err)
		}
		if ok, _ := store.Rotate(ctx, tok, "h-revoke", "h-after", ttl); ok {
			t.Fatal("rotation succeeded after revocation")
		}
	})

	t.Run("concurrent rotations: exactly one wins", func(t *testing.T) {
		tok := newFamily(t, "h-race")
		const n = 20
		var wins atomic.Int32
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				ok, err := store.Rotate(ctx, tok, "h-race", fmt.Sprintf("h-race-%d", i), ttl)
				if err != nil {
					t.Error(err)
				}
				if ok {
					wins.Add(1)
				}
			})
		}
		wg.Wait()
		if wins.Load() != 1 {
			t.Fatalf("%d of %d concurrent rotations succeeded, want 1", wins.Load(), n)
		}
	})

	t.Run("keys stay in the identity namespace", func(t *testing.T) {
		keys, err := rdb.Keys(ctx, "*").Result()
		if err != nil {
			t.Fatal(err)
		}
		for _, k := range keys {
			if !strings.HasPrefix(k, tokenKeyPrefix) && !strings.HasPrefix(k, familyKeyPrefix) {
				t.Errorf("unexpected key %q", k)
			}
		}
	})
}
