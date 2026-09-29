package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"url-shortner/internal/auth"
	"url-shortner/internal/services"
)

const recentURLsLimit = 10

type ShortenURLRequest struct {
	URL        string `json:"url"`
	CustomCode string `json:"customCode,omitempty"`
}

type ShortenURLResponse struct {
	Success     bool       `json:"success"`
	ShortCode   string     `json:"shortCode"`
	ShortURL    string     `json:"shortUrl"`
	OriginalURL string     `json:"originalUrl"`
	IsCustom    bool       `json:"isCustom"`
	IsEphemeral bool       `json:"isEphemeral"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

type RecentURLItem struct {
	ID          int64      `json:"id"`
	OriginalURL string     `json:"originalUrl"`
	ShortCode   string     `json:"shortCode"`
	ShortURL    string     `json:"shortUrl"`
	IsCustom    bool       `json:"isCustom"`
	IsEphemeral bool       `json:"isEphemeral"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	ClicksCount int64      `json:"clicksCount"`
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
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
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

	var userIDPtr *string
	if uid, ok := auth.GetUserID(r.Context()); ok && uid != "" {
		userIDPtr = &uid
	}

	record, err := services.URLSvc.ShortenURL(r.Context(), originalURL, urlBody.CustomCode, userIDPtr)
	if err != nil {
		if strings.Contains(err.Error(), "already in use") {
			writeError(w, http.StatusConflict, "This custom code is already taken. Please pick another one")
			return
		}
		if strings.Contains(err.Error(), "require an account") || strings.Contains(err.Error(), "must be") || strings.Contains(err.Error(), "reserved") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to shorten URL: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, ShortenURLResponse{
		Success:     true,
		ShortCode:   record.ShortCode,
		ShortURL:    buildShortURL(r, record.ShortCode),
		OriginalURL: originalURL,
		IsCustom:    record.IsCustom,
		IsEphemeral: record.IsEphemeral,
		ExpiresAt:   record.ExpiresAt,
	})
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	target, urlID, ok := services.URLSvc.ResolveURL(r.Context(), code)
	if !ok || target == "" {
		writeError(w, http.StatusNotFound, "Short link not found or has expired")
		return
	}

	// Asynchronously record click analytics
	if urlID > 0 {
		services.RecordClick(r, urlID, code)
	}

	http.Redirect(w, r, target, http.StatusFound)
}

func RecentURLsHandler(w http.ResponseWriter, r *http.Request) {
	records, err := services.URLSvc.GetRecentURLs(r.Context(), recentURLsLimit)
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
			IsCustom:    rec.IsCustom,
			IsEphemeral: rec.IsEphemeral,
			ExpiresAt:   rec.ExpiresAt,
			CreatedAt:   rec.CreatedAt,
			ClicksCount: rec.ClicksCount,
		})
	}

	writeJSON(w, http.StatusOK, RecentURLsResponse{
		Success: true,
		Data:    items,
	})
}

func UserURLsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	limit := 50
	offset := 0
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if val, err := strconv.Atoi(lStr); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}
	if oStr := r.URL.Query().Get("offset"); oStr != "" {
		if val, err := strconv.Atoi(oStr); err == nil && val >= 0 {
			offset = val
		}
	}

	records, err := services.URLSvc.GetUserURLs(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load user URLs")
		return
	}

	items := make([]RecentURLItem, 0, len(records))
	for _, rec := range records {
		items = append(items, RecentURLItem{
			ID:          rec.ID,
			OriginalURL: rec.OriginalURL,
			ShortCode:   rec.ShortCode,
			ShortURL:    buildShortURL(r, rec.ShortCode),
			IsCustom:    rec.IsCustom,
			IsEphemeral: rec.IsEphemeral,
			ExpiresAt:   rec.ExpiresAt,
			CreatedAt:   rec.CreatedAt,
			ClicksCount: rec.ClicksCount,
		})
	}

	writeJSON(w, http.StatusOK, RecentURLsResponse{
		Success: true,
		Data:    items,
	})
}

func DeleteUserURLHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r.Context())
	if !ok || userID == "" {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	code := r.PathValue("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing short code")
		return
	}

	deleted, err := services.URLSvc.DeleteUserURL(r.Context(), code, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete URL")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "URL not found or not owned by you")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}