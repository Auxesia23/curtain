package services

import (
	"errors"
	"path/filepath"
	"testing"

	"curtain/internal/authz"
	"curtain/internal/database"
	"curtain/internal/models"
)

func TestHasCapabilityRequiresMembershipBeforeAdminManagementBypass(t *testing.T) {
	db, err := database.Connect(filepath.Join(t.TempDir(), "access.db"))
	if err != nil {
		t.Fatalf("database.Connect: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	project := models.Project{Name: "access-test"}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	membership := models.ProjectUser{
		ProjectID: project.ID, UserID: 7, Role: models.ProjectRoleViewer,
		EncryptedProjectKey: "wrapped-key",
	}
	if err := db.Create(&membership).Error; err != nil {
		t.Fatalf("create membership: %v", err)
	}
	enforcer, err := authz.NewEnforcer(db)
	if err != nil {
		t.Fatalf("authz.NewEnforcer: %v", err)
	}
	if _, err := enforcer.AddGroupingPolicy(authz.UserSubject(7), authz.RoleSubject(models.ProjectRoleViewer), authz.ProjectDomain(project.ID)); err != nil {
		t.Fatalf("add viewer role: %v", err)
	}

	if _, _, err := HasCapability(db, enforcer, project.ID, 8, authz.MemberManage, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-member instance admin error = %v, want ErrForbidden", err)
	}

	allowed, role, err := HasCapability(db, enforcer, project.ID, 7, authz.MemberManage, true)
	if err != nil || !allowed || role != models.ProjectRoleViewer {
		t.Fatalf("member instance admin management check = (%v, %q, %v), want (true, viewer, nil)", allowed, role, err)
	}

	allowed, _, err = HasCapability(db, enforcer, project.ID, 7, authz.VarCreate, true)
	if err != nil || allowed {
		t.Fatalf("instance admin content check = (%v, %v), want (false, nil)", allowed, err)
	}
}
