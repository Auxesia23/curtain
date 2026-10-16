package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"curtain/internal/middleware"
	"curtain/internal/services"

	"github.com/go-chi/chi/v5"
)

// UserHandler handles user lookup and instance-admin account operations.
type UserHandler struct {
	svc *services.UserService
}

func NewUserHandler(svc *services.UserService) *UserHandler { return &UserHandler{svc: svc} }

func (h *UserHandler) LookupByEmail(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		writeError(w, http.StatusBadRequest, "email query parameter is required")
		return
	}
	res, err := h.svc.LookupByEmail(email)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to lookup user")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.GetByID(chi.URLParam(r, "userID"))
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch user")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ListAll handles GET /api/admin/users.
func (h *UserHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// SetDisabled handles account disable/enable requests.
func (h *UserHandler) SetDisabled(disabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := middleware.UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		targetID, err := strconv.ParseUint(chi.URLParam(r, "userID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user id")
			return
		}

		if err := h.svc.SetDisabled(actorID, uint(targetID), disabled); err != nil {
			h.writeUserError(w, err, "failed to update user")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": uint(targetID), "disabled": disabled})
	}
}

// Delete handles DELETE /api/admin/users/{userID}.
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actorID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	targetID, err := strconv.ParseUint(chi.URLParam(r, "userID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	if err := h.svc.Delete(actorID, uint(targetID)); err != nil {
		h.writeUserError(w, err, "failed to delete user")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) writeUserError(w http.ResponseWriter, err error, fallback string) {
	switch {
	case errors.Is(err, services.ErrUserNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrSelfAction), errors.Is(err, services.ErrLastAdmin):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, fallback)
	}
}
