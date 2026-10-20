package catalog

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
)

var tags = []string{"courses"}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(api huma.API) {
	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID:   "createCourse",
		Method:        http.MethodPost,
		Path:          "/courses",
		Tags:          tags,
		Summary:       "Create a draft course (instructors only)",
		Description:   "The caller becomes the course's instructor.",
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusForbidden},
	}), h.create)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "listCourses",
		Method:      http.MethodGet,
		Path:        "/courses",
		Tags:        tags,
		Summary:     "List courses visible to the caller",
		Description: "Students see published courses; instructors also see their own courses in any status; " +
			"admins see everything. Newest first.",
	}), h.list)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "getCourse",
		Method:      http.MethodGet,
		Path:        "/courses/{id}",
		Tags:        tags,
		Summary:     "A course",
		Description: "Courses the caller may not see return 404, so their existence isn't revealed.",
		Errors:      []int{http.StatusNotFound},
	}), h.getByID)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "updateCourse",
		Method:      http.MethodPatch,
		Path:        "/courses/{id}",
		Tags:        tags,
		Summary:     "Update a course's title or description",
		Description: "Only the course's instructor or an admin. Omitted fields are unchanged; " +
			"an empty description clears it; at least one field is required.",
		Errors: []int{http.StatusForbidden, http.StatusNotFound},
	}), h.update)
}

type CourseResponse struct {
	ID           uuid.UUID `json:"id"            format:"uuid"`
	InstructorID uuid.UUID `json:"instructor_id" format:"uuid"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	Status       Status    `json:"status"        enum:"draft,published"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toCourseResponse(c *Course) CourseResponse {
	return CourseResponse{
		ID:           c.ID,
		InstructorID: c.InstructorID,
		Title:        c.Title,
		Description:  c.Description,
		Status:       c.Status,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

type courseOutput struct {
	Body CourseResponse
}

type CreateCourseRequest struct {
	Title       string `json:"title"                 minLength:"1" maxLength:"255"`
	Description string `json:"description,omitempty"`
}

type createCourseInput struct {
	Body CreateCourseRequest
}

func (h *Handler) create(ctx context.Context, in *createCourseInput) (*courseOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	c, err := h.service.Create(ctx, actor, CreateInput(in.Body))
	if err != nil {
		return nil, err
	}
	return &courseOutput{Body: toCourseResponse(c)}, nil
}

type courseIDInput struct {
	ID uuid.UUID `path:"id" format:"uuid"`
}

func (h *Handler) getByID(ctx context.Context, in *courseIDInput) (*courseOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	c, err := h.service.GetByID(ctx, actor, in.ID)
	if err != nil {
		return nil, err
	}
	return &courseOutput{Body: toCourseResponse(c)}, nil
}

type UpdateCourseRequest struct {
	Title       *string `json:"title,omitempty"       minLength:"1" maxLength:"255"`
	Description *string `json:"description,omitempty"`
}

type updateCourseInput struct {
	ID   uuid.UUID `path:"id" format:"uuid"`
	Body UpdateCourseRequest
}

func (h *Handler) update(ctx context.Context, in *updateCourseInput) (*courseOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	c, err := h.service.Update(ctx, actor, in.ID, UpdateInput(in.Body))
	if err != nil {
		return nil, err
	}
	return &courseOutput{Body: toCourseResponse(c)}, nil
}

type listCoursesInput struct {
	InstructorID uuid.UUID `query:"instructor_id" format:"uuid" doc:"Only courses of this instructor."`
	Limit        int       `query:"limit"         minimum:"1"   maximum:"100"                            default:"20"`
	Offset       int       `query:"offset"        minimum:"0"   default:"0"`
}

type CourseListResponse struct {
	Items  []CourseResponse `json:"items"`
	Total  int64            `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type courseListOutput struct {
	Body CourseListResponse
}

func (h *Handler) list(ctx context.Context, in *listCoursesInput) (*courseListOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	f := ListInput{Limit: in.Limit, Offset: in.Offset}
	if in.InstructorID != uuid.Nil {
		f.InstructorID = &in.InstructorID
	}

	res, err := h.service.List(ctx, actor, f)
	if err != nil {
		return nil, err
	}

	items := make([]CourseResponse, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toCourseResponse(&res.Items[i]))
	}
	return &courseListOutput{Body: CourseListResponse{
		Items:  items,
		Total:  res.Total,
		Limit:  res.Limit,
		Offset: res.Offset,
	}}, nil
}
