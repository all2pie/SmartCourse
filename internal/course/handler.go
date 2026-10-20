package course

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/errs"
	"github.com/all2pie/go-smart-course/internal/web"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(router gin.IRouter) {
	router.POST("/courses", web.Wrap(h.create))
	router.GET("/courses/:id", web.Wrap(h.getByID))
}

type createCourseRequest struct {
	InstructorID uuid.UUID `json:"instructor_id" binding:"required"`
	Title        string    `json:"title"         binding:"required,max=255"`
	Description  string    `json:"description"`
}

func (h *Handler) create(c *gin.Context) error {
	req, err := web.Bind[createCourseRequest](c)
	if err != nil {
		return err
	}

	course, err := h.service.Create(c.Request.Context(), CreateInput(req))
	if err != nil {
		return err
	}

	c.JSON(http.StatusCreated, course)
	return nil
}

func (h *Handler) getByID(c *gin.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errs.New(errs.Invalid, "invalid course id")
	}

	course, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, course)
	return nil
}
