package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"curtain/internal/authz"
	"curtain/internal/models"
	"curtain/internal/services"

	"github.com/casbin/casbin/v3"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// Authorize enforces project membership and capabilities.
type Authorize struct {
	db       *gorm.DB
	enforcer *casbin.SyncedEnforcer
}

func NewAuthorize(db *gorm.DB, enforcer *casbin.SyncedEnforcer) *Authorize {
	return &Authorize{db: db, enforcer: enforcer}
}

func isInstanceAdmin(r *http.Request) bool {
	return UserRoleFromContext(r.Context()) == models.UserRoleAdmin
}

// checkProjectCapability checks membership and the requested capability.
// It returns the member's project role and whether access is allowed.
func (a *Authorize) checkProjectCapability(
	w http.ResponseWriter,
	r *http.Request,
	projectID, userID uint,
	capability authz.Capability,
) (string, bool) {
	allowed, role, err := services.HasCapability(a.db, a.enforcer, projectID, userID, capability, isInstanceAdmin(r))
	if err != nil {
		if errors.Is(err, services.ErrForbidden) {
			writeJSONError(w, http.StatusForbidden, "forbidden: not a project member")
			return "", false
		}
		writeJSONError(w, http.StatusInternalServerError, "authorization check failed")
		return "", false
	}
	if !allowed {
		writeJSONError(w, http.StatusForbidden, fmt.Sprintf("forbidden: missing capability %q", capability))
		return "", false
	}
	return role, true
}

// withProjectRole stores the project role in the request context.
func withProjectRole(r *http.Request, role string) *http.Request {
	ctx := context.WithValue(r.Context(), projectRoleKey, role)
	return r.WithContext(ctx)
}

// RequireProjectMembership validates the project ID and requires the caller
// to be a member. It is used by read routes and stores the project role in the
// request context. It returns 404 for missing projects and 403 for non-members.
func (a *Authorize) RequireProjectMembership(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		projectID, err := parseUintParam(r, "projectID")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid project id")
			return
		}
		var project models.Project
		if err := a.db.First(&project, projectID).Error; err != nil {
			writeJSONError(w, http.StatusNotFound, "project not found")
			return
		}
		var membership models.ProjectUser
		if err := a.db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&membership).Error; err != nil {
			writeJSONError(w, http.StatusForbidden, "forbidden: not a project member")
			return
		}
		next.ServeHTTP(w, withProjectRole(r, membership.Role))
	})
}

// RequireProjectCapability protects routes under /projects/{projectID}.
func (a *Authorize) RequireProjectCapability(capability authz.Capability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			projectID, err := parseUintParam(r, "projectID")
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid project id")
				return
			}
			role, allowed := a.checkProjectCapability(w, r, projectID, userID, capability)
			if !allowed {
				return
			}
			next.ServeHTTP(w, withProjectRole(r, role))
		})
	}
}

// RequireEnvironmentMembership resolves the parent project and checks membership
// for read routes under /environments/{envID}.
func (a *Authorize) RequireEnvironmentMembership(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		envID, err := parseUintParam(r, "envID")
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid environment id")
			return
		}
		var env models.Environment
		if err := a.db.First(&env, envID).Error; err != nil {
			writeJSONError(w, http.StatusNotFound, "environment not found")
			return
		}
		var membership models.ProjectUser
		if err := a.db.Where("project_id = ? AND user_id = ?", env.ProjectID, userID).First(&membership).Error; err != nil {
			writeJSONError(w, http.StatusForbidden, "forbidden: not a project member")
			return
		}
		next.ServeHTTP(w, withProjectRole(r, membership.Role))
	})
}

// RequireEnvironmentCapability protects write routes under /environments/{envID}.
func (a *Authorize) RequireEnvironmentCapability(capability authz.Capability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			envID, err := parseUintParam(r, "envID")
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid environment id")
				return
			}
			var env models.Environment
			if err := a.db.First(&env, envID).Error; err != nil {
				writeJSONError(w, http.StatusNotFound, "environment not found")
				return
			}
			role, allowed := a.checkProjectCapability(w, r, env.ProjectID, userID, capability)
			if !allowed {
				return
			}
			next.ServeHTTP(w, withProjectRole(r, role))
		})
	}
}

// RequireVariableMembership resolves the parent project through the variable
// and its environment, then checks membership for read routes.
func (a *Authorize) RequireVariableMembership(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		projectID, ok := a.resolveVariableProject(w, r)
		if !ok {
			return
		}
		var membership models.ProjectUser
		if err := a.db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&membership).Error; err != nil {
			writeJSONError(w, http.StatusForbidden, "forbidden: not a project member")
			return
		}
		next.ServeHTTP(w, withProjectRole(r, membership.Role))
	})
}

// RequireVariableCapability protects write routes under /variables/{variableID}.
func (a *Authorize) RequireVariableCapability(capability authz.Capability) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			projectID, ok := a.resolveVariableProject(w, r)
			if !ok {
				return
			}
			role, allowed := a.checkProjectCapability(w, r, projectID, userID, capability)
			if !allowed {
				return
			}
			next.ServeHTTP(w, withProjectRole(r, role))
		})
	}
}

// resolveVariableProject resolves the project through a variable and environment.
// It writes an error response and returns false when resolution fails.
func (a *Authorize) resolveVariableProject(w http.ResponseWriter, r *http.Request) (uint, bool) {
	varID, err := parseUintParam(r, "variableID")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid variable id")
		return 0, false
	}
	var variable models.EnvironmentVariable
	if err := a.db.First(&variable, varID).Error; err != nil {
		writeJSONError(w, http.StatusNotFound, "variable not found")
		return 0, false
	}
	var env models.Environment
	if err := a.db.First(&env, variable.EnvironmentID).Error; err != nil {
		writeJSONError(w, http.StatusNotFound, "environment not found")
		return 0, false
	}
	return env.ProjectID, true
}

// ProjectRoleFromContext returns the project role ("owner", "editor", or "viewer").
func ProjectRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(projectRoleKey).(string)
	return role
}

func parseUintParam(r *http.Request, name string) (uint, error) {
	raw := chi.URLParam(r, name)
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}
