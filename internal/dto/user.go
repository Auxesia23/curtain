package dto

import "time"

// --- User ---

type UserLookupResponse struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	PublicKey string `json:"public_key"`
}

// AdminUserResponse is used by the admin Settings page to list users.
type AdminUserResponse struct {
	ID         uint       `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	DisabledAt *time.Time `json:"disabled_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
