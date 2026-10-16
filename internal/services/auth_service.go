package services

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	cryptoutil "curtain/internal/crypto"
	"curtain/internal/dto"
	"curtain/internal/models"
	"curtain/internal/token"

	"gorm.io/gorm"
)

const (
	sessionTTLWeb = 7 * 24 * time.Hour
	sessionTTLCLI = 30 * 24 * time.Hour
)

// AuthService handles registration, login, sessions, and the current user's profile.
//
// The server only receives the KDF-derived `authKey` and stores its hash.
// Private and project keys remain opaque to the server.
//
// Registration is open only during first-run setup, when the first user becomes
// admin. Later registrations require a valid invitation token.
type AuthService struct {
	db *gorm.DB

	// registerMu serializes registration within this process and prevents two
	// simultaneous first users in the single-binary SQLite deployment.
	// Multi-instance deployments should use a database lock instead.
	registerMu sync.Mutex
}

func NewAuthService(db *gorm.DB) *AuthService { return &AuthService{db: db} }

// SessionResult contains the result of registration or login.
type SessionResult struct {
	User   models.User
	Token  string // Raw token returned only to the client; the database stores its hash.
	Client string
}

// SessionTTL returns the session lifetime for a client type.
func SessionTTL(client string) time.Duration {
	if client == models.SessionClientCLI {
		return sessionTTLCLI
	}
	return sessionTTLWeb
}

func normalizeClient(client string) string {
	if client == models.SessionClientCLI {
		return models.SessionClientCLI
	}
	return models.SessionClientWeb
}

// NeedsSetup reports whether the instance has no users and needs first-run setup.
func (s *AuthService) NeedsSetup() (bool, error) {
	var count int64
	if err := s.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *AuthService) Register(req dto.RegisterRequest, userAgent string) (SessionResult, error) {
	authHash, err := cryptoutil.HashSecret(req.AuthKey)
	if err != nil {
		return SessionResult{}, err
	}
	params, err := json.Marshal(req.KDF.Params)
	if err != nil {
		return SessionResult{}, err
	}

	var user models.User
	s.registerMu.Lock()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Model(&models.User{}).Count(&userCount).Error; err != nil {
			return err
		}

		role := models.UserRoleMember
		if userCount == 0 {
			// Bootstrap: the first user becomes the instance admin.
			role = models.UserRoleAdmin
		} else {
			if strings.TrimSpace(req.InviteToken) == "" {
				return ErrInviteRequired
			}
			if _, err := consumeInviteTx(tx, req.InviteToken); err != nil {
				return err
			}
		}

		user = models.User{
			Email:               strings.ToLower(req.Email),
			Role:                role,
			AuthHash:            authHash,
			KDFAlgorithm:        req.KDF.Algorithm,
			KDFSalt:             req.KDF.Salt,
			KDFParams:           string(params),
			PublicKey:           req.PublicKey,
			EncryptedPrivateKey: req.EncryptedPrivateKey,
		}
		if err := tx.Create(&user).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrEmailTaken
			}
			return err
		}
		return nil
	})
	s.registerMu.Unlock()
	if err != nil {
		return SessionResult{}, err
	}
	return s.newSession(user, normalizeClient(req.Client), userAgent)
}

func (s *AuthService) Login(req dto.LoginRequest, userAgent string) (SessionResult, error) {
	var user models.User
	if err := s.db.Where("email = ?", strings.ToLower(req.Email)).First(&user).Error; err != nil {
		return SessionResult{}, ErrInvalidCredentials
	}
	if user.IsDisabled() {
		return SessionResult{}, ErrUserDisabled
	}
	if err := cryptoutil.VerifySecret(req.AuthKey, user.AuthHash); err != nil {
		return SessionResult{}, ErrInvalidCredentials
	}
	return s.newSession(user, normalizeClient(req.Client), userAgent)
}

// Me returns the current user's profile, including the KDF salt and encrypted
// private key required by the client to unlock the local vault.
func (s *AuthService) Me(userID uint) (models.UserPublicProfile, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return models.UserPublicProfile{}, ErrUserNotFound
	}
	return user.PublicProfile(), nil
}

// KDFByEmail returns the salt and KDF parameters needed by the client to derive
// the authKey before login.
func (s *AuthService) KDFByEmail(email string) (dto.KDFResponse, error) {
	var user models.User
	if err := s.db.Where("email = ?", strings.ToLower(email)).First(&user).Error; err != nil {
		return dto.KDFResponse{}, ErrUserNotFound
	}
	return dto.KDFResponse{
		KDFAlgorithm: user.KDFAlgorithm,
		KDFSalt:      user.KDFSalt,
		KDFParams:    user.KDFParams,
	}, nil
}

// Logout revokes the session identified by the raw token. It is idempotent.
func (s *AuthService) Logout(rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.db.Where("token_hash = ?", token.Hash(rawToken)).Delete(&models.Session{}).Error
}

func (s *AuthService) newSession(user models.User, client, userAgent string) (SessionResult, error) {
	raw, err := token.Generate()
	if err != nil {
		return SessionResult{}, err
	}
	session := models.Session{
		TokenHash: token.Hash(raw),
		UserID:    user.ID,
		Client:    client,
		UserAgent: truncateString(userAgent, 255),
		ExpiresAt: time.Now().Add(SessionTTL(client)),
	}
	if err := s.db.Create(&session).Error; err != nil {
		return SessionResult{}, err
	}
	return SessionResult{User: user, Token: raw, Client: client}, nil
}

func isUniqueViolation(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "unique") || strings.Contains(lower, "duplicate")
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
