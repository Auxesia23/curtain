package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"curtain/internal/dto"
	"curtain/internal/middleware"
	"curtain/internal/models"
	"curtain/internal/services"
	"curtain/internal/validation"

	"github.com/go-chi/chi/v5"
)

type ProjectHandler struct {
	svc *services.ProjectService
}

func NewProjectHandler(svc *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

// CreateProject creates a project and adds its creator as owner.
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req dto.CreateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.EncryptedProjectKey = strings.TrimSpace(req.EncryptedProjectKey)
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	res, err := h.svc.CreateProject(userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project")
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

// ListMyProjects returns all projects the current user belongs to.
func (h *ProjectHandler) ListMyProjects(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	out, err := h.svc.ListMyProjects(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// GetProject returns project details and the caller's membership.
// RequireProjectMembership checks access before this handler runs.
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	userID, _ := middleware.UserIDFromContext(r.Context())
	pid, _ := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)

	res, err := h.svc.GetProject(userID, uint(pid))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrProjectNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		case errors.Is(err, services.ErrForbidden):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to fetch project")
		}
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// InviteUser adds a member to the project.
// RequireProjectCapability(member.invite) checks access. Inviting someone as
// owner also requires member.manage, which is checked by the service.
func (h *ProjectHandler) InviteUser(w http.ResponseWriter, r *http.Request) {
	actorID, _ := middleware.UserIDFromContext(r.Context())
	pid, _ := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)

	var req dto.InviteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.EncryptedProjectKey = strings.TrimSpace(req.EncryptedProjectKey)
	req.Role = strings.TrimSpace(req.Role)
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	isAdmin := middleware.UserRoleFromContext(r.Context()) == models.UserRoleAdmin
	membership, err := h.svc.InviteUser(actorID, uint(pid), req, isAdmin)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrUserNotFound):
			writeError(w, http.StatusNotFound, "invited user not found")
		case errors.Is(err, services.ErrAlreadyMember):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, services.ErrInviteOwnerForbidden):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to invite user")
		}
		return
	}
	writeJSON(w, http.StatusCreated, membership)
}

// UpdateMemberRole handles PATCH /projects/{projectID}/members/{userID}.
func (h *ProjectHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	pid, uid, ok := memberParams(w, r)
	if !ok {
		return
	}

	var req dto.UpdateMemberRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Role = strings.TrimSpace(req.Role)
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	if err := h.svc.SetMemberRole(pid, uid, req.Role); err != nil {
		h.writeMemberError(w, err, "failed to update member role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": uid, "role": req.Role})
}

// UpdateMemberPermissions handles PUT /projects/{projectID}/members/{userID}/permissions.
// The request replaces all explicit overrides for the member.
func (h *ProjectHandler) UpdateMemberPermissions(w http.ResponseWriter, r *http.Request) {
	pid, uid, ok := memberParams(w, r)
	if !ok {
		return
	}

	var req dto.UpdateMemberPermissionsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	for _, entry := range req.Overrides {
		if err := validation.Validate.Struct(entry); err != nil {
			writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
			return
		}
	}

	if err := h.svc.SetMemberPermissions(pid, uid, req.Overrides); err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCapability), errors.Is(err, services.ErrDuplicateCapability):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			h.writeMemberError(w, err, "failed to update member permissions")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RemoveMember: DELETE /projects/{projectID}/members/{userID}
func (h *ProjectHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	pid, uid, ok := memberParams(w, r)
	if !ok {
		return
	}
	if err := h.svc.RemoveMember(pid, uid); err != nil {
		h.writeMemberError(w, err, "failed to remove member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func memberParams(w http.ResponseWriter, r *http.Request) (uint, uint, bool) {
	pid, err := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return 0, 0, false
	}
	uid, err := strconv.ParseUint(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return 0, 0, false
	}
	return uint(pid), uint(uid), true
}

func (h *ProjectHandler) writeMemberError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, services.ErrNotMember):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrLastOwner):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

// ListMembers returns project members without exposing other members' wrapped keys.
func (h *ProjectHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)

	out, err := h.svc.ListMembers(uint(pid))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// DeleteProject removes the project and its contents. The middleware requires
// the project.delete capability.
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if err := h.svc.DeleteProject(uint(pid)); err != nil {
		if errors.Is(err, services.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete project")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
