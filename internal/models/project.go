package models

import "gorm.io/gorm"

// Project membership roles.
const (
	ProjectRoleOwner  = "owner"
	ProjectRoleEditor = "editor"
	ProjectRoleViewer = "viewer"
)

// Project groups environments. Secret values are stored only in encrypted form.
type Project struct {
	gorm.Model
	Name string `gorm:"not null" json:"name"`

	// Project members and their wrapped project keys.
	Members []User `gorm:"many2many:project_users;" json:"-"`
	// Environments in this project.
	Environments []Environment `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
}

// ProjectUser links a user to a project and stores their wrapped project key and role.
type ProjectUser struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	ProjectID           uint   `gorm:"not null;uniqueIndex:idx_project_user,priority:1" json:"project_id"`
	UserID              uint   `gorm:"not null;uniqueIndex:idx_project_user,priority:2" json:"user_id"`
	EncryptedProjectKey string `gorm:"type:text;not null" json:"encrypted_project_key"`
	Role                string `gorm:"not null;default:editor" json:"role"`

	Project Project `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
	User    User    `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
}

func (ProjectUser) TableName() string { return "project_users" }

func IsValidProjectRole(role string) bool {
	return role == ProjectRoleOwner || role == ProjectRoleEditor || role == ProjectRoleViewer
}
