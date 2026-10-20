package identity

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/errs"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST("/users", web.Wrap(h.create))
	router.GET("/users/:id", web.Wrap(h.getByID))
}

type createUserRequest struct {
	Name  string    `json:"name"  binding:"required"`
	Email string    `json:"email" binding:"required,email"`
	Role  auth.Role `json:"role"  binding:"required"`
}

func (h *Handler) create(c *gin.Context) error {
	req, err := web.Bind[createUserRequest](c)
	if err != nil {
		return err
	}

	u, err := h.service.Create(c.Request.Context(), CreateInput(req))
	if err != nil {
		return err
	}

	c.JSON(http.StatusCreated, u)
	return nil
}

func (h *Handler) getByID(c *gin.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errs.New(errs.Invalid, "invalid user id")
	}

	u, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, u)
	return nil
}
