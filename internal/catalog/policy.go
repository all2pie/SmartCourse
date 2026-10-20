package catalog

import "github.com/all2pie/go-smart-course/internal/platform/auth"

// Access rules from PLAN.md section 2: students see published courses,
// instructors also see their own, admins see everything.

func canView(a auth.Actor, c *Course) bool {
	return a.Role == auth.RoleAdmin || c.Status == StatusPublished || c.InstructorID == a.ID
}

func canEdit(a auth.Actor, c *Course) bool {
	return a.Role == auth.RoleAdmin || c.InstructorID == a.ID
}

func restrictToVisible(a auth.Actor, f *ListFilter) {
	switch a.Role {
	case auth.RoleAdmin:
	case auth.RoleInstructor:
		f.PublishedOnly = true
		f.OwnerExempt = &a.ID
	default:
		f.PublishedOnly = true
	}
}
