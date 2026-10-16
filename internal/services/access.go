package services

import (
	"errors"

	"curtain/internal/authz"
	"curtain/internal/models"

	"github.com/casbin/casbin/v3"
	"gorm.io/gorm"
)

// HasCapability requires project membership before evaluating a capability.
// Instance admins may bypass a capability denial for management actions, but
// they must still be project members, matching the API's documented behavior.
func HasCapability(
	db *gorm.DB,
	enforcer casbin.IEnforcer,
	projectID, userID uint,
	capability authz.Capability,
	isInstanceAdmin bool,
) (bool, string, error) {
	var membership models.ProjectUser
	if err := db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, "", ErrForbidden
		}
		return false, "", err
	}

	if isInstanceAdmin && authz.IsManagement(capability) {
		return true, membership.Role, nil
	}
	allowed, err := enforcer.Enforce(authz.UserSubject(userID), authz.ProjectDomain(projectID), string(capability))
	if err != nil {
		return false, "", err
	}
	return allowed, membership.Role, nil
}
