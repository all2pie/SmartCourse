package identity

import "github.com/google/uuid"

// RefreshToken is what the token store keeps per issued refresh token (keyed by its hash).
type RefreshToken struct {
	UserID   uuid.UUID
	FamilyID uuid.UUID
}

// TokenPair is the result of a login or refresh.
type TokenPair struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int
	RefreshToken string
}
