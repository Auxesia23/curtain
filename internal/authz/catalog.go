// Package authz defines project capabilities, role templates, and Casbin helpers.
// Project membership grants read access; capabilities control write and management actions.
package authz

import "curtain/internal/models"

// Role is a project membership role.
type Role string

const (
	RoleOwner  Role = Role(models.ProjectRoleOwner)
	RoleEditor Role = Role(models.ProjectRoleEditor)
	RoleViewer Role = Role(models.ProjectRoleViewer)
)

// Capability is a project action that can be granted to a member.
type Capability string

const (
	EnvCreate     Capability = "env.create"
	EnvDelete     Capability = "env.delete"
	VarCreate     Capability = "var.create"
	VarUpdate     Capability = "var.update"
	VarDelete     Capability = "var.delete"
	MemberInvite  Capability = "member.invite"
	MemberManage  Capability = "member.manage"
	ProjectDelete Capability = "project.delete"
)

const (
	EffectAllow = "allow"
	EffectDeny  = "deny"
)

// Group is used to group capabilities in the permissions UI.
type Group string

const (
	GroupContent Group = "content"
	GroupMembers Group = "members"
	GroupProject Group = "project"
)

// CapabilityInfo describes a capability exposed in the API catalog.
type CapabilityInfo struct {
	Capability  Capability `json:"capability"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Group       Group      `json:"group"`
}

// Catalog is the complete capability list, ordered for display in the UI.
var Catalog = []CapabilityInfo{
	{EnvCreate, "Create environments", "Add new environments to the project", GroupContent},
	{EnvDelete, "Delete environments", "Delete environments and all their variables", GroupContent},
	{VarCreate, "Create variables", "Add variables to environments", GroupContent},
	{VarUpdate, "Edit variables", "Change existing variable names and values", GroupContent},
	{VarDelete, "Delete variables", "Remove variables from environments", GroupContent},
	{MemberInvite, "Invite members", "Add new members to the project", GroupMembers},
	{MemberManage, "Manage members", "Change roles, permissions, and remove members", GroupMembers},
	{ProjectDelete, "Delete project", "Permanently delete the project and its data", GroupProject},
}

// RoleInfo describes a role and its default capabilities.
type RoleInfo struct {
	Role         Role         `json:"role"`
	Label        string       `json:"label"`
	Description  string       `json:"description"`
	Capabilities []Capability `json:"capabilities"`
}

var (
	contentCapabilities = []Capability{EnvCreate, EnvDelete, VarCreate, VarUpdate, VarDelete}

	// The owner role includes every capability in the catalog.
	Roles = []RoleInfo{
		{
			Role:        RoleOwner,
			Label:       "Owner",
			Description: "Full control: content, members, and the project itself.",
			Capabilities: append(append([]Capability{}, contentCapabilities...),
				MemberInvite, MemberManage, ProjectDelete),
		},
		{
			Role:         RoleEditor,
			Label:        "Editor",
			Description:  "Can read and manage environments and variables, but not members.",
			Capabilities: contentCapabilities,
		},
		{
			Role:         RoleViewer,
			Label:        "Viewer",
			Description:  "Read-only access to the project.",
			Capabilities: []Capability{},
		},
	}
)

var catalogIndex = func() map[Capability]CapabilityInfo {
	index := make(map[Capability]CapabilityInfo, len(Catalog))
	for _, info := range Catalog {
		index[info.Capability] = info
	}
	return index
}()

// IsValidCapability reports whether value is in the capability catalog.
func IsValidCapability(value string) bool {
	_, ok := catalogIndex[Capability(value)]
	return ok
}

// IsValidRole reports whether value is a supported project role.
func IsValidRole(value string) bool {
	return value == string(RoleOwner) || value == string(RoleEditor) || value == string(RoleViewer)
}

// IsManagement identifies capabilities that an instance admin may bypass
// while acting as a project member. Content capabilities are never bypassed.
func IsManagement(cap Capability) bool {
	switch cap {
	case MemberInvite, MemberManage, ProjectDelete:
		return true
	default:
		return false
	}
}
