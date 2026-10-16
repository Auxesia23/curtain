package services

import (
	"errors"

	"curtain/internal/authz"
	"curtain/internal/dto"
	"curtain/internal/models"

	"github.com/casbin/casbin/v3"
	"gorm.io/gorm"
)

type ProjectService struct {
	db       *gorm.DB
	enforcer *casbin.SyncedEnforcer
}

func NewProjectService(db *gorm.DB, enforcer *casbin.SyncedEnforcer) *ProjectService {
	return &ProjectService{db: db, enforcer: enforcer}
}

// CreateProject creates a project and adds its creator as owner in one transaction.
func (s *ProjectService) CreateProject(userID uint, req dto.CreateProjectRequest) (dto.ProjectResponse, error) {
	var project models.Project
	err := authz.InTransaction(s.enforcer, func(tx *gorm.DB, enforcer casbin.IEnforcer) error {
		project = models.Project{Name: req.Name}
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		membership := models.ProjectUser{
			ProjectID:           project.ID,
			UserID:              userID,
			EncryptedProjectKey: req.EncryptedProjectKey,
			Role:                models.ProjectRoleOwner,
		}
		if err := tx.Create(&membership).Error; err != nil {
			return err
		}
		_, err := enforcer.AddGroupingPolicy(authz.UserSubject(userID), authz.RoleSubject(membership.Role), authz.ProjectDomain(project.ID))
		return err
	})
	if err != nil {
		return dto.ProjectResponse{}, err
	}
	return dto.ProjectResponse{
		ID:                  project.ID,
		Name:                project.Name,
		Role:                models.ProjectRoleOwner,
		EncryptedProjectKey: req.EncryptedProjectKey,
	}, nil
}

// ListMyProjects returns all projects the user belongs to.
func (s *ProjectService) ListMyProjects(userID uint) ([]dto.ProjectResponse, error) {
	var memberships []models.ProjectUser
	if err := s.db.Where("user_id = ?", userID).Find(&memberships).Error; err != nil {
		return nil, err
	}
	if len(memberships) == 0 {
		return []dto.ProjectResponse{}, nil
	}
	ids := make([]uint, 0, len(memberships))
	byProject := make(map[uint]models.ProjectUser, len(memberships))
	for _, m := range memberships {
		ids = append(ids, m.ProjectID)
		byProject[m.ProjectID] = m
	}
	var projects []models.Project
	if err := s.db.Where("id IN ?", ids).Find(&projects).Error; err != nil {
		return nil, err
	}
	out := make([]dto.ProjectResponse, 0, len(projects))
	for _, p := range projects {
		m := byProject[p.ID]
		out = append(out, dto.ProjectResponse{
			ID:                  p.ID,
			Name:                p.Name,
			Role:                m.Role,
			EncryptedProjectKey: m.EncryptedProjectKey,
		})
	}
	return out, nil
}

// GetProject returns project details, the caller's membership, and effective
// capabilities used by the UI to show or hide actions.
func (s *ProjectService) GetProject(userID, projectID uint) (dto.ProjectResponse, error) {
	var project models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return dto.ProjectResponse{}, ErrProjectNotFound
	}
	var m models.ProjectUser
	if err := s.db.Where("project_id = ? AND user_id = ?", project.ID, userID).First(&m).Error; err != nil {
		return dto.ProjectResponse{}, ErrForbidden
	}
	caps, err := authz.Effective(s.enforcer, project.ID, userID)
	if err != nil {
		return dto.ProjectResponse{}, err
	}
	return dto.ProjectResponse{
		ID:                  project.ID,
		Name:                project.Name,
		Role:                m.Role,
		EncryptedProjectKey: m.EncryptedProjectKey,
		Capabilities:        caps,
	}, nil
}

