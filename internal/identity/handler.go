package identity

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
	router.POST("/auth/register", web.Wrap(h.register))
	router.POST("/auth/login", web.Wrap(h.login))
	router.POST("/auth/refresh", web.Wrap(h.refresh))
	router.POST("/auth/logout", web.Wrap(h.logout))

	router.GET("/users/me", web.Wrap(h.me))
	router.GET("/users/:id", web.Wrap(h.getByID))
}

func (h *Handler) me(c *gin.Context) error {
	actor, err := web.Actor(c)
	if err != nil {
		return err
	}

	u, err := h.service.GetByID(c.Request.Context(), actor, actor.ID)
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, u)
	return nil
}

func (h *Handler) getByID(c *gin.Context) error {
	actor, err := web.Actor(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errs.New(errs.Invalid, "invalid user id")
	}

	u, err := h.service.GetByID(c.Request.Context(), actor, id)
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, u)
	return nil
}
