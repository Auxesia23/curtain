package models

import (
	"time"

	"gorm.io/gorm"
)

// Instance-level user roles.
const (
	UserRoleAdmin  = "admin"
	UserRoleMember = "member"
)

// User stores identity data and public cryptographic material. The backend
// never sees plaintext passwords, private keys, or project keys.
//
//	Role                : "admin" (instance management) or "member".
//	DisabledAt          : a non-nil value prevents login and invalidates existing sessions.
//	AuthHash            : server-side Argon2id hash of the client-derived auth key.
//	KDFAlgorithm/Params : client KDF settings used to derive keys at the next login.
//	KDFSalt             : the user's KDF salt, encoded as base64.
//	PublicKey           : RSA-OAEP public key (base64 SPKI) used to share project keys.
//	EncryptedPrivateKey : client-encrypted private key, opaque to the backend.
type User struct {
	gorm.Model
	Email               string     `gorm:"uniqueIndex;not null" json:"email"`
	Role                string     `gorm:"not null;default:member" json:"role"`
	DisabledAt          *time.Time `json:"disabled_at"`
	AuthHash            string     `gorm:"not null" json:"-"`
	KDFAlgorithm        string     `gorm:"not null;default:argon2id" json:"kdf_algo"`
	KDFSalt             string     `gorm:"not null" json:"kdf_salt"`
	KDFParams           string     `gorm:"type:text" json:"kdf_params"`
	PublicKey           string     `gorm:"type:text;not null" json:"public_key"`
	EncryptedPrivateKey string     `gorm:"type:text;not null" json:"encrypted_private_key"`

	// Many-to-many relationship with projects through ProjectUser.
	Projects []Project `gorm:"many2many:project_users;" json:"-"`
}

func (u *User) IsAdmin() bool    { return u.Role == UserRoleAdmin }
func (u *User) IsDisabled() bool { return u.DisabledAt != nil }

func IsValidUserRole(role string) bool {
	return role == UserRoleAdmin || role == UserRoleMember
}

// PublicProfile is the safe API representation of a user. AuthHash is never serialized.
type UserPublicProfile struct {
	ID                  uint   `json:"id"`
	Email               string `json:"email"`
	Role                string `json:"role"`
	KDFAlgorithm        string `json:"kdf_algo"`
	KDFSalt             string `json:"kdf_salt"`
	KDFParams           string `json:"kdf_params"`
	PublicKey           string `json:"public_key"`
	EncryptedPrivateKey string `json:"encrypted_private_key"`
}

func (u *User) PublicProfile() UserPublicProfile {
	return UserPublicProfile{
		ID:                  u.ID,
		Email:               u.Email,
		Role:                u.Role,
		KDFAlgorithm:        u.KDFAlgorithm,
		KDFSalt:             u.KDFSalt,
		KDFParams:           u.KDFParams,
		PublicKey:           u.PublicKey,
		EncryptedPrivateKey: u.EncryptedPrivateKey,
	}
}
