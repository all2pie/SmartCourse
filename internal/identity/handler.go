package identity

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
)

type Handler struct {
	service *Service
	auth    *AuthService
}

func NewHandler(service *Service, auth *AuthService) *Handler {
	return &Handler{service: service, auth: auth}
}

func (h *Handler) Register(api huma.API) {
	h.registerAuth(api)

	users := []string{"users"}
	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "getCurrentUser",
		Method:      http.MethodGet,
		Path:        "/users/me",
		Tags:        users,
		Summary:     "The caller's own profile",
	}), h.me)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "getUser",
		Method:      http.MethodGet,
		Path:        "/users/{id}",
		Tags:        users,
		Summary:     "A user's profile",
		Description: "`email` is included only when the caller is that user or an admin.",
		Errors:      []int{http.StatusNotFound},
	}), h.getByID)
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"              format:"uuid"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty" format:"email" doc:"Present only for the user themselves or an admin."`
	Role      auth.Role `json:"role"            enum:"student,instructor,admin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserResponse(u *User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

type userOutput struct {
	Body UserResponse
}

func (h *Handler) me(ctx context.Context, _ *struct{}) (*userOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	u, err := h.service.GetByID(ctx, actor, actor.ID)
	if err != nil {
		return nil, err
	}
	return &userOutput{Body: toUserResponse(u)}, nil
}

type userIDInput struct {
	ID uuid.UUID `path:"id" format:"uuid"`
}

func (h *Handler) getByID(ctx context.Context, in *userIDInput) (*userOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	u, err := h.service.GetByID(ctx, actor, in.ID)
	if err != nil {
		return nil, err
	}
	return &userOutput{Body: toUserResponse(u)}, nil
}
