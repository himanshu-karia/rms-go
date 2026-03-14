package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"ingestion-go/internal/adapters/secondary"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

type AuthService struct {
	repo      *secondary.PostgresUserRepo
	jwtSecret []byte
}

type AuthTokens struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	SessionID        string
}

func NewAuthService(repo *secondary.PostgresUserRepo, secret string) *AuthService {
	return &AuthService{
		repo:      repo,
		jwtSecret: []byte(secret),
	}
}

func (s *AuthService) Register(username, password, role string) error {
	// 1. Hash Block
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 2. Save
	return s.repo.CreateUser(username, string(hash), role)
}

func (s *AuthService) RegisterWithSession(username, password, role string, ipAddress, userAgent *string) (*secondary.UserRecord, *AuthTokens, error) {
	if err := s.Register(username, password, role); err != nil {
		return nil, nil, err
	}
	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, errors.New("user not found after register")
	}
	return s.issueSessionTokens(user, ipAddress, userAgent)
}

func (s *AuthService) Login(username, password string) (string, error) {
	// 1. Find User
	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("user not found")
	}
	if !user.Active {
		return "", errors.New("user disabled")
	}

	// 2. Compare Hash
	err = bcrypt.CompareHashAndPassword([]byte(user.Hash), []byte(password))
	if err != nil {
		return "", errors.New("invalid password")
	}

	// 3. Issue Token
	claims := jwt.MapClaims{
		"id":   user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(time.Hour * 72).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) LoginWithSession(username, password string, ipAddress, userAgent *string) (*secondary.UserRecord, *AuthTokens, error) {
	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, errors.New("user not found")
	}
	if !user.Active {
		return nil, nil, errors.New("user disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Hash), []byte(password)); err != nil {
		return nil, nil, errors.New("invalid password")
	}

	return s.issueSessionTokens(user, ipAddress, userAgent)
}

func (s *AuthService) LoginWithGoogle(ctx context.Context, idToken string) (string, error) {
	// 1. Verify Google Token
	// Requires: "google.golang.org/api/idtoken"
	// Ensure audience matches our Client ID (from Env)
	clientId := os.Getenv("GOOGLE_CLIENT_ID")
	payload, err := idtoken.Validate(ctx, idToken, clientId)
	if err != nil {
		return "", fmt.Errorf("google auth failed: %v", err)
	}

	// 2. Extract Info
	email := payload.Claims["email"].(string)
	// name := payload.Claims["name"].(string)

	// 3. Find or Create User
	// We treat 'email' as the username for Google users
	user, err := s.repo.GetUserByUsername(email)
	if err != nil {
		return "", err
	}

	if user == nil {
		// Auto-Register as Viewer
		// Use a random password (they won't use it)
		// Better: Update DB to allow null password for OAuth users
		// For V1 parity, we generate a dummy hash
		dummyHash, _ := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), bcrypt.DefaultCost)
		err = s.repo.CreateUser(email, string(dummyHash), "viewer")
		if err != nil {
			return "", err
		}
		// Refetch
		user, _ = s.repo.GetUserByUsername(email)
	}

	// 4. Issue Internal Token (Same as Normal Login)
	claims := jwt.MapClaims{
		"id":   user.ID,
		"role": user.Role,
		"exp":  time.Now().Add(time.Hour * 72).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) RefreshSession(refreshToken string, ipAddress, userAgent *string) (*secondary.UserRecord, *AuthTokens, error) {
	if refreshToken == "" {
		return nil, nil, errors.New("refresh token required")
	}

	refreshHash := hashToken(refreshToken)
	session, err := s.repo.GetSessionByRefreshHash(refreshHash)
	if err != nil {
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, errors.New("invalid refresh token")
	}
	if session.RevokedAt != nil {
		return nil, nil, errors.New("refresh token revoked")
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, nil, errors.New("refresh token expired")
	}

	user, err := s.repo.GetUserByID(session.UserID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, errors.New("user not found")
	}
	if !user.Active {
		return nil, nil, errors.New("user disabled")
	}

	// rotate refresh token
	refreshTokenNew, refreshExpiresAt, err := generateRefreshToken()
	if err != nil {
		return nil, nil, err
	}
	refreshHashNew := hashToken(refreshTokenNew)
	if err := s.repo.RotateSessionRefresh(session.ID, refreshHashNew, refreshExpiresAt); err != nil {
		return nil, nil, err
	}

	tokens, err := s.issueAccessToken(user, session.ID)
	if err != nil {
		return nil, nil, err
	}
	tokens.RefreshToken = refreshTokenNew
	tokens.RefreshExpiresAt = refreshExpiresAt
	tokens.SessionID = session.ID
	return user, tokens, nil
}

