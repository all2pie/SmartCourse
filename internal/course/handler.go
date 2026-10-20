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
	router.GET("/courses", web.Wrap(h.list))
	router.GET("/courses/:id", web.Wrap(h.getByID))
	router.PATCH("/courses/:id", web.Wrap(h.update))
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

type updateCourseRequest struct {
	Title       *string `json:"title"       binding:"omitempty,min=1,max=255"`
	Description *string `json:"description"`
}

func (h *Handler) update(c *gin.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errs.New(errs.Invalid, "invalid course id")
	}

	req, err := web.Bind[updateCourseRequest](c)
	if err != nil {
		return err
	}

	course, err := h.service.Update(c.Request.Context(), id, UpdateInput(req))
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, course)
	return nil
}

type listCoursesQuery struct {
	InstructorID *uuid.UUID `form:"instructor_id,parser=encoding.TextUnmarshaler"`
	Limit        int        `form:"limit"                                         binding:"omitempty,min=1,max=100"`
	Offset       int        `form:"offset"                                        binding:"omitempty,min=0"`
}

type listCoursesResponse struct {
	Items  []Course `json:"items"`
	Total  int64    `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

func (h *Handler) list(c *gin.Context) error {
	q, err := web.BindQuery[listCoursesQuery](c)
	if err != nil {
		return err
	}

	res, err := h.service.List(c.Request.Context(), ListFilter(q))
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, listCoursesResponse(res))
	return nil
}
