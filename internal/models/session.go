package models

import "time"

// Session client types.
const (
	SessionClientWeb = "web"
	SessionClientCLI = "cli"
)

// Session is an opaque login session. The raw token stays with the client
// (an HttpOnly cookie or CLI bearer token); the database stores its SHA-256 hash.
type Session struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TokenHash string    `gorm:"uniqueIndex;not null" json:"-"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Client    string    `gorm:"not null;default:web" json:"client"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User User `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
}
