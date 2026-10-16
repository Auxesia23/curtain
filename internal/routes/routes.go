package routes

import (
	"net/http"

	"curtain/internal/authz"
	"curtain/internal/handlers"
	"curtain/internal/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// NewRouter configures the resource API routes and SPA handler.
// Authorization is provided by *middleware.Authorize; authentication is
// provided by *middleware.Authenticator (HttpOnly cookie for web, bearer for CLI).
//
// Endpoints:
//
//	POST /api/auth/register            (public, rate-limited; invite required except during setup)
//	POST /api/auth/login               (public, rate-limited)
//	GET  /api/auth/kdf?email=          (public, rate-limited; KDF salt and parameters)
//	GET  /api/auth/me                  (auth)
//	POST /api/auth/logout              (auth)
//
//	GET  /api/instance                 (public; needs_setup for first-run setup)
//	GET  /api/invites/{token}          (public, rate-limited; validate an invite)
//
//	GET  /api/admin/invites            (admin)
//	POST /api/admin/invites            (admin; raw token returned once)
//	DELETE /api/admin/invites/{inviteID} (admin; revoke)
//	GET  /api/admin/users              (admin)
//	POST /api/admin/users/{userID}/disable|enable (admin)
//	DELETE /api/admin/users/{userID}   (admin)
//
//	GET  /api/users/lookup?email=      (auth, for the invite flow)
//	GET  /api/users/{userID}           (auth)
//
//	GET  /api/projects                 (auth, caller's projects)
//	POST /api/projects                 (auth)
//	GET  /api/projects/{projectID}                    (member)
//	GET  /api/projects/{projectID}/members            (member)
//	POST /api/projects/{projectID}/invite             (owner only)
//	GET  /api/projects/{projectID}/environments      (member)
//	POST /api/projects/{projectID}/environments      (member)
//
//	GET  /api/environments/{envID}/variables          (member via env)
//	POST /api/environments/{envID}/variables          (member via env)
//	DELETE /api/environments/{envID}                  (member via env)
//
//	PUT    /api/variables/{variableID}                (member via variable)
//	DELETE /api/variables/{variableID}                (member via variable)
//
//	/*                                                 (built SPA from ui/)
func NewRouter(
	authH *handlers.AuthHandler,
	inviteH *handlers.InviteHandler,
	userH *handlers.UserHandler,
	projectH *handlers.ProjectHandler,
	envH *handlers.EnvironmentHandler,
	varH *handlers.VariableHandler,
	permissionH *handlers.PermissionHandler,
	authorizer *middleware.Authorize,
	authn *middleware.Authenticator,
	webUI http.Handler,
) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	// Reject cross-origin cookie requests (CSRF protection).
	r.Use(middleware.CSRFProtect)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	// Rate-limit authentication endpoints by IP to reduce brute-force attempts.
	authLimiter := middleware.NewRateLimiter(20, 20)

	r.Route("/api", func(r chi.Router) {
		// Public instance info used by the SPA during first-run setup.
		r.Get("/instance", authH.Instance)

		// Public, rate-limited invitation validation.
		r.With(authLimiter.Middleware).Get("/invites/{token}", inviteH.Validate)

		r.Route("/auth", func(r chi.Router) {
			r.With(authLimiter.Middleware).Post("/register", authH.Register)
			r.With(authLimiter.Middleware).Post("/login", authH.Login)
			r.With(authLimiter.Middleware).Get("/kdf", authH.KDF)
			r.With(authn.Handle).Get("/me", authH.Me)
			r.With(authn.Handle).Post("/logout", authH.Logout)
		})

		// Instance-admin routes for invitation and user management.
		r.Route("/admin", func(r chi.Router) {
			r.Use(authn.Handle)
			r.Use(middleware.RequireAdmin)

			r.Route("/invites", func(r chi.Router) {
				r.Get("/", inviteH.List)
				r.Post("/", inviteH.Create)
				r.Delete("/{inviteID}", inviteH.Revoke)
			})

			r.Route("/users", func(r chi.Router) {
				r.Get("/", userH.ListAll)
				r.Post("/{userID}/disable", userH.SetDisabled(true))
				r.Post("/{userID}/enable", userH.SetDisabled(false))
				r.Delete("/{userID}", userH.Delete)
			})
		})

		// All resources below require a valid session (cookie or bearer token).
		r.Group(func(r chi.Router) {
			r.Use(authn.Handle)

			r.Route("/users", func(r chi.Router) {
				r.Get("/lookup", userH.LookupByEmail)
				r.Get("/{userID}", userH.GetByID)
			})

			r.Route("/projects", func(r chi.Router) {
				r.Get("/", projectH.ListMyProjects)
				r.Post("/", projectH.CreateProject)

				// Project-scoped routes require project membership.
				r.Route("/{projectID}", func(r chi.Router) {
					r.Use(authorizer.RequireProjectMembership)

					r.Get("/", projectH.GetProject)
					r.Get("/members", projectH.ListMembers)
					r.Get("/environments", envH.ListByProject)

					// Write and management actions require the matching capability.
					r.With(authorizer.RequireProjectCapability(authz.EnvCreate)).
						Post("/environments", envH.CreateEnvironment)
					r.With(authorizer.RequireProjectCapability(authz.MemberInvite)).
						Post("/invite", projectH.InviteUser)
					r.With(authorizer.RequireProjectCapability(authz.MemberManage)).
						Patch("/members/{userID}", projectH.UpdateMemberRole)
					r.With(authorizer.RequireProjectCapability(authz.MemberManage)).
						Put("/members/{userID}/permissions", projectH.UpdateMemberPermissions)
					r.With(authorizer.RequireProjectCapability(authz.MemberManage)).
						Delete("/members/{userID}", projectH.RemoveMember)
					r.With(authorizer.RequireProjectCapability(authz.ProjectDelete)).
						Delete("/", projectH.DeleteProject)
				})
			})

			// Capability catalog and role templates for the UI.
			r.Get("/permissions/catalog", permissionH.Catalog)

			// Resolve project membership through the environment.
			r.Route("/environments", func(r chi.Router) {
				r.Route("/{envID}", func(r chi.Router) {
					r.With(authorizer.RequireEnvironmentMembership).Get("/variables", varH.ListByEnvironment)
					r.With(authorizer.RequireEnvironmentCapability(authz.VarCreate)).
						Post("/variables", varH.CreateVariable)
					r.With(authorizer.RequireEnvironmentCapability(authz.EnvDelete)).
						Delete("/", envH.DeleteEnvironment)
				})
			})

			// Resolve project membership through the variable and environment.
			r.Route("/variables", func(r chi.Router) {
				r.Route("/{variableID}", func(r chi.Router) {
					r.With(authorizer.RequireVariableCapability(authz.VarUpdate)).
						Put("/", varH.UpdateVariable)
					r.With(authorizer.RequireVariableCapability(authz.VarDelete)).
						Delete("/", varH.DeleteVariable)
				})
			})
		})
	})

	// Serve the SPA for all remaining client-side routes.
	if webUI != nil {
		r.Handle("/*", webUI)
	}

	return r
}
