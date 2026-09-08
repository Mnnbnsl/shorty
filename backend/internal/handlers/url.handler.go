package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"url-shortner/internal/services"
)

type ShortenURLRequest struct {
	URL string `json:"url"`
} 

// type GetURLRequest struct {
// 	ShortCode string `json:"shortcode"`
// }

func ShortenURLHandler(w http.ResponseWriter, r *http.Request) {
	var urlBody ShortenURLRequest

	err := json.NewDecoder(r.Body).Decode(&urlBody)
	if err != nil {
		http.Error(w, "Invalid JSON Payload", http.StatusBadRequest)
		return
	}

	fmt.Println("URL:", urlBody.URL)

	id := int64(125)
	shortenCode := services.EncodeBase62(id)

	services.StoreURL(shortenCode, urlBody.URL)
	
	fmt.Fprint(w, shortenCode)
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {

    code := r.PathValue("code")

    url, exists := services.GetURL(code)

    if !exists {
        http.Error(w, "URL not found", http.StatusNotFound)
        return
    }

    http.Redirect(w, r, url, http.StatusFound)
}