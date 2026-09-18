package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// rateLimiter constants
const (
	// refillRate is the number of tokens added per second per IP.
	refillRate float64 = 5
	// burstCap is the maximum tokens an IP can accumulate (burst size).
	burstCap float64 = 10
)

type bucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

var buckets sync.Map


func allow(ip string) bool {
	val, _ := buckets.LoadOrStore(ip, &bucket{tokens: burstCap, last: time.Now()})
	b := val.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.last = now

	b.tokens += elapsed * refillRate
	if b.tokens > burstCap {
		b.tokens = burstCap
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		if !allow(ip) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"success":false,"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
