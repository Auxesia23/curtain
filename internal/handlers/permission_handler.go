package handlers

import (
	"curtain/internal/authz"
	"curtain/internal/dto"
	"net/http"
)

// PermissionHandler serves the capability catalog and role templates enforced
// by the authorization middleware.
type PermissionHandler struct{}

func NewPermissionHandler() *PermissionHandler { return &PermissionHandler{} }

// Catalog: GET /api/permissions/catalog
func (h *PermissionHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	response := dto.PermissionCatalogResponse{
		Capabilities: make([]dto.CapabilityResponse, 0, len(authz.Catalog)),
		Roles:        make([]dto.RoleResponse, 0, len(authz.Roles)),
	}
	for _, info := range authz.Catalog {
		response.Capabilities = append(response.Capabilities, dto.CapabilityResponse{
			Capability:  string(info.Capability),
			Label:       info.Label,
			Description: info.Description,
			Group:       string(info.Group),
		})
	}
	for _, role := range authz.Roles {
		capabilities := make([]string, 0, len(role.Capabilities))
		for _, capability := range role.Capabilities {
			capabilities = append(capabilities, string(capability))
		}
		response.Roles = append(response.Roles, dto.RoleResponse{
			Role:         string(role.Role),
			Label:        role.Label,
			Description:  role.Description,
			Capabilities: capabilities,
		})
	}
	writeJSON(w, http.StatusOK, response)
}
