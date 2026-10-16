package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"curtain/internal/dto"
	"curtain/internal/services"
	"curtain/internal/validation"

	"github.com/go-chi/chi/v5"
)

// VariableHandler provides CRUD operations for EnvironmentVariable.
// Payloads are opaque because the client encrypts them before sending.
type VariableHandler struct {
	svc *services.VariableService
}

func NewVariableHandler(svc *services.VariableService) *VariableHandler {
	return &VariableHandler{svc: svc}
}

// CreateVariable handles POST /environments/{envID}/variables.
func (h *VariableHandler) CreateVariable(w http.ResponseWriter, r *http.Request) {
	eid, _ := strconv.ParseUint(chi.URLParam(r, "envID"), 10, 64)

	var req dto.UpsertVariableRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	v, err := h.svc.Create(uint(eid), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create variable")
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

// ListByEnvironment handles GET /environments/{envID}/variables.
func (h *VariableHandler) ListByEnvironment(w http.ResponseWriter, r *http.Request) {
	eid, _ := strconv.ParseUint(chi.URLParam(r, "envID"), 10, 64)

	vars, err := h.svc.ListByEnvironment(uint(eid))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list variables")
		return
	}
	writeJSON(w, http.StatusOK, vars)
}

// UpdateVariable handles PUT /variables/{variableID}.
func (h *VariableHandler) UpdateVariable(w http.ResponseWriter, r *http.Request) {
	vid, _ := strconv.ParseUint(chi.URLParam(r, "variableID"), 10, 64)

	var req dto.UpsertVariableRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	v, err := h.svc.Update(uint(vid), req)
	if err != nil {
		if errors.Is(err, services.ErrVariableNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update variable")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// DeleteVariable handles DELETE /variables/{variableID}.
func (h *VariableHandler) DeleteVariable(w http.ResponseWriter, r *http.Request) {
	vid, _ := strconv.ParseUint(chi.URLParam(r, "variableID"), 10, 64)

	if err := h.svc.Delete(uint(vid)); err != nil {
		if errors.Is(err, services.ErrVariableNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete variable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
