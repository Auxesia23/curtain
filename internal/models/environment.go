package models

import "gorm.io/gorm"

// Environment is a namespace (for example, development, staging, or production)
// within a project.
type Environment struct {
	gorm.Model
	ProjectID uint   `gorm:"not null;uniqueIndex:idx_project_env_name,priority:1;index" json:"project_id"`
	Name      string `gorm:"not null;uniqueIndex:idx_project_env_name,priority:2" json:"name"`

	Project   Project               `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
	Variables []EnvironmentVariable `gorm:"constraint:OnDelete:CASCADE;" json:"variables,omitempty"`
}

// EnvironmentVariable stores one variable whose key and value are encrypted
// by the client. No unique constraint is applied to EncryptedKey because
// randomized encryption produces different ciphertext for the same plaintext.
type EnvironmentVariable struct {
	gorm.Model
	EnvironmentID  uint   `gorm:"not null;index" json:"environment_id"`
	EncryptedKey   string `gorm:"type:text;not null" json:"encrypted_key"`
	EncryptedValue string `gorm:"type:text;not null" json:"encrypted_value"`

	Environment Environment `gorm:"constraint:OnDelete:CASCADE;" json:"-"`
}
