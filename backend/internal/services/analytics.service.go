package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"url-shortner/internal/cache"
	"url-shortner/internal/database"
)

var (
	clickQueue   chan database.ClickRecord
	queueOnce    sync.Once
	workerCancel context.CancelFunc
)

// InitAnalytics initializes the click worker channel and starts background consumer.
func InitAnalytics(batchSize int, flushInterval time.Duration) {
	queueOnce.Do(func() {
		clickQueue = make(chan database.ClickRecord, 10_000)

		ctx, cancel := context.WithCancel(context.Background())
		workerCancel = cancel

		go runClickWorker(ctx, batchSize, flushInterval)
	})
}

// StopAnalytics cleanly flushes and shuts down the analytics worker.
func StopAnalytics() {
	if workerCancel != nil {
		workerCancel()
	}
}

// RecordClick enqueues a click event without blocking HTTP requests.
func RecordClick(r *http.Request, urlID int64, shortCode string) {
	if clickQueue == nil {
		return
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	// Anonymize IP via SHA-256 for privacy
	hash := sha256.Sum256([]byte(ip))
	ipHash := hex.EncodeToString(hash[:8]) // first 16 chars is plenty for unique count

	ua := r.UserAgent()
	referer := r.Referer()
	deviceType := detectDeviceType(ua)

	// Clean referer to domain only
	cleanReferer := extractRefererDomain(referer)

	event := database.ClickRecord{
		URLID:      urlID,
		ClickedAt:  time.Now(),
		IPHash:     ipHash,
		UserAgent:  ua,
		Referer:    cleanReferer,
		DeviceType: deviceType,
	}

	// Update real-time Redis counter
	go func() {
		if cache.Client != nil {
			_, _ = cache.Client.IncrClick(context.Background(), shortCode)
		}
	}()

	select {
	case clickQueue <- event:
	default:
		log.Printf("analytics queue full, dropping click event for %s", shortCode)
	}
}

func runClickWorker(ctx context.Context, batchSize int, flushInterval time.Duration) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]database.ClickRecord, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		toInsert := make([]database.ClickRecord, len(batch))
		copy(toInsert, batch)
		batch = batch[:0]

		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := database.BatchInsertClicks(flushCtx, toInsert); err != nil {
			log.Printf("error batch inserting clicks: %v", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return

		case event := <-clickQueue:
			batch = append(batch, event)
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

func detectDeviceType(ua string) string {
	lower := strings.ToLower(ua)
	if strings.Contains(lower, "bot") || strings.Contains(lower, "crawler") ||
		strings.Contains(lower, "spider") || strings.Contains(lower, "slurp") {
		return "bot"
	}
	if strings.Contains(lower, "ipad") || strings.Contains(lower, "tablet") ||
		strings.Contains(lower, "playbook") || strings.Contains(lower, "silk") {
		return "tablet"
	}
	if strings.Contains(lower, "mobile") || strings.Contains(lower, "android") ||
		strings.Contains(lower, "iphone") || strings.Contains(lower, "ipod") {
		return "mobile"
	}
	return "desktop"
}

func extractRefererDomain(ref string) string {
	if ref == "" {
		return "direct"
	}
	ref = strings.TrimPrefix(ref, "https://")
	ref = strings.TrimPrefix(ref, "http://")
	parts := strings.Split(ref, "/")
	if len(parts) > 0 && parts[0] != "" {
		domain := strings.ToLower(parts[0])
		domain = strings.TrimPrefix(domain, "www.")
		return domain
	}
	return "direct"
}

// GetURLStats gets aggregated statistics for a short code.
func GetURLStats(ctx context.Context, shortCode string) (*database.URLStats, error) {
	return database.GetURLStats(ctx, shortCode)
}

// GetURLClicks gets paginated raw clicks for a short code.
func GetURLClicks(ctx context.Context, shortCode string, limit, offset int) ([]database.ClickRecord, error) {
	return database.GetURLClicks(ctx, shortCode, limit, offset)
}
