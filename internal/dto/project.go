package dto

// --- Project ---

type CreateProjectRequest struct {
	Name string `json:"name" validate:"required"`
	// Project key encrypted by the client with the owner's public key.
	EncryptedProjectKey string `json:"encrypted_project_key" validate:"required"`
}

type ProjectResponse struct {
	ID                  uint   `json:"id"`
	Name                string `json:"name"`
	Role                string `json:"role"`
	EncryptedProjectKey string `json:"encrypted_project_key"`
	// The caller's effective capabilities, used by the UI to hide unavailable actions.
	Capabilities []string `json:"capabilities,omitempty"`
}

type InviteRequest struct {
	// ID of the user to invite; the client resolves email to ID/public key first.
	UserID uint `json:"user_id" validate:"required"`
	// Project key encrypted by the inviting client with the invitee's public key.
	// The backend stores this value opaquely.
	EncryptedProjectKey string `json:"encrypted_project_key" validate:"required"`
	// Invitee's initial role: owner | editor | viewer (defaults to editor).
	Role string `json:"role" validate:"omitempty,oneof=owner editor viewer"`
}

// PermissionEntry is one explicit capability override.
type PermissionEntry struct {
	Capability string `json:"capability" validate:"required"`
	Effect     string `json:"effect" validate:"required,oneof=allow deny"`
}

type MemberResponse struct {
	UserID    uint   `json:"user_id"`
	Email     string `json:"email"`
	PublicKey string `json:"public_key"`
	Role      string `json:"role"`
	// Explicit overrides; an empty list means the role template applies unchanged.
	Overrides []PermissionEntry `json:"overrides"`
	// Effective capabilities after applying the role template and overrides.
	Capabilities []string `json:"capabilities"`
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=owner editor viewer"`
}

type UpdateMemberPermissionsRequest struct {
	Overrides []PermissionEntry `json:"overrides"`
}

// --- Permission catalog ---

type CapabilityResponse struct {
	Capability  string `json:"capability"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
}

type RoleResponse struct {
	Role         string   `json:"role"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
}

type PermissionCatalogResponse struct {
	Capabilities []CapabilityResponse `json:"capabilities"`
	Roles        []RoleResponse       `json:"roles"`
}
