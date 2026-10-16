package dto

import "time"

// --- Invite ---

// CreateInviteRequest uses defaults (five uses, seven days) when values are zero.
type CreateInviteRequest struct {
	MaxUses        int `json:"max_uses" validate:"omitempty,min=1,max=100"`
	ExpiresInHours int `json:"expires_in_hours" validate:"omitempty,min=1,max=8760"`
}

// InviteResponse includes `token` and `url` only at creation time. The raw token
// cannot be shown again afterward.
type InviteResponse struct {
	ID           uint       `json:"id"`
	Token        string     `json:"token,omitempty"`
	URL          string     `json:"url,omitempty"`
	MaxUses      int        `json:"max_uses"`
	Uses         int        `json:"uses"`
	ExpiresAt    *time.Time `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at"`
	Status       string     `json:"status"`
	CreatedBy    uint       `json:"created_by"`
	CreatedEmail string     `json:"created_by_email,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// InviteInfoResponse is returned by the public invitation validation endpoint.
type InviteInfoResponse struct {
	Valid         bool       `json:"valid"`
	ExpiresAt     *time.Time `json:"expires_at"`
	RemainingUses int        `json:"remaining_uses"`
}
