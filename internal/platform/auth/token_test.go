package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func TestNewTokensRejectsShortSecret(t *testing.T) {
	if _, err := auth.NewTokens("too-short"); err == nil {
		t.Fatal("NewTokens accepted a secret shorter than 32 bytes")
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	tokens := testutil.Tokens(t)
	want := testutil.Actor(auth.RoleInstructor)

	token, err := tokens.Sign(want, time.Minute, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, err := tokens.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != want {
		t.Fatalf("Verify = %+v, want %+v", got, want)
	}
}

func TestVerifyRejects(t *testing.T) {
	tokens := testutil.Tokens(t)
	actor := testutil.Actor(auth.RoleStudent)
	now := time.Now()

	valid := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss":  auth.Issuer,
			"sub":  actor.ID.String(),
			"role": string(actor.Role),
			"iat":  now.Unix(),
			"exp":  now.Add(time.Minute).Unix(),
		}
	}
	sign := func(method jwt.SigningMethod, key any, c jwt.MapClaims) string {
		t.Helper()
		s, err := jwt.NewWithClaims(method, c).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	with := func(k string, v any) jwt.MapClaims {
		c := valid()
		if v == nil {
			delete(c, k)
		} else {
			c[k] = v
		}
		return c
	}
	key := []byte(testutil.Secret)

	expired, err := tokens.Sign(actor, time.Minute, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"expired":        expired,
		"wrong secret":   sign(jwt.SigningMethodHS256, []byte("another-secret-that-is-32-bytes-long!"), valid()),
		"alg none":       sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid()),
		"alg HS512":      sign(jwt.SigningMethodHS512, key, valid()),
		"wrong issuer":   sign(jwt.SigningMethodHS256, key, with("iss", "someone-else")),
		"no exp":         sign(jwt.SigningMethodHS256, key, with("exp", nil)),
		"sub not a uuid": sign(jwt.SigningMethodHS256, key, with("sub", "42")),
		"no sub":         sign(jwt.SigningMethodHS256, key, with("sub", nil)),
		"unknown role":   sign(jwt.SigningMethodHS256, key, with("role", "superuser")),
		"no role":        sign(jwt.SigningMethodHS256, key, with("role", nil)),
		"garbage":        "not.a.jwt",
		"empty":          "",
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			a, err := tokens.Verify(token)
			if !errors.Is(err, auth.ErrInvalidToken) {
				t.Fatalf("Verify err = %v, want ErrInvalidToken", err)
			}
			if a.ID != uuid.Nil {
				t.Fatalf("Verify returned an actor %+v alongside the error", a)
			}
		})
	}
}

func TestRoleValid(t *testing.T) {
	for _, r := range []auth.Role{auth.RoleStudent, auth.RoleInstructor, auth.RoleAdmin} {
		if !r.Valid() {
			t.Errorf("%q.Valid() = false", r)
		}
	}
	for _, r := range []auth.Role{"", "Admin", "root"} {
		if r.Valid() {
			t.Errorf("%q.Valid() = true", r)
		}
	}
}
