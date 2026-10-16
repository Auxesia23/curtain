package dto

import "curtain/internal/models"

// --- Auth ---

// KDFParams contains client-selected Argon2id parameters. Bounds prevent
// extreme values that could cause denial of service in the browser.
type KDFParams struct {
	Memory      uint32 `json:"m" validate:"required,min=8192,max=1048576"`
	Iterations  uint32 `json:"t" validate:"required,min=1,max=10"`
	Parallelism uint32 `json:"p" validate:"required,min=1,max=8"`
	HashLength  uint32 `json:"hash_length" validate:"required,min=32,max=64"`
}

type KDFConfig struct {
	Algorithm string    `json:"algorithm" validate:"required,oneof=argon2id"`
	Salt      string    `json:"salt" validate:"required"`
	Params    KDFParams `json:"params"`
}

// RegisterRequest contains the KDF-derived authKey (not the password), KDF
// settings, public key, and encrypted private key.
//
// InviteToken is required except during first-run setup.
type RegisterRequest struct {
	Email               string    `json:"email" validate:"required,email"`
	AuthKey             string    `json:"auth_key" validate:"required,min=43,max=128"`
	KDF                 KDFConfig `json:"kdf"`
	PublicKey           string    `json:"public_key" validate:"required"`
	EncryptedPrivateKey string    `json:"encrypted_private_key" validate:"required"`
	InviteToken         string    `json:"invite_token" validate:"omitempty,max=128"`
	Client              string    `json:"client" validate:"omitempty,oneof=web cli"`
}

type LoginRequest struct {
	Email   string `json:"email" validate:"required,email"`
	AuthKey string `json:"auth_key" validate:"required"`
	Client  string `json:"client" validate:"omitempty,oneof=web cli"`
}

// AuthResponse includes a token only for CLI clients; web clients use a cookie.
type AuthResponse struct {
	Token string                   `json:"token,omitempty"`
	User  models.UserPublicProfile `json:"user"`
}

// KDFResponse provides the salt and parameters required for pre-login key derivation.
type KDFResponse struct {
	KDFAlgorithm string `json:"kdf_algo"`
	KDFSalt      string `json:"kdf_salt"`
	KDFParams    string `json:"kdf_params"`
}

// InstanceResponse tells the SPA whether first-run setup is required.
type InstanceResponse struct {
	NeedsSetup bool `json:"needs_setup"`
}
