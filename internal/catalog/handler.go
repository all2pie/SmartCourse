package catalog

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/errs"
	"github.com/all2pie/go-smart-course/internal/platform/web"
)

var errInvalidID = errs.New(errs.Invalid, "invalid course id")

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
	Title       string `json:"title"       binding:"required,max=255"`
	Description string `json:"description"`
}

func (h *Handler) create(c *gin.Context) error {
	actor, err := web.Actor(c)
	if err != nil {
		return err
	}

	req, err := web.Bind[createCourseRequest](c)
	if err != nil {
		return err
	}

	course, err := h.service.Create(c.Request.Context(), actor, CreateInput(req))
	if err != nil {
		return err
	}

	c.JSON(http.StatusCreated, course)
	return nil
}

func (h *Handler) getByID(c *gin.Context) error {
	actor, err := web.Actor(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errInvalidID
	}

	course, err := h.service.GetByID(c.Request.Context(), actor, id)
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
	actor, err := web.Actor(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errInvalidID
	}

	req, err := web.Bind[updateCourseRequest](c)
	if err != nil {
		return err
	}

	course, err := h.service.Update(c.Request.Context(), actor, id, UpdateInput(req))
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
	actor, err := web.Actor(c)
	if err != nil {
		return err
	}

	q, err := web.BindQuery[listCoursesQuery](c)
	if err != nil {
		return err
	}

	res, err := h.service.List(c.Request.Context(), actor, ListInput(q))
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, listCoursesResponse(res))
	return nil
}
