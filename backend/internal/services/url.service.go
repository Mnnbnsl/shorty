package services

import (
	"fmt"
	"url-shortner/internal/cache"
	"url-shortner/internal/database"
)

const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

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

func GetURL(shortCode string) (string, bool) {
	if url, ok := cache.Get(shortCode); ok {
		return url, true
	}

	originalURL, err := database.GetOriginalURL(shortCode)
	if err != nil {
		return "", false
	}

	cache.Set(shortCode, originalURL)
	return originalURL, true
}

func GetRecentURLs(limit int) ([]database.URLRecord, error) {
	return database.GetRecentURLs(limit)
}

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