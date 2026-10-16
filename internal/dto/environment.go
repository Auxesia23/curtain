package dto

// --- Environment ---

type CreateEnvironmentRequest struct {
	Name string `json:"name" validate:"required"`
}
