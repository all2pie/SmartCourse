package catalog

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"

	"github.com/all2pie/go-smart-course/internal/platform/auth"
	"github.com/all2pie/go-smart-course/internal/platform/testutil"
)

func TestCanViewAndEdit(t *testing.T) {
	owner := testutil.Actor(auth.RoleInstructor)
	other := testutil.Actor(auth.RoleInstructor)
	student := testutil.Actor(auth.RoleStudent)
	admin := testutil.Actor(auth.RoleAdmin)
	draft := &Course{InstructorID: owner.ID, Status: StatusDraft}
	published := &Course{InstructorID: owner.ID, Status: StatusPublished}

	tests := []struct {
		name             string
		actor            auth.Actor
		course           *Course
		wantView, wantEd bool
	}{
		{"owner, draft", owner, draft, true, true},
		{"other instructor, draft", other, draft, false, false},
		{"student, draft", student, draft, false, false},
		{"admin, draft", admin, draft, true, true},
		{"other instructor, published", other, published, true, false},
		{"student, published", student, published, true, false},
		{"admin, published", admin, published, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canView(tt.actor, tt.course); got != tt.wantView {
				t.Errorf("canView = %v, want %v", got, tt.wantView)
			}
			if got := canEdit(tt.actor, tt.course); got != tt.wantEd {
				t.Errorf("canEdit = %v, want %v", got, tt.wantEd)
			}
		})
	}
}

func TestRestrictToVisible(t *testing.T) {
	instructor := testutil.Actor(auth.RoleInstructor)

	tests := []struct {
		actor auth.Actor
		want  ListFilter
	}{
		{testutil.Actor(auth.RoleAdmin), ListFilter{}},
		{instructor, ListFilter{PublishedOnly: true, OwnerExempt: &instructor.ID}},
		{testutil.Actor(auth.RoleStudent), ListFilter{PublishedOnly: true}},
		{auth.Actor{ID: uuid.New()}, ListFilter{PublishedOnly: true}},
	}
	for _, tt := range tests {
		t.Run(string(tt.actor.Role), func(t *testing.T) {
			var got ListFilter
			restrictToVisible(tt.actor, &got)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("filter mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
