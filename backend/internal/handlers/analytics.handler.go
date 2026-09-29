package handlers

import (
	"net/http"
	"strconv"

	"url-shortner/internal/services"
)

func URLStatsHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing short code")
		return
	}

	stats, err := services.GetURLStats(r.Context(), code)
	if err != nil {
		writeError(w, http.StatusNotFound, "Short link not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

func URLClicksHandler(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "Missing short code")
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

	clicks, err := services.GetURLClicks(r.Context(), code, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve click records")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    clicks,
	})
}
