package dto

// --- Variable ---

type UpsertVariableRequest struct {
	EncryptedKey   string `json:"encrypted_key" validate:"required"`
	EncryptedValue string `json:"encrypted_value" validate:"required"`
}
