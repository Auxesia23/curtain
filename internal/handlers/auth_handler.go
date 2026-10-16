package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"curtain/internal/dto"
	"curtain/internal/middleware"
	"curtain/internal/models"
	"curtain/internal/services"
	"curtain/internal/validation"
)

// AuthHandler handles registration and login requests and selects the session
// response format: an HttpOnly cookie for web clients or a JSON token for the CLI.
type AuthHandler struct {
	svc              *services.AuthService
	cookieSecureMode CookieSecureMode
}

type CookieSecureMode string

const (
	CookieSecureAuto  CookieSecureMode = "auto"
	CookieSecureTrue  CookieSecureMode = "true"
	CookieSecureFalse CookieSecureMode = "false"
)

func NewAuthHandler(svc *services.AuthService, cookieSecureMode CookieSecureMode) *AuthHandler {
	return &AuthHandler{svc: svc, cookieSecureMode: cookieSecureMode}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req dto.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.KDF.Salt = strings.TrimSpace(req.KDF.Salt)
	req.PublicKey = strings.TrimSpace(req.PublicKey)
	req.EncryptedPrivateKey = strings.TrimSpace(req.EncryptedPrivateKey)
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}
	if err := validation.Validate.Struct(req.KDF); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	res, err := h.svc.Register(req, r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, services.ErrEmailTaken):
			writeError(w, http.StatusConflict, err.Error())
		case errors.Is(err, services.ErrInviteRequired), errors.Is(err, services.ErrInviteInvalid):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to create user")
		}
		return
	}
	h.writeSession(w, r, res, http.StatusCreated)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dto.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if err := validation.Validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, validation.ErrorMessage(err))
		return
	}

	res, err := h.svc.Login(req, r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, err.Error())
		case errors.Is(err, services.ErrUserDisabled):
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "failed to create session")
		}
		return
	}
	h.writeSession(w, r, res, http.StatusOK)
}

// Instance is public and lets the SPA determine whether first-run setup is needed.
func (h *AuthHandler) Instance(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := h.svc.NeedsSetup()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to inspect instance")
		return
	}
	writeJSON(w, http.StatusOK, dto.InstanceResponse{NeedsSetup: needsSetup})
}

// KDF returns the user's salt and KDF parameters so the client can derive the
// authKey before login using the same settings as registration.
func (h *AuthHandler) KDF(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("email")))
	if err := validation.ValidateVar(email, "required,email"); err != nil {
		writeError(w, http.StatusBadRequest, "valid email is required")
		return
	}

	res, err := h.svc.KDFByEmail(email)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch kdf parameters")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// Logout revokes the server session and clears the cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Logout(middleware.RawToken(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}
	clearSessionCookie(w, r, h.cookieSecureMode)
	w.WriteHeader(http.StatusNoContent)
}

// Me returns the current user's profile, including the KDF salt and encrypted
// private key required by the client.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	profile, err := h.svc.Me(userID)
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to fetch profile")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// writeSession returns the result of registration/login: web clients receive an
// HttpOnly cookie, while CLI clients receive a token in the JSON body.
func (h *AuthHandler) writeSession(w http.ResponseWriter, r *http.Request, res services.SessionResult, status int) {
	body := dto.AuthResponse{User: res.User.PublicProfile()}
	if res.Client == models.SessionClientCLI {
		body.Token = res.Token
	} else {
		setSessionCookie(w, r, res.Token, services.SessionTTL(res.Client), h.cookieSecureMode)
	}
	writeJSON(w, status, body)
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, value string, ttl time.Duration, secureMode CookieSecureMode) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   cookieSecure(r, secureMode),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request, secureMode CookieSecureMode) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure(r, secureMode),
		SameSite: http.SameSiteLaxMode,
	})
}

// cookieSecure selects the Secure cookie attribute from explicit configuration.
// Auto mode enables it for HTTPS requests or TLS-terminating proxies.
func cookieSecure(r *http.Request, mode CookieSecureMode) bool {
	switch mode {
	case CookieSecureTrue:
		return true
	case CookieSecureFalse:
		return false
	}
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
