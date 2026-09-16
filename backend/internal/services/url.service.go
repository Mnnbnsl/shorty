package services

import (
	"fmt"
	"url-shortner/internal/database"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// ShortenURL inserts the original URL, encodes the generated primary key into
// a Base62 short code, then persists the code back onto the record.
func ShortenURL(originalURL string) (*database.URLRecord, error) {
	id, err := database.InsertURL(originalURL)
	if err != nil {
		return nil, fmt.Errorf("insert url: %w", err)
	}

	shortCode := EncodeBase62(id)
	if err := database.UpdateShortCode(id, shortCode); err != nil {
		return nil, fmt.Errorf("update short code: %w", err)
	}

	return &database.URLRecord{
		ID:          id,
		OriginalURL: originalURL,
		ShortCode:   shortCode,
	}, nil
}

// GetURL resolves a short code to the target URL from SQLite.
func GetURL(shortCode string) (string, bool) {
	originalURL, err := database.GetOriginalURL(shortCode)
	if err != nil {
		return "", false
	}

	return originalURL, true
}

// GetRecentURLs returns the most recently shortened URLs.
func GetRecentURLs(limit int) ([]database.URLRecord, error) {
	return database.GetRecentURLs(limit)
}

// EncodeBase62 converts a non-negative integer into a Base62 string.
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