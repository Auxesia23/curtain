package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"curtain/internal/dto"
	"curtain/internal/services"
	"curtain/internal/validation"

	"github.com/go-chi/chi/v5"
)

type EnvironmentHandler struct {
	svc *services.EnvironmentService
}

func NewEnvironmentHandler(svc *services.EnvironmentService) *EnvironmentHandler {
	return &EnvironmentHandler{svc: svc}
}

// CreateEnvironment handles POST /projects/{projectID}/environments.
// Authorization is enforced by the router middleware.
func (h *EnvironmentHandler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)

	var req dto.CreateEnvironmentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	env, err := h.svc.Create(uint(pid), req)
	if err != nil {
		if errors.Is(err, services.ErrEnvironmentTaken) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create environment")
		return
	}
	writeJSON(w, http.StatusCreated, env)
}

// ListByProject handles GET /projects/{projectID}/environments.
func (h *EnvironmentHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	pid, _ := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)

	envs, err := h.svc.ListByProject(uint(pid))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	writeJSON(w, http.StatusOK, envs)
}

// DeleteEnvironment handles DELETE /environments/{envID} and deletes its variables.
func (h *EnvironmentHandler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
	eid, _ := strconv.ParseUint(chi.URLParam(r, "envID"), 10, 64)

	if err := h.svc.Delete(uint(eid)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete environment")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
