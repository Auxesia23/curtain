package services

import (
	"time"

	"curtain/internal/dto"
	"curtain/internal/models"
	"curtain/internal/token"

	"gorm.io/gorm"
)

const (
	inviteDefaultMaxUses  = 5
	inviteDefaultTTLHours = 7 * 24
	inviteMaxExpiryHours  = 24 * 365
)

// InviteService manages generic, reusable registration links. The raw token is
// returned only once at creation; the database stores its hash.
type InviteService struct {
	db *gorm.DB
}

func NewInviteService(db *gorm.DB) *InviteService { return &InviteService{db: db} }

func (s *InviteService) Create(adminID uint, req dto.CreateInviteRequest) (dto.InviteResponse, error) {
	maxUses := req.MaxUses
	if maxUses <= 0 {
		maxUses = inviteDefaultMaxUses
	}
	expiresInHours := req.ExpiresInHours
	if expiresInHours <= 0 {
		expiresInHours = inviteDefaultTTLHours
	}
	if expiresInHours > inviteMaxExpiryHours {
		expiresInHours = inviteMaxExpiryHours
	}

	raw, err := token.Generate()
	if err != nil {
		return dto.InviteResponse{}, err
	}
	expiresAt := time.Now().Add(time.Duration(expiresInHours) * time.Hour)

	invite := models.Invite{
		TokenHash: token.Hash(raw),
		MaxUses:   maxUses,
		ExpiresAt: &expiresAt,
		CreatedBy: adminID,
	}
	if err := s.db.Create(&invite).Error; err != nil {
		return dto.InviteResponse{}, err
	}

	return dto.InviteResponse{
		ID:        invite.ID,
		Token:     raw,
		URL:       "/invite/" + raw,
		MaxUses:   invite.MaxUses,
		Uses:      invite.Uses,
		ExpiresAt: invite.ExpiresAt,
		Status:    invite.Status(time.Now()),
		CreatedBy: invite.CreatedBy,
		CreatedAt: invite.CreatedAt,
	}, nil
}

func (s *InviteService) List() ([]dto.InviteResponse, error) {
	var invites []models.Invite
	if err := s.db.Preload("Creator").Order("id DESC").Find(&invites).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]dto.InviteResponse, 0, len(invites))
	for _, invite := range invites {
		out = append(out, dto.InviteResponse{
			ID:           invite.ID,
			MaxUses:      invite.MaxUses,
			Uses:         invite.Uses,
			ExpiresAt:    invite.ExpiresAt,
			RevokedAt:    invite.RevokedAt,
			Status:       invite.Status(now),
			CreatedBy:    invite.CreatedBy,
			CreatedEmail: invite.Creator.Email,
			CreatedAt:    invite.CreatedAt,
		})
	}
	return out, nil
}

func (s *InviteService) Revoke(id uint) error {
	var invite models.Invite
	if err := s.db.First(&invite, id).Error; err != nil {
		return ErrInviteNotFound
	}
	if invite.RevokedAt != nil {
		return nil
	}
	now := time.Now()
	return s.db.Model(&invite).Update("revoked_at", now).Error
}

// Validate checks an invitation when a user opens /invite/<token>.
func (s *InviteService) Validate(rawToken string) (dto.InviteInfoResponse, error) {
	var invite models.Invite
	if err := s.db.Where("token_hash = ?", token.Hash(rawToken)).First(&invite).Error; err != nil {
		return dto.InviteInfoResponse{}, ErrInviteNotFound
	}
	now := time.Now()
	return dto.InviteInfoResponse{
		Valid:         invite.IsActive(now),
		ExpiresAt:     invite.ExpiresAt,
		RemainingUses: invite.RemainingUses(),
	}, nil
}

// consumeInviteTx validates an invitation and increments its use count inside
// the registration transaction. It is used after first-run setup.
func consumeInviteTx(tx *gorm.DB, rawToken string) (models.Invite, error) {
	var invite models.Invite
	if err := tx.Where("token_hash = ?", token.Hash(rawToken)).First(&invite).Error; err != nil {
		return models.Invite{}, ErrInviteInvalid
	}
	if !invite.IsActive(time.Now()) {
		return models.Invite{}, ErrInviteInvalid
	}
	if err := tx.Model(&invite).Update("uses", gorm.Expr("uses + 1")).Error; err != nil {
		return models.Invite{}, err
	}
	invite.Uses++
	return invite, nil
}
