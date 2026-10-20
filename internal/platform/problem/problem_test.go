package problem_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/go-cmp/cmp"

	"github.com/all2pie/go-smart-course/internal/platform/errs"
	"github.com/all2pie/go-smart-course/internal/platform/problem"
)

func TestFrom(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantDetail string
	}{
		{"not found", errs.New(errs.NotFound, "course not found"), http.StatusNotFound, "course not found"},
		{"invalid", errs.New(errs.Invalid, "bad input"), http.StatusUnprocessableEntity, "bad input"},
		{"conflict", errs.New(errs.Conflict, "taken"), http.StatusConflict, "taken"},
		{"unauthenticated", errs.New(errs.Unauthenticated, "who"), http.StatusUnauthorized, "who"},
		{"forbidden", errs.New(errs.Forbidden, "no"), http.StatusForbidden, "no"},
		{"wrapped sentinel", fmt.Errorf("ctx: %w", errs.New(errs.NotFound, "gone")), http.StatusNotFound, "gone"},
		{"internal kind hides message", errs.New(errs.Internal, "db password is hunter2"), http.StatusInternalServerError, "internal server error"},
		{"unknown error hides message", errors.New("dial tcp 10.0.0.5:5432"), http.StatusInternalServerError, "internal server error"},
		{"status error passes through", huma.Error409Conflict("already"), http.StatusConflict, "already"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := problem.From(tt.err).(*huma.ErrorModel)
			if !ok {
				t.Fatalf("From returned %T, want *huma.ErrorModel", problem.From(tt.err))
			}
			if got.Status != tt.wantStatus || got.Detail != tt.wantDetail || len(got.Errors) != 0 {
				t.Fatalf("From = %d %q %v, want %d %q no errors", got.Status, got.Detail, got.Errors, tt.wantStatus, tt.wantDetail)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	problem.Write(rec, http.StatusBadGateway, "upstream unavailable")

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != problem.ContentType {
		t.Fatalf("Content-Type = %q", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"title": "Bad Gateway", "status": float64(502), "detail": "upstream unavailable"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("body mismatch (-want +got):\n%s", diff)
	}
}
