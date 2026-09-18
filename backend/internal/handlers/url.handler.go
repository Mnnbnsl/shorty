package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"url-shortner/internal/services"
)

const recentURLsLimit = 10

type ShortenURLRequest struct {
	URL string `json:"url"`
}

type ShortenURLResponse struct {
	Success     bool   `json:"success"`
	ShortCode   string `json:"shortCode"`
	ShortURL    string `json:"shortUrl"`
	OriginalURL string `json:"originalUrl"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type RecentURLItem struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"originalUrl"`
	ShortCode   string `json:"shortCode"`
	ShortURL    string `json:"shortUrl"`
	CreatedAt   string `json:"createdAt"`
}

type RecentURLsResponse struct {
	Success bool            `json:"success"`
	Data    []RecentURLItem `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Success: false,
		Error:   message,
	})
}

// buildShortURL reconstructs the public short URL from the current request.
func buildShortURL(r *http.Request, shortCode string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	return scheme + "://" + r.Host + "/" + shortCode
}

func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil
	}
	if parsed.Host == "" {
		return "", nil
	}

	return raw, nil
}

func ShortenURLHandler(w http.ResponseWriter, r *http.Request) {
	var urlBody ShortenURLRequest
	if err := json.NewDecoder(r.Body).Decode(&urlBody); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	originalURL, err := normalizeURL(urlBody.URL)
	if err != nil || originalURL == "" {
		writeError(w, http.StatusBadRequest, "Please enter a valid URL")
		return
	}

	record, err := services.ShortenURL(originalURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to shorten URL")
		return
	}

	writeJSON(w, http.StatusCreated, ShortenURLResponse{
		Success:     true,
		ShortCode:   record.ShortCode,
		ShortURL:    buildShortURL(r, record.ShortCode),
		OriginalURL: originalURL,
	})
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	target, ok := services.GetURL(code)
	if !ok || target == "" {
		writeError(w, http.StatusNotFound, "Short URL not found")
		return
	}

	http.Redirect(w, r, target, http.StatusFound)
}

func RecentURLsHandler(w http.ResponseWriter, r *http.Request) {
	records, err := services.GetRecentURLs(recentURLsLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load recent URLs")
		return
	}

	items := make([]RecentURLItem, 0, len(records))
	for _, rec := range records {
		items = append(items, RecentURLItem{
			ID:          rec.ID,
			OriginalURL: rec.OriginalURL,
			ShortCode:   rec.ShortCode,
			ShortURL:    buildShortURL(r, rec.ShortCode),
			CreatedAt:   rec.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, RecentURLsResponse{
		Success: true,
		Data:    items,
	})
}