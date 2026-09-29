package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"url-shortner/internal/cache"
	"url-shortner/internal/database"
	"url-shortner/internal/validation"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var (
	ErrAliasLoginRequired = errors.New("custom aliases require an account. Please log in or register")
	ErrLinkExpired        = errors.New("short link has expired")
)

type URLService struct {
	ephemeralTTL time.Duration
}

var URLSvc *URLService

func InitURLService(ephemeralTTL time.Duration) {
	URLSvc = &URLService{
		ephemeralTTL: ephemeralTTL,
	}
}

// ShortenURL handles shortening with optional custom alias and user attribution.
func (s *URLService) ShortenURL(ctx context.Context, originalURL, customAlias string, userID *string) (*database.URLRecord, error) {
	customAlias = strings.TrimSpace(customAlias)

	var shortCode string
	var isCustom bool
	var isEphemeral bool
	var expiresAt *time.Time
	var id *int64

	// Determine expiration and ownership
	if userID == nil {
		isEphemeral = true
		exp := time.Now().Add(s.ephemeralTTL)
		expiresAt = &exp
	}

	if customAlias != "" {
		if userID == nil {
			return nil, ErrAliasLoginRequired
		}
		if err := validation.ValidateCustomAlias(customAlias); err != nil {
			return nil, err
		}
		shortCode = customAlias
		isCustom = true
	} else {
		// Auto-generate Base62 code using PostgreSQL sequence
		nextSeq, err := database.GetNextURLSequence(ctx)
		if err != nil {
			return nil, fmt.Errorf("generate id: %w", err)
		}
		id = &nextSeq
		shortCode = EncodeBase62(nextSeq)
	}

	record, err := database.InsertURL(ctx, id, originalURL, shortCode, isCustom, userID, isEphemeral, expiresAt)
	if err != nil {
		return nil, err
	}

	// Cache in Redis: format is "ID|ORIGINAL_URL"
	cacheVal := fmt.Sprintf("%d|%s", record.ID, record.OriginalURL)
	var cacheTTL time.Duration
	if isEphemeral {
		cacheTTL = s.ephemeralTTL
	} else {
		cacheTTL = 24 * time.Hour
	}

	if cache.Client != nil {
		_ = cache.Client.Set(ctx, shortCode, cacheVal, cacheTTL)
	}

	return record, nil
}

// ResolveURL retrieves the original URL and its database ID for redirection and analytics.
func (s *URLService) ResolveURL(ctx context.Context, shortCode string) (string, int64, bool) {
	// 1. Check Redis cache
	if cache.Client != nil {
		if val, ok := cache.Client.Get(ctx, shortCode); ok && val != "" {
			parts := strings.SplitN(val, "|", 2)
			if len(parts) == 2 {
				urlID, _ := strconv.ParseInt(parts[0], 10, 64)
				return parts[1], urlID, true
			}
			// Legacy or plain url format fallback
			return val, 0, true
		}
	}

	// 2. Query PostgreSQL
	record, err := database.GetOriginalURL(ctx, shortCode)
	if err != nil {
		return "", 0, false
	}

	// 3. Populate Redis
	if cache.Client != nil {
		cacheVal := fmt.Sprintf("%d|%s", record.ID, record.OriginalURL)
		var ttl time.Duration
		if record.IsEphemeral && record.ExpiresAt != nil {
			remaining := time.Until(*record.ExpiresAt)
			if remaining > 0 {
				ttl = remaining
			} else {
				return "", 0, false
			}
		} else {
			ttl = 24 * time.Hour
		}
		_ = cache.Client.Set(ctx, shortCode, cacheVal, ttl)
	}

	return record.OriginalURL, record.ID, true
}

// GetRecentURLs returns recent active URLs.
func (s *URLService) GetRecentURLs(ctx context.Context, limit int) ([]database.URLRecord, error) {
	return database.GetRecentURLs(ctx, limit)
}

// GetUserURLs returns links owned by the given user.
func (s *URLService) GetUserURLs(ctx context.Context, userID string, limit, offset int) ([]database.URLRecord, error) {
	return database.GetUserURLs(ctx, userID, limit, offset)
}

// DeleteUserURL removes a user's link from DB and Redis.
func (s *URLService) DeleteUserURL(ctx context.Context, shortCode, userID string) (bool, error) {
	deleted, err := database.DeleteUserURL(ctx, shortCode, userID)
	if err != nil || !deleted {
		return deleted, err
	}

	if cache.Client != nil {
		_ = cache.Client.Delete(ctx, shortCode)
	}
	return true, nil
}

// StartCleanupWorker periodically deletes expired ephemeral links from PostgreSQL.
func StartCleanupWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := database.DeleteExpiredURLs(ctx)
			if err != nil {
				log.Printf("cleanup worker error: %v", err)
			} else if deleted > 0 {
				log.Printf("cleaned up %d expired ephemeral links", deleted)
			}

			_, _ = database.DeleteExpiredRefreshTokens(ctx)
		}
	}
}

// EncodeBase62 converts a numeric sequence ID into a Base62 string.
func EncodeBase62(num int64) string {
	if num == 0 {
		return "0"
	}

	result := ""
	for num > 0 {
		index := num % 62
		result = string(alphabet[index]) + result
		num /= 62
	}

	return result
}