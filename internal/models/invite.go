package models

import (
	"time"

	"gorm.io/gorm"
)

// Invitation statuses are derived and are not stored in the database.
const (
	InviteStatusActive    = "active"
	InviteStatusExpired   = "expired"
	InviteStatusRevoked   = "revoked"
	InviteStatusExhausted = "exhausted"
)

// Invite is a generic, reusable registration link. The raw token is shown only
// when an admin creates it; the database stores its SHA-256 hash, as with sessions.
type Invite struct {
	gorm.Model
	TokenHash string     `gorm:"uniqueIndex;not null" json:"-"`
	MaxUses   int        `gorm:"not null;default:1" json:"max_uses"`
	Uses      int        `gorm:"not null;default:0" json:"uses"`
	ExpiresAt *time.Time `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at"`
	CreatedBy uint       `gorm:"not null;index" json:"created_by"`

	Creator User `gorm:"foreignKey:CreatedBy" json:"-"`
}

func (i *Invite) IsActive(now time.Time) bool {
	if i.RevokedAt != nil {
		return false
	}
	if i.ExpiresAt != nil && now.After(*i.ExpiresAt) {
		return false
	}
	return i.Uses < i.MaxUses
}

func (i *Invite) RemainingUses() int {
	remaining := i.MaxUses - i.Uses
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (i *Invite) Status(now time.Time) string {
	switch {
	case i.RevokedAt != nil:
		return InviteStatusRevoked
	case i.ExpiresAt != nil && now.After(*i.ExpiresAt):
		return InviteStatusExpired
	case i.Uses >= i.MaxUses:
		return InviteStatusExhausted
	default:
		return InviteStatusActive
	}
}
