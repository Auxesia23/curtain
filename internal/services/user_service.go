package services

import (
	"time"

	"curtain/internal/authz"
	"curtain/internal/dto"
	"curtain/internal/models"

	"github.com/casbin/casbin/v3"
	"gorm.io/gorm"
)

// UserService handles user lookup and instance-admin account operations.
type UserService struct {
	db       *gorm.DB
	enforcer *casbin.SyncedEnforcer
}

func NewUserService(db *gorm.DB, enforcer *casbin.SyncedEnforcer) *UserService {
	return &UserService{db: db, enforcer: enforcer}
}

func (s *UserService) LookupByEmail(email string) (dto.UserLookupResponse, error) {
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		return dto.UserLookupResponse{}, ErrUserNotFound
	}
	return dto.UserLookupResponse{ID: user.ID, Email: user.Email, PublicKey: user.PublicKey}, nil
}

func (s *UserService) GetByID(id any) (dto.UserLookupResponse, error) {
	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return dto.UserLookupResponse{}, ErrUserNotFound
	}
	return dto.UserLookupResponse{ID: user.ID, Email: user.Email, PublicKey: user.PublicKey}, nil
}

// ListAll returns users and account status, ordered by ID.
func (s *UserService) ListAll() ([]dto.AdminUserResponse, error) {
	var users []models.User
	if err := s.db.Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]dto.AdminUserResponse, 0, len(users))
	for _, user := range users {
		out = append(out, dto.AdminUserResponse{
			ID:         user.ID,
			Email:      user.Email,
			Role:       user.Role,
			DisabledAt: user.DisabledAt,
			CreatedAt:  user.CreatedAt,
		})
	}
	return out, nil
}

// SetDisabled suspends or reactivates a user. Suspending also revokes all sessions.
func (s *UserService) SetDisabled(actorID, targetID uint, disabled bool) error {
	if actorID == targetID {
		return ErrSelfAction
	}
	var target models.User
	if err := s.db.First(&target, targetID).Error; err != nil {
		return ErrUserNotFound
	}

	if disabled {
		if err := s.ensureNotLastAdmin(target, true); err != nil {
			return err
		}
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var disabledAt *time.Time
		if disabled {
			now := time.Now()
			disabledAt = &now
		}
		if err := tx.Model(&target).Update("disabled_at", disabledAt).Error; err != nil {
			return err
		}
		if disabled {
			return tx.Where("user_id = ?", target.ID).Delete(&models.Session{}).Error
		}
		return nil
	})
}

// Delete permanently removes a user.
//
//   - Projects for which the user is the sole owner are deleted (ownership transfer is not supported).
//   - Invitations created by the user are deleted.
//   - Project keys previously seen by the user are not rotated (deferred to Phase 3).
func (s *UserService) Delete(actorID, targetID uint) error {
	if actorID == targetID {
		return ErrSelfAction
	}
	var target models.User
	if err := s.db.First(&target, targetID).Error; err != nil {
		return ErrUserNotFound
	}
	if err := s.ensureNotLastAdmin(target, true); err != nil {
		return err
	}

	return authz.InTransaction(s.enforcer, func(tx *gorm.DB, enforcer casbin.IEnforcer) error {
		var ownerProjectIDs []uint
		if err := tx.Model(&models.ProjectUser{}).
			Where("user_id = ? AND role = ?", target.ID, models.ProjectRoleOwner).
			Pluck("project_id", &ownerProjectIDs).Error; err != nil {
			return err
		}
		deletedProjects := make(map[uint]bool, len(ownerProjectIDs))
		for _, projectID := range ownerProjectIDs {
			var ownerCount int64
			if err := tx.Model(&models.ProjectUser{}).
				Where("project_id = ? AND role = ?", projectID, models.ProjectRoleOwner).
				Count(&ownerCount).Error; err != nil {
				return err
			}
			if ownerCount == 1 {
				if err := authz.RemoveProjectPolicies(enforcer, projectID); err != nil {
					return err
				}
				if err := deleteProjectTx(tx, projectID); err != nil {
					return err
				}
				deletedProjects[projectID] = true
			}
		}

		var memberships []models.ProjectUser
		if err := tx.Where("user_id = ?", target.ID).Find(&memberships).Error; err != nil {
			return err
		}
		for _, membership := range memberships {
			if deletedProjects[membership.ProjectID] {
				continue
			}
			if err := authz.RemoveMemberPolicies(enforcer, membership.ProjectID, target.ID, membership.Role); err != nil {
				return err
			}
		}

		if err := tx.Where("created_by = ?", target.ID).Delete(&models.Invite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", target.ID).Delete(&models.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", target.ID).Delete(&models.ProjectUser{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.User{}, target.ID).Error
	})
}

// ensureNotLastAdmin prevents removing or disabling the last active instance admin.
func (s *UserService) ensureNotLastAdmin(target models.User, onlyWhenActive bool) error {
	if target.Role != models.UserRoleAdmin {
		return nil
	}
	if onlyWhenActive && target.DisabledAt != nil {
		return nil
	}
	var activeAdmins int64
	if err := s.db.Model(&models.User{}).
		Where("role = ? AND disabled_at IS NULL", models.UserRoleAdmin).
		Count(&activeAdmins).Error; err != nil {
		return err
	}
	if activeAdmins <= 1 {
		return ErrLastAdmin
	}
	return nil
}