// InviteUser adds a member to a project. The default role is editor; inviting
// someone as owner requires the member.manage capability.
func (s *ProjectService) InviteUser(
	actorID, projectID uint,
	req dto.InviteRequest,
	isInstanceAdmin bool,
) (models.ProjectUser, error) {
	role := req.Role
	if role == "" {
		role = models.ProjectRoleEditor
	}
	if !authz.IsValidRole(role) {
		return models.ProjectUser{}, ErrNotMember
	}
	if role == models.ProjectRoleOwner {
		allowed, _, err := HasCapability(s.db, s.enforcer, projectID, actorID, authz.MemberManage, isInstanceAdmin)
		if err != nil {
			return models.ProjectUser{}, err
		}
		if !allowed {
			return models.ProjectUser{}, ErrInviteOwnerForbidden
		}
	}

	var invitee models.User
	if err := s.db.First(&invitee, req.UserID).Error; err != nil {
		return models.ProjectUser{}, ErrUserNotFound
	}
	membership := models.ProjectUser{
		ProjectID:           projectID,
		UserID:              req.UserID,
		EncryptedProjectKey: req.EncryptedProjectKey,
		Role:                role,
	}
	if err := authz.InTransaction(s.enforcer, func(tx *gorm.DB, enforcer casbin.IEnforcer) error {
		var existing models.ProjectUser
		if err := tx.Where("project_id = ? AND user_id = ?", projectID, req.UserID).First(&existing).Error; err == nil {
			return ErrAlreadyMember
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Create(&membership).Error; err != nil {
			return err
		}
		_, err := enforcer.AddGroupingPolicy(authz.UserSubject(req.UserID), authz.RoleSubject(role), authz.ProjectDomain(projectID))
		return err
	}); err != nil {
		return models.ProjectUser{}, err
	}
	return membership, nil
}

// SetMemberRole changes a member's role while preventing removal of the last owner.
func (s *ProjectService) SetMemberRole(projectID, targetUserID uint, role string) error {
	if !authz.IsValidRole(role) {
		return ErrNotMember
	}
	return authz.InTransaction(s.enforcer, func(tx *gorm.DB, enforcer casbin.IEnforcer) error {
		membership, err := findMembership(tx, projectID, targetUserID)
		if err != nil {
			return err
		}
		if membership.Role == role {
			return nil
		}
		if membership.Role == models.ProjectRoleOwner && role != models.ProjectRoleOwner {
			if err := ensureNotLastOwner(tx, projectID); err != nil {
				return err
			}
		}
		if _, err := enforcer.RemoveGroupingPolicy(authz.UserSubject(targetUserID), authz.RoleSubject(membership.Role), authz.ProjectDomain(projectID)); err != nil {
			return err
		}
		if _, err := enforcer.AddGroupingPolicy(authz.UserSubject(targetUserID), authz.RoleSubject(role), authz.ProjectDomain(projectID)); err != nil {
			return err
		}
		return tx.Model(&membership).Update("role", role).Error
	})
}

// SetMemberPermissions replaces all capability overrides for a member.
func (s *ProjectService) SetMemberPermissions(
	projectID, targetUserID uint,
	overrides []dto.PermissionEntry,
) error {
	seen := make(map[string]bool, len(overrides))
	for _, entry := range overrides {
		if !authz.IsValidCapability(entry.Capability) {
			return ErrInvalidCapability
		}
		if !authz.IsValidEffect(entry.Effect) {
			return ErrInvalidCapability
		}
		if seen[entry.Capability] {
			return ErrDuplicateCapability
		}
		seen[entry.Capability] = true
	}

	return authz.InTransaction(s.enforcer, func(tx *gorm.DB, enforcer casbin.IEnforcer) error {
		if _, err := findMembership(tx, projectID, targetUserID); err != nil {
			return err
		}
		if _, err := enforcer.RemoveFilteredPolicy(0, authz.UserSubject(targetUserID), authz.ProjectDomain(projectID)); err != nil {
			return err
		}
		for _, entry := range overrides {
			if _, err := enforcer.AddPolicy(authz.UserSubject(targetUserID), authz.ProjectDomain(projectID), entry.Capability, entry.Effect); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveMember removes a project member and their wrapped project key. The last
// project owner cannot be removed.
func (s *ProjectService) RemoveMember(projectID, targetUserID uint) error {
	return authz.InTransaction(s.enforcer, func(tx *gorm.DB, enforcer casbin.IEnforcer) error {
		membership, err := findMembership(tx, projectID, targetUserID)
		if err != nil {
			return err
		}
		if membership.Role == models.ProjectRoleOwner {
			if err := ensureNotLastOwner(tx, projectID); err != nil {
				return err
			}
		}
		if err := authz.RemoveMemberPolicies(enforcer, projectID, targetUserID, membership.Role); err != nil {
			return err
		}
		return tx.Delete(&models.ProjectUser{}, membership.ID).Error
	})
}

// DeleteProject removes a project, its environments, variables, and memberships.
// RequireProjectCapability(project.delete) checks access before this method runs.
func (s *ProjectService) DeleteProject(projectID uint) error {
	var project models.Project
	if err := s.db.First(&project, projectID).Error; err != nil {
		return ErrProjectNotFound
	}
	return authz.InTransaction(s.enforcer, func(tx *gorm.DB, enforcer casbin.IEnforcer) error {
		if err := authz.RemoveProjectPolicies(enforcer, projectID); err != nil {
			return err
		}
		return deleteProjectTx(tx, projectID)
	})
}

// deleteProjectTx explicitly removes a project and its related data instead of
// relying on SQLite foreign-key cascades.
func deleteProjectTx(tx *gorm.DB, projectID uint) error {
	var environmentIDs []uint
	if err := tx.Model(&models.Environment{}).
		Where("project_id = ?", projectID).
		Pluck("id", &environmentIDs).Error; err != nil {
		return err
	}
	if len(environmentIDs) > 0 {
		if err := tx.Where("environment_id IN ?", environmentIDs).
			Delete(&models.EnvironmentVariable{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("project_id = ?", projectID).Delete(&models.Environment{}).Error; err != nil {
		return err
	}
	if err := tx.Where("project_id = ?", projectID).Delete(&models.ProjectUser{}).Error; err != nil {
		return err
	}
	return tx.Delete(&models.Project{}, projectID).Error
}

// ListMembers returns project members, roles, explicit overrides, and effective capabilities.
func (s *ProjectService) ListMembers(projectID uint) ([]dto.MemberResponse, error) {
	var memberships []models.ProjectUser
	if err := s.db.Where("project_id = ?", projectID).Find(&memberships).Error; err != nil {
		return nil, err
	}

	userIDs := make([]uint, 0, len(memberships))
	for _, m := range memberships {
		userIDs = append(userIDs, m.UserID)
	}
	var users []models.User
	if len(userIDs) > 0 {
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, err
		}
	}
	emailByUser := make(map[uint]models.User, len(users))
	for _, u := range users {
		emailByUser[u.ID] = u
	}

	out := make([]dto.MemberResponse, 0, len(memberships))
	for _, m := range memberships {
		user := emailByUser[m.UserID]
		policies, err := s.enforcer.GetFilteredPolicy(0, authz.UserSubject(m.UserID), authz.ProjectDomain(projectID))
		if err != nil {
			return nil, err
		}
		entries := make([]dto.PermissionEntry, 0, len(policies))
		for _, policy := range policies {
			if len(policy) >= 4 {
				entries = append(entries, dto.PermissionEntry{Capability: policy[2], Effect: policy[3]})
			}
		}
		out = append(out, dto.MemberResponse{
			UserID:    m.UserID,
			Email:     user.Email,
			PublicKey: user.PublicKey,
			Role:      m.Role,
			Overrides: entries,
		})
		capabilities, err := authz.Effective(s.enforcer, projectID, m.UserID)
		if err != nil {
			return nil, err
		}
		out[len(out)-1].Capabilities = capabilities
	}
	return out, nil
}

func findMembership(db *gorm.DB, projectID, userID uint) (models.ProjectUser, error) {
	var membership models.ProjectUser
	if err := db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&membership).Error; err != nil {
		return models.ProjectUser{}, ErrNotMember
	}
	return membership, nil
}

func ensureNotLastOwner(db *gorm.DB, projectID uint) error {
	var owners int64
	if err := db.Model(&models.ProjectUser{}).
		Where("project_id = ? AND role = ?", projectID, models.ProjectRoleOwner).
		Count(&owners).Error; err != nil {
		return err
	}
	if owners <= 1 {
		return ErrLastOwner
	}
	return nil
}
