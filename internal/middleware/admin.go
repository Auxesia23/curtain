package middleware

import (
	"net/http"

	"curtain/internal/models"
)

// RequireAdmin restricts a route to instance admins. It must be installed
// after Authenticator, which adds the role to the request context.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserRoleFromContext(r.Context()) != models.UserRoleAdmin {
			writeJSONError(w, http.StatusForbidden, "forbidden: admin only")
			return
		}
		next.ServeHTTP(w, r)
	})
}
