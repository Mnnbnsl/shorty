package cache

import (
	"context"
	"time"
)

// Get retrieves a URL from the Redis cache.
func Get(shortCode string) (string, bool) {
	if Client == nil {
		return "", false
	}
	return Client.Get(context.Background(), shortCode)
}

// Set stores a URL in the Redis cache with an optional TTL.
func Set(shortCode, url string, ttl time.Duration) {
	if Client != nil {
		_ = Client.Set(context.Background(), shortCode, url, ttl)
	}
}

// Delete removes a URL from the Redis cache.
func Delete(shortCode string) {
	if Client != nil {
		_ = Client.Delete(context.Background(), shortCode)
	}
}
