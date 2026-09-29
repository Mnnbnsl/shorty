package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"url-shortner/internal/auth"
	"url-shortner/internal/config"
	"url-shortner/internal/database"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrPasswordTooShort   = errors.New("password must be at least 6 characters")
	ErrInvalidEmail       = errors.New("invalid email address")
)

type UserService struct {
	cfg *config.Config
}

var UserSvc *UserService

func InitUserService(cfg *config.Config) {
	UserSvc = &UserService{cfg: cfg}
}

// Register creates a new user account, creates a refresh token, and returns tokens.
func (s *UserService) Register(ctx context.Context, email, password, displayName string) (*database.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, "", "", ErrInvalidEmail
	}
	if len(password) < 6 {
		return nil, "", "", ErrPasswordTooShort
	}
	if displayName == "" {
		parts := strings.Split(email, "@")
		displayName = parts[0]
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, "", "", fmt.Errorf("hash password: %w", err)
	}

	user, err := database.CreateUser(ctx, email, hash, displayName)
	if err != nil {
		return nil, "", "", err
	}

	accessToken, refreshToken, err := s.generateTokens(ctx, user.ID, user.Email)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

// Login verifies credentials and returns access & refresh tokens.
func (s *UserService) Login(ctx context.Context, email, password string) (*database.User, string, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := database.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, "", "", ErrInvalidCredentials
	}

	if !auth.CheckPassword(password, user.PasswordHash) {
		return nil, "", "", ErrInvalidCredentials
	}

	accessToken, refreshToken, err := s.generateTokens(ctx, user.ID, user.Email)
	if err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

// RefreshToken validates a refresh token, rotates it, and returns new tokens.
func (s *UserService) RefreshToken(ctx context.Context, rawRefreshToken string) (string, string, error) {
	if rawRefreshToken == "" {
		return "", "", errors.New("missing refresh token")
	}

	tokenHash := auth.HashToken(rawRefreshToken)
	tokenRecord, err := database.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return "", "", errors.New("invalid or expired refresh token")
	}

	user, err := database.GetUserByID(ctx, tokenRecord.UserID)
	if err != nil {
		return "", "", errors.New("user not found")
	}

	// Revoke old refresh token
	_ = database.DeleteRefreshToken(ctx, tokenHash)

	// Issue new token pair
	return s.generateTokens(ctx, user.ID, user.Email)
}

// Logout revokes the given refresh token.
func (s *UserService) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	tokenHash := auth.HashToken(rawRefreshToken)
	return database.DeleteRefreshToken(ctx, tokenHash)
}

// GetUserProfile loads user details by ID.
func (s *UserService) GetUserProfile(ctx context.Context, userID string) (*database.User, error) {
	return database.GetUserByID(ctx, userID)
}

func (s *UserService) generateTokens(ctx context.Context, userID, email string) (string, string, error) {
	accessToken, err := auth.GenerateAccessToken(userID, email, s.cfg.JWTSecret, s.cfg.JWTAccessTTL)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	rawRefresh, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	expiresAt := time.Now().Add(s.cfg.JWTRefreshTTL)
	if err := database.CreateRefreshToken(ctx, userID, refreshHash, expiresAt); err != nil {
		return "", "", fmt.Errorf("persist refresh token: %w", err)
	}

	return accessToken, rawRefresh, nil
}
