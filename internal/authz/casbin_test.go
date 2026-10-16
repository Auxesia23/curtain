package authz

import (
	"path/filepath"
	"slices"
	"testing"

	"curtain/internal/database"
	"curtain/internal/models"
)

func TestCasbinPoliciesPersistAndStayProjectScoped(t *testing.T) {
	db, err := database.Connect(filepath.Join(t.TempDir(), "authz.db"))
	if err != nil {
		t.Fatalf("database.Connect: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("database handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	enforcer, err := NewEnforcer(db)
	if err != nil {
		t.Fatalf("NewEnforcer: %v", err)
	}

	projects := []models.Project{{Name: "project-one"}, {Name: "project-two"}}
	if err := db.Create(&projects).Error; err != nil {
		t.Fatalf("create projects: %v", err)
	}
	memberships := []models.ProjectUser{
		{ProjectID: projects[0].ID, UserID: 1, Role: models.ProjectRoleOwner, EncryptedProjectKey: "owner-key"},
		{ProjectID: projects[0].ID, UserID: 2, Role: models.ProjectRoleEditor, EncryptedProjectKey: "editor-key"},
		{ProjectID: projects[1].ID, UserID: 2, Role: models.ProjectRoleViewer, EncryptedProjectKey: "viewer-key"},
	}
	if err := db.Create(&memberships).Error; err != nil {
		t.Fatalf("create memberships: %v", err)
	}
	groupings := [][3]string{
		{UserSubject(1), RoleSubject(models.ProjectRoleOwner), ProjectDomain(projects[0].ID)},
		{UserSubject(2), RoleSubject(models.ProjectRoleEditor), ProjectDomain(projects[0].ID)},
		{UserSubject(2), RoleSubject(models.ProjectRoleViewer), ProjectDomain(projects[1].ID)},
	}
	for _, rule := range groupings {
		if _, err := enforcer.AddGroupingPolicy(rule[0], rule[1], rule[2]); err != nil {
			t.Fatalf("add grouping policy: %v", err)
		}
	}
	if _, err := enforcer.AddPolicy(UserSubject(2), ProjectDomain(projects[0].ID), string(VarUpdate), EffectDeny); err != nil {
		t.Fatalf("add deny override: %v", err)
	}
	if _, err := enforcer.AddPolicy(UserSubject(2), ProjectDomain(projects[0].ID), string(MemberInvite), EffectAllow); err != nil {
		t.Fatalf("add allow override: %v", err)
	}

	if !db.Migrator().HasTable("casbin_rule") {
		t.Fatal("Casbin adapter did not create casbin_rule")
	}
	if db.Migrator().HasTable("project_user_permissions") {
		t.Fatal("legacy project permission table should not be created")
	}

	checkPolicies(t, enforcer, projects)

	// The next enforcer instance loads its policies directly from SQLite.
	restarted, err := NewEnforcer(db)
	if err != nil {
		t.Fatalf("NewEnforcer after restart: %v", err)
	}
	checkPolicies(t, restarted, projects)

	domain := ProjectDomain(projects[0].ID)
	if _, err := restarted.RemoveFilteredPolicy(0, UserSubject(2), domain, string(VarUpdate)); err != nil {
		t.Fatalf("remove deny override: %v", err)
	}
	if _, err := restarted.AddPolicy(UserSubject(2), domain, string(VarUpdate), EffectAllow); err != nil {
		t.Fatalf("add allow override: %v", err)
	}

	thirdEnforcer, err := NewEnforcer(db)
	if err != nil {
		t.Fatalf("NewEnforcer after policy update: %v", err)
	}
	allowed, err := thirdEnforcer.Enforce(UserSubject(2), domain, string(VarUpdate))
	if err != nil || !allowed {
		t.Fatalf("updated persisted override = (%v, %v), want (true, nil)", allowed, err)
	}
}

func checkPolicies(t *testing.T, enforcer interface {
	Enforce(rvals ...interface{}) (bool, error)
	GetFilteredPolicy(fieldIndex int, fieldValues ...string) ([][]string, error)
}, projects []models.Project) {
	t.Helper()

	tests := []struct {
		name       string
		projectID  uint
		userID     uint
		capability Capability
		want       bool
	}{
		{"owner gets project management", projects[0].ID, 1, ProjectDelete, true},
		{"editor gets content permissions", projects[0].ID, 2, VarCreate, true},
		{"editor override allows invite", projects[0].ID, 2, MemberInvite, true},
		{"deny overrides editor role", projects[0].ID, 2, VarUpdate, false},
		{"same user has isolated role in another project", projects[1].ID, 2, VarCreate, false},
		{"override does not leak across projects", projects[1].ID, 2, MemberInvite, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := enforcer.Enforce(UserSubject(tt.userID), ProjectDomain(tt.projectID), string(tt.capability))
			if err != nil {
				t.Fatalf("Enforce: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Enforce() = %v, want %v", got, tt.want)
			}
		})
	}

	policies, err := enforcer.GetFilteredPolicy(0, UserSubject(2), ProjectDomain(projects[0].ID))
	if err != nil {
		t.Fatalf("GetFilteredPolicy: %v", err)
	}
	capabilities := make([]string, 0, len(policies))
	for _, policy := range policies {
		if len(policy) >= 4 && policy[3] == EffectAllow {
			capabilities = append(capabilities, policy[2])
		}
	}
	if !slices.Contains(capabilities, string(MemberInvite)) || slices.Contains(capabilities, string(VarUpdate)) {
		t.Fatalf("unexpected direct allow overrides: %v", capabilities)
	}
}
