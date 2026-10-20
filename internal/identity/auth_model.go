package identity

import "github.com/google/uuid"

// RefreshToken is what the token store keeps per issued refresh token (keyed by its hash).
type RefreshToken struct {
	UserID   uuid.UUID
	FamilyID uuid.UUID
}

// TokenPair is the login/refresh response body (OAuth 2.0 token response field names).
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}
