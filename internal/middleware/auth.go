package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"curtain/internal/models"
	"curtain/internal/services"
	"curtain/internal/token"

	"gorm.io/gorm"
)

type contextKey string

const (
	userIDKey      contextKey = "user_id"
	userRoleKey    contextKey = "user_role"
	projectRoleKey contextKey = "project_role"
)

// SessionCookieName is the HttpOnly session cookie used by web clients.
const SessionCookieName = "curtain_session"

// Authenticator validates sessions for both client types:
//   - web: HttpOnly `curtain_session` cookie
//   - CLI: `Authorization: Bearer <token>` header
//
// Opaque token hashes are checked against the sessions table; tokens are not JWTs.
type Authenticator struct {
	db *gorm.DB
}

func NewAuthenticator(db *gorm.DB) *Authenticator { return &Authenticator{db: db} }

func (a *Authenticator) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := RawToken(r)
		if raw == "" {
			writeJSONError(w, http.StatusUnauthorized, "missing session cookie or bearer token")
			return
		}

		var session models.Session
		if err := a.db.Preload("User").Where("token_hash = ?", token.Hash(raw)).First(&session).Error; err != nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid session")
			return
		}
		if session.User.IsDisabled() {
			a.db.Delete(&models.Session{}, session.ID)
			writeJSONError(w, http.StatusUnauthorized, "account is disabled")
			return
		}
		if time.Now().After(session.ExpiresAt) {
			a.db.Delete(&models.Session{}, session.ID)
			writeJSONError(w, http.StatusUnauthorized, "session expired")
			return
		}

		a.touch(session)

		ctx := context.WithValue(r.Context(), userIDKey, session.UserID)
		ctx = context.WithValue(ctx, userRoleKey, session.User.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// touch extends a session's sliding expiration when less than half its TTL remains.
func (a *Authenticator) touch(session models.Session) {
	ttl := services.SessionTTL(session.Client)
	if time.Until(session.ExpiresAt) > ttl/2 {
		return
	}
	a.db.Model(&models.Session{}).
		Where("id = ?", session.ID).
		Update("expires_at", time.Now().Add(ttl))
}

// RawToken extracts the token from a bearer header or session cookie.
func RawToken(r *http.Request) string {
	if header := r.Header.Get("Authorization"); header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		return cookie.Value
	}
	return ""
}

// UserIDFromContext returns the user ID stored by Authenticator.
func UserIDFromContext(ctx context.Context) (uint, bool) {
	id, ok := ctx.Value(userIDKey).(uint)
	return id, ok && id != 0
}

// UserRoleFromContext returns the instance role ("admin" or "member").
func UserRoleFromContext(ctx context.Context) string {
	role, _ := ctx.Value(userRoleKey).(string)
	return role
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