func (s *AuthService) LogoutSession(sessionID string) error {
	if sessionID == "" {
		return errors.New("session id required")
	}
	return s.repo.RevokeSession(sessionID)
}

func (s *AuthService) ResetPassword(username, currentPassword, newPassword string) error {
	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Hash), []byte(currentPassword)); err != nil {
		return errors.New("invalid current password")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(user.ID, string(hash), false)
}

func (s *AuthService) ResetPasswordByUserID(userID, newPassword string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("user id required")
	}
	if strings.TrimSpace(newPassword) == "" {
		return errors.New("new password required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(userID, string(hash), false)
}

func (s *AuthService) GetUserByID(id string) (*secondary.UserRecord, error) {
	return s.repo.GetUserByID(id)
}

func (s *AuthService) GetUserByUsername(username string) (*secondary.UserRecord, error) {
	return s.repo.GetUserByUsername(username)
}

func (s *AuthService) EnsureMobileUser(username string) (*secondary.UserRecord, error) {
	if username == "" {
		return nil, errors.New("username required")
	}

	user, err := s.repo.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	dummyHash, err := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateUser(username, string(dummyHash), "viewer"); err != nil {
		return nil, err
	}
	return s.repo.GetUserByUsername(username)
}

func (s *AuthService) IssueSessionForUserID(userID string, ipAddress, userAgent *string) (*secondary.UserRecord, *AuthTokens, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, errors.New("user not found")
	}
	return s.issueSessionTokens(user, ipAddress, userAgent)
}

func (s *AuthService) ValidateToken(tokenString string) (map[string]interface{}, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, _ := claims["id"].(string)
		if strings.TrimSpace(userID) == "" {
			return nil, errors.New("invalid token claims")
		}

		sessionID, _ := claims["session_id"].(string)
		if strings.TrimSpace(sessionID) == "" {
			return nil, errors.New("invalid token claims")
		}

		session, err := s.repo.GetSessionByID(sessionID)
		if err != nil {
			return nil, err
		}
		if session == nil || session.UserID != userID {
			return nil, errors.New("session inactive")
		}
		if session.RevokedAt != nil {
			return nil, errors.New("session revoked")
		}
		if time.Now().After(session.ExpiresAt) {
			return nil, errors.New("session expired")
		}

		user, err := s.repo.GetUserByID(userID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, errors.New("user not found")
		}
		if !user.Active {
			return nil, errors.New("user disabled")
		}

		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// IssueDeviceToken creates a short-lived JWT for a device using project and device identifiers.
func (s *AuthService) IssueDeviceToken(projectID, deviceID string, ttl time.Duration) (string, error) {
	if projectID == "" || deviceID == "" {
		return "", errors.New("projectID and deviceID required")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	claims := jwt.MapClaims{
		"project_id": projectID,
		"device_id":  deviceID,
		"exp":        time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *AuthService) issueSessionTokens(user *secondary.UserRecord, ipAddress, userAgent *string) (*secondary.UserRecord, *AuthTokens, error) {
	refreshToken, refreshExpiresAt, err := generateRefreshToken()
	if err != nil {
		return nil, nil, err
	}
	refreshHash := hashToken(refreshToken)
	sessionID, err := s.repo.CreateSession(user.ID, refreshHash, refreshExpiresAt, ipAddress, userAgent)
	if err != nil {
		return nil, nil, err
	}

	tokens, err := s.issueAccessToken(user, sessionID)
	if err != nil {
		return nil, nil, err
	}
	tokens.RefreshToken = refreshToken
	tokens.RefreshExpiresAt = refreshExpiresAt
	tokens.SessionID = sessionID
	return user, tokens, nil
}

func (s *AuthService) issueAccessToken(user *secondary.UserRecord, sessionID string) (*AuthTokens, error) {
	capabilities, err := s.repo.ListCapabilitiesByUserID(user.ID)
	if err != nil {
		return nil, err
	}
	accessExpiresAt := time.Now().Add(1 * time.Hour)
	claims := jwt.MapClaims{
		"id":           user.ID,
		"role":         user.Role,
		"session_id":   sessionID,
		"capabilities": capabilities,
		"exp":          accessExpiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, err
	}
	return &AuthTokens{
		AccessToken:     accessToken,
		AccessExpiresAt: accessExpiresAt,
	}, nil
}

func generateRefreshToken() (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	refreshToken := hex.EncodeToString(buf)
	expiresAt := time.Now().Add(14 * 24 * time.Hour)
	return refreshToken, expiresAt, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
