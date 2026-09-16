package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"url-shortner/internal/database"
	"url-shortner/internal/handlers"
	"url-shortner/internal/middleware"
)

// frontendDir resolves the frontend directory, preferring an explicit
// FRONTEND_DIR override and falling back to ../frontend (run from backend/).
func frontendDir() string {
	if dir := os.Getenv("FRONTEND_DIR"); dir != "" {
		return dir
	}

	return filepath.Join("..", "frontend")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	indexPath := filepath.Join(frontendDir(), "index.html")

	if _, err := os.Stat(indexPath); err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, indexPath)
}

func assetsHandler(w http.ResponseWriter, r *http.Request) {
	assetsRoot := filepath.Join(frontendDir(), "assets")

	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/assets/"))
	if clean == "." || strings.HasPrefix(clean, "..") {
		http.NotFound(w, r)
		return
	}

	fullPath := filepath.Join(assetsRoot, clean)
	if !strings.HasPrefix(fullPath, assetsRoot) {
		http.NotFound(w, r)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, fullPath)
}

func main() {
	if err := database.InitDB(); err != nil {
		log.Fatalf("init database: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", indexHandler)
	mux.HandleFunc("GET /assets/", assetsHandler)
	mux.HandleFunc("POST /url/shorten", handlers.ShortenURLHandler)
	mux.HandleFunc("GET /api/urls/recent", handlers.RecentURLsHandler)
	mux.HandleFunc("GET /{code}", handlers.RedirectHandler)

	fmt.Println("Server running on http://localhost:8080")

	if err := http.ListenAndServe(":8080", middleware.CORSMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}