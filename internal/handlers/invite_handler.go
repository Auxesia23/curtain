package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"curtain/internal/dto"
	"curtain/internal/middleware"
	"curtain/internal/services"
	"curtain/internal/validation"

	"github.com/go-chi/chi/v5"
)

// InviteHandler handles admin invitation management and public validation.
type InviteHandler struct {
	svc *services.InviteService
}

func NewInviteHandler(svc *services.InviteService) *InviteHandler {
	return &InviteHandler{svc: svc}
}

// Create handles POST /api/admin/invites and returns the raw token once.
func (h *InviteHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req dto.CreateInviteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	invite, err := h.svc.Create(userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

// List handles GET /api/admin/invites.
func (h *InviteHandler) List(w http.ResponseWriter, r *http.Request) {
	invites, err := h.svc.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list invites")
		return
	}
	writeJSON(w, http.StatusOK, invites)
}

// Revoke handles DELETE /api/admin/invites/{inviteID}.
func (h *InviteHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "inviteID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invite id")
		return
	}
	if err := h.svc.Revoke(uint(id)); err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to revoke invite")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Validate handles GET /api/invites/{token}, used by /invite/<token>.
func (h *InviteHandler) Validate(w http.ResponseWriter, r *http.Request) {
	rawToken := strings.TrimSpace(chi.URLParam(r, "token"))
	if rawToken == "" {
		writeError(w, http.StatusBadRequest, "invite token is required")
		return
	}

	info, err := h.svc.Validate(rawToken)
	if err != nil {
		if errors.Is(err, services.ErrInviteNotFound) {
			writeJSON(w, http.StatusOK, dto.InviteInfoResponse{Valid: false})
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to validate invite")
		return
	}
	writeJSON(w, http.StatusOK, info)
}
