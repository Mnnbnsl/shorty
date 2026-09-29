package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"url-shortner/internal/auth"
	"url-shortner/internal/cache"
	"url-shortner/internal/config"
	"url-shortner/internal/database"
	"url-shortner/internal/handlers"
	"url-shortner/internal/middleware"
	"url-shortner/internal/services"
)

var appConfig *config.Config

func frontendDir() string {
	if appConfig != nil && appConfig.FrontendDir != "" {
		return appConfig.FrontendDir
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
	appConfig = config.Load()

	// 1. Initialize PostgreSQL
	log.Printf("Connecting to PostgreSQL at %s...", appConfig.DatabaseURL)
	if err := database.InitDB(appConfig.DatabaseURL); err != nil {
		log.Fatalf("init database: %v", err)
	}
	defer database.Close()
	log.Println("PostgreSQL connection pool established and migrations applied.")

	// 2. Initialize Redis
	log.Printf("Connecting to Redis at %s...", appConfig.RedisURL)
	if err := cache.Init(appConfig.RedisURL); err != nil {
		log.Fatalf("init redis: %v", err)
	}
	defer func() {
		if cache.Client != nil {
			_ = cache.Client.Close()
		}
	}()
	log.Println("Redis client connected.")

	// 3. Initialize Services
	services.InitUserService(appConfig)
	services.InitURLService(appConfig.EphemeralTTL)
	services.InitAnalytics(appConfig.ClickBatchSize, appConfig.ClickFlushInterval)
	defer services.StopAnalytics()

	// 4. Start background ephemeral cleanup worker
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	go services.StartCleanupWorker(cleanupCtx, 15*time.Minute)

	// Auth middlewares
	authRequired := auth.RequiredMiddleware(appConfig.JWTSecret)
	optionalAuth := auth.OptionalMiddleware(appConfig.JWTSecret)

	// 5. Router
	mux := http.NewServeMux()

	// Static assets
	mux.HandleFunc("GET /{$}", indexHandler)
	mux.HandleFunc("GET /assets/", assetsHandler)

	// Auth routes (public)
	mux.HandleFunc("POST /api/auth/register", handlers.RegisterHandler)
	mux.HandleFunc("POST /api/auth/login", handlers.LoginHandler)
	mux.HandleFunc("POST /api/auth/refresh", handlers.RefreshTokenHandler)
	mux.HandleFunc("POST /api/auth/logout", handlers.LogoutHandler)

	// User dashboard & management (auth required)
	mux.Handle("GET /api/user/me", authRequired(http.HandlerFunc(handlers.GetMeHandler)))
	mux.Handle("GET /api/user/urls", authRequired(http.HandlerFunc(handlers.UserURLsHandler)))
	mux.Handle("DELETE /api/user/urls/{code}", authRequired(http.HandlerFunc(handlers.DeleteUserURLHandler)))

	// Analytics routes
	mux.HandleFunc("GET /api/urls/{code}/stats", handlers.URLStatsHandler)
	mux.HandleFunc("GET /api/urls/{code}/clicks", handlers.URLClicksHandler)

	// URL shortener (rate-limited + optional auth)
	mux.Handle("POST /url/shorten",
		middleware.RateLimitMiddleware(optionalAuth(http.HandlerFunc(handlers.ShortenURLHandler))))
	mux.HandleFunc("GET /api/urls/recent", handlers.RecentURLsHandler)

	// Redirect handler (wildcard matching /{code})
	mux.HandleFunc("GET /{code}", handlers.RedirectHandler)

	serverAddr := ":" + appConfig.Port
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      middleware.CORSMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown handling
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("🚀 Shorty v2 running on http://localhost:%s\n", appConfig.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced shutdown: %v", err)
	}

	log.Println("Server exiting.")
}