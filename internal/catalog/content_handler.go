package catalog

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/httpapi"
)

const contentDescription = "Only the course's instructor or an admin. Changes go to the course's working copy and " +
	"are not visible to students until the course is published."

func (h *Handler) registerContent(api huma.API) {
	tags := []string{"content"}
	editErrors := []int{http.StatusForbidden, http.StatusNotFound, http.StatusConflict}

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "listModules",
		Method:      http.MethodGet,
		Path:        "/courses/{id}/modules",
		Tags:        tags,
		Summary:     "A course's modules and lessons, in order",
		Description: "Instructors and admins see the working copy; students see the published content.",
		Errors:      []int{http.StatusNotFound},
	}), h.listModules)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID:   "createModule",
		Method:        http.MethodPost,
		Path:          "/courses/{id}/modules",
		Tags:          tags,
		Summary:       "Add a module at the end of the course",
		Description:   contentDescription,
		DefaultStatus: http.StatusCreated,
		Errors:        editErrors,
	}), h.createModule)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "reorderModules",
		Method:      http.MethodPut,
		Path:        "/courses/{id}/modules/order",
		Tags:        tags,
		Summary:     "Reorder modules",
		Description: contentDescription + " The list must contain every module ID exactly once.",
		Errors:      editErrors,
	}), h.reorderModules)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "updateModule",
		Method:      http.MethodPatch,
		Path:        "/courses/{id}/modules/{module_id}",
		Tags:        tags,
		Summary:     "Rename a module",
		Description: contentDescription,
		Errors:      editErrors,
	}), h.updateModule)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID:   "deleteModule",
		Method:        http.MethodDelete,
		Path:          "/courses/{id}/modules/{module_id}",
		Tags:          tags,
		Summary:       "Delete a module and its lessons",
		Description:   contentDescription,
		DefaultStatus: http.StatusNoContent,
		Errors:        editErrors,
	}), h.deleteModule)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID:   "createLesson",
		Method:        http.MethodPost,
		Path:          "/courses/{id}/modules/{module_id}/lessons",
		Tags:          tags,
		Summary:       "Add a lesson at the end of a module",
		Description:   contentDescription + " Articles carry a `body`; video, document and external_link lessons carry an http(s) `url`.",
		DefaultStatus: http.StatusCreated,
		Errors:        editErrors,
	}), h.createLesson)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "reorderLessons",
		Method:      http.MethodPut,
		Path:        "/courses/{id}/modules/{module_id}/lessons/order",
		Tags:        tags,
		Summary:     "Reorder a module's lessons",
		Description: contentDescription + " The list must contain every lesson ID of the module exactly once.",
		Errors:      editErrors,
	}), h.reorderLessons)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID: "updateLesson",
		Method:      http.MethodPatch,
		Path:        "/courses/{id}/modules/{module_id}/lessons/{lesson_id}",
		Tags:        tags,
		Summary:     "Update a lesson's title, url or body",
		Description: contentDescription + " Omitted fields are unchanged; the lesson type can't change.",
		Errors:      editErrors,
	}), h.updateLesson)

	httpapi.Register(api, httpapi.Protected(huma.Operation{
		OperationID:   "deleteLesson",
		Method:        http.MethodDelete,
		Path:          "/courses/{id}/modules/{module_id}/lessons/{lesson_id}",
		Tags:          tags,
		Summary:       "Delete a lesson",
		Description:   contentDescription,
		DefaultStatus: http.StatusNoContent,
		Errors:        editErrors,
	}), h.deleteLesson)
}

type LessonResponse struct {
	ID    uuid.UUID  `json:"id"             format:"uuid"`
	Title string     `json:"title"`
	Type  LessonType `json:"type"           enum:"video,document,article,external_link"`
	URL   string     `json:"url,omitempty"`
	Body  string     `json:"body,omitempty"`
}

type ModuleResponse struct {
	ID      uuid.UUID        `json:"id"      format:"uuid"`
	Title   string           `json:"title"`
	Lessons []LessonResponse `json:"lessons"`
}

type ModuleListResponse struct {
	Items []ModuleResponse `json:"items"`
}

func toLessonResponse(l Lesson) LessonResponse {
	return LessonResponse(l)
}

func toModuleResponse(m Module) ModuleResponse {
	lessons := make([]LessonResponse, 0, len(m.Lessons))
	for _, l := range m.Lessons {
		lessons = append(lessons, toLessonResponse(l))
	}
	return ModuleResponse{ID: m.ID, Title: m.Title, Lessons: lessons}
}

func toModuleListResponse(modules []Module) ModuleListResponse {
	items := make([]ModuleResponse, 0, len(modules))
	for _, m := range modules {
		items = append(items, toModuleResponse(m))
	}
	return ModuleListResponse{Items: items}
}

type moduleOutput struct {
	Body ModuleResponse
}

type moduleListOutput struct {
	Body ModuleListResponse
}

type lessonOutput struct {
	Body LessonResponse
}

type moduleIDInput struct {
	ID       uuid.UUID `path:"id"        format:"uuid"`
	ModuleID uuid.UUID `path:"module_id" format:"uuid"`
}

type lessonIDInput struct {
	ID       uuid.UUID `path:"id"        format:"uuid"`
	ModuleID uuid.UUID `path:"module_id" format:"uuid"`
	LessonID uuid.UUID `path:"lesson_id" format:"uuid"`
}

func (h *Handler) listModules(ctx context.Context, in *courseIDInput) (*moduleListOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	modules, err := h.service.Modules(ctx, actor, in.ID)
	if err != nil {
		return nil, err
	}
	return &moduleListOutput{Body: toModuleListResponse(modules)}, nil
}

type ModuleRequest struct {
	Title string `json:"title" minLength:"1" maxLength:"255"`
}

type createModuleInput struct {
	ID   uuid.UUID `path:"id" format:"uuid"`
	Body ModuleRequest
}

func (h *Handler) createModule(ctx context.Context, in *createModuleInput) (*moduleOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	m, err := h.service.AddModule(ctx, actor, in.ID, in.Body.Title)
	if err != nil {
		return nil, err
	}
	return &moduleOutput{Body: toModuleResponse(m)}, nil
}

type updateModuleInput struct {
	ID       uuid.UUID `path:"id"        format:"uuid"`
	ModuleID uuid.UUID `path:"module_id" format:"uuid"`
	Body     ModuleRequest
}

func (h *Handler) updateModule(ctx context.Context, in *updateModuleInput) (*moduleOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	m, err := h.service.RenameModule(ctx, actor, in.ID, in.ModuleID, in.Body.Title)
	if err != nil {
		return nil, err
	}
	return &moduleOutput{Body: toModuleResponse(m)}, nil
}

func (h *Handler) deleteModule(ctx context.Context, in *moduleIDInput) (*struct{}, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.service.DeleteModule(ctx, actor, in.ID, in.ModuleID); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}

// IDList is a JSON array of UUID strings; Huma would otherwise describe items as plain strings.
type IDList []uuid.UUID

func (IDList) Schema(huma.Registry) *huma.Schema {
	maxItems := max(maxModules, maxLessonsPerModule)
	return &huma.Schema{
		Type:     huma.TypeArray,
		Items:    &huma.Schema{Type: huma.TypeString, Format: "uuid"},
		MaxItems: &maxItems,
	}
}

type ReorderModulesRequest struct {
	ModuleIDs IDList `json:"module_ids" doc:"Every module ID of the course, in the new order."`
}

type reorderModulesInput struct {
	ID   uuid.UUID `path:"id" format:"uuid"`
	Body ReorderModulesRequest
}

func (h *Handler) reorderModules(ctx context.Context, in *reorderModulesInput) (*moduleListOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	modules, err := h.service.ReorderModules(ctx, actor, in.ID, in.Body.ModuleIDs)
	if err != nil {
		return nil, err
	}
	return &moduleListOutput{Body: toModuleListResponse(modules)}, nil
}

type CreateLessonRequest struct {
	Title string     `json:"title"          minLength:"1" maxLength:"255"`
	Type  LessonType `json:"type"           enum:"video,document,article,external_link"`
	URL   string     `json:"url,omitempty"  maxLength:"2048"`
	Body  string     `json:"body,omitempty" maxLength:"100000"`
}

type createLessonInput struct {
	ID       uuid.UUID `path:"id"        format:"uuid"`
	ModuleID uuid.UUID `path:"module_id" format:"uuid"`
	Body     CreateLessonRequest
}

func (h *Handler) createLesson(ctx context.Context, in *createLessonInput) (*lessonOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	l, err := h.service.AddLesson(ctx, actor, in.ID, in.ModuleID, LessonInput(in.Body))
	if err != nil {
		return nil, err
	}
	return &lessonOutput{Body: toLessonResponse(l)}, nil
}

type UpdateLessonRequest struct {
	Title *string `json:"title,omitempty" minLength:"1" maxLength:"255"`
	URL   *string `json:"url,omitempty"   maxLength:"2048"`
	Body  *string `json:"body,omitempty"  maxLength:"100000"`
}

type updateLessonInput struct {
	ID       uuid.UUID `path:"id"        format:"uuid"`
	ModuleID uuid.UUID `path:"module_id" format:"uuid"`
	LessonID uuid.UUID `path:"lesson_id" format:"uuid"`
	Body     UpdateLessonRequest
}

func (h *Handler) updateLesson(ctx context.Context, in *updateLessonInput) (*lessonOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	l, err := h.service.UpdateLesson(ctx, actor, in.ID, in.ModuleID, in.LessonID, LessonUpdate(in.Body))
	if err != nil {
		return nil, err
	}
	return &lessonOutput{Body: toLessonResponse(l)}, nil
}

func (h *Handler) deleteLesson(ctx context.Context, in *lessonIDInput) (*struct{}, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.service.DeleteLesson(ctx, actor, in.ID, in.ModuleID, in.LessonID); err != nil {
		return nil, err
	}
	return &struct{}{}, nil
}

type ReorderLessonsRequest struct {
	LessonIDs IDList `json:"lesson_ids" doc:"Every lesson ID of the module, in the new order."`
}

type reorderLessonsInput struct {
	ID       uuid.UUID `path:"id"        format:"uuid"`
	ModuleID uuid.UUID `path:"module_id" format:"uuid"`
	Body     ReorderLessonsRequest
}

func (h *Handler) reorderLessons(ctx context.Context, in *reorderLessonsInput) (*moduleOutput, error) {
	actor, err := httpapi.Actor(ctx)
	if err != nil {
		return nil, err
	}

	m, err := h.service.ReorderLessons(ctx, actor, in.ID, in.ModuleID, in.Body.LessonIDs)
	if err != nil {
		return nil, err
	}
	return &moduleOutput{Body: toModuleResponse(m)}, nil
}
