package identity

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
)

func (h *Handler) registerAuth(api huma.API) {
	tags := []string{"auth"}

	httpapi.Register(api, httpapi.Public(huma.Operation{
		OperationID:   "register",
		Method:        http.MethodPost,
		Path:          "/auth/register",
		Tags:          tags,
		Summary:       "Register a student or instructor",
		Description:   "Admin accounts cannot self-register; they are created by an operator with `task admin:create`.",
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusConflict},
	}), h.register)

	httpapi.Register(api, httpapi.Public(huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/auth/login",
		Tags:        tags,
		Summary:     "Exchange email and password for tokens",
		Description: "Wrong password and unknown email return the same 401, so accounts can't be enumerated.",
		Errors:      []int{http.StatusUnauthorized},
	}), h.login)

	httpapi.Register(api, httpapi.Public(huma.Operation{
		OperationID: "refreshTokens",
		Method:      http.MethodPost,
		Path:        "/auth/refresh",
		Tags:        tags,
		Summary:     "Rotate a refresh token",
		Description: "Returns a new access token and a new refresh token; the presented refresh token stops working. " +
			"Presenting an already rotated token revokes the whole session.",
		Errors: []int{http.StatusUnauthorized},
	}), h.refresh)

	httpapi.Register(api, httpapi.Public(huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          "/auth/logout",
		Tags:          tags,
		Summary:       "End the session a refresh token belongs to",
		Description:   "Idempotent; unknown or already revoked tokens also return 204.",
		DefaultStatus: http.StatusNoContent,
	}), h.logout)
}

type RegisterRequest struct {
	Name     string    `json:"name"     minLength:"1"                maxLength:"255"`
	Email    string    `json:"email"    format:"email"               maxLength:"255"`
	Password string    `json:"password" minLength:"8"                maxLength:"72"  writeOnly:"true" doc:"8 characters to 72 bytes (bcrypt's limit)."`
	Role     auth.Role `json:"role"     enum:"student,instructor"`
}

type registerInput struct {
	Body RegisterRequest
}

func (h *Handler) register(ctx context.Context, in *registerInput) (*userOutput, error) {
	u, err := h.service.Register(ctx, RegisterInput(in.Body))
	if err != nil {
		return nil, err
	}
	return &userOutput{Body: toUserResponse(u)}, nil
}

type LoginRequest struct {
	Email    string `json:"email"    minLength:"1"`
	Password string `json:"password" minLength:"1" writeOnly:"true"`
}

type loginInput struct {
	Body LoginRequest
}

// TokenResponse uses the OAuth 2.0 token response field names (RFC 6749 section 5.1).
type TokenResponse struct {
	AccessToken  string `json:"access_token"  doc:"JWT (HS256)."`
	TokenType    string `json:"token_type"    enum:"Bearer"`
	ExpiresIn    int    `json:"expires_in"    doc:"Access token lifetime in seconds."             example:"900"`
	RefreshToken string `json:"refresh_token" doc:"Opaque; valid for 7 days or until used once."`
}

type tokensOutput struct {
	// Token responses must not be cached (RFC 6749 section 5.1).
	CacheControl string `header:"Cache-Control" enum:"no-store"`
	Body         TokenResponse
}

func toTokensOutput(p TokenPair) *tokensOutput {
	return &tokensOutput{CacheControl: "no-store", Body: TokenResponse(p)}
}

func (h *Handler) login(ctx context.Context, in *loginInput) (*tokensOutput, error) {
	pair, err := h.auth.Login(ctx, in.Body.Email, in.Body.Password)
	if err != nil {
		return nil, err
	}
	return toTokensOutput(pair), nil
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" minLength:"1"`
}

type refreshInput struct {
	Body RefreshRequest
}

func (h *Handler) refresh(ctx context.Context, in *refreshInput) (*tokensOutput, error) {
	pair, err := h.auth.Refresh(ctx, in.Body.RefreshToken)
	if err != nil {
		return nil, err
	}
	return toTokensOutput(pair), nil
}

func (h *Handler) logout(ctx context.Context, in *refreshInput) (*struct{}, error) {
	if err := h.auth.Logout(ctx, in.Body.RefreshToken); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}
