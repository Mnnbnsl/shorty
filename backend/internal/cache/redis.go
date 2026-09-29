package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	rdb *redis.Client
}

var Client *RedisClient

// Init connects to Redis and verifies connection via PING.
func Init(redisURL string) error {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return fmt.Errorf("parse redis url: %w", err)
	}

	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}

	Client = &RedisClient{rdb: rdb}
	return nil
}

// Get returns the cached destination URL for the given short code.
func (c *RedisClient) Get(ctx context.Context, shortCode string) (string, bool) {
	if c == nil || c.rdb == nil {
		return "", false
	}

	key := fmt.Sprintf("url:%s", shortCode)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

// Set stores a short code mapping with an optional expiration duration.
// If ttl <= 0, the key will not expire in Redis (persistent).
func (c *RedisClient) Set(ctx context.Context, shortCode, originalURL string, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}

	key := fmt.Sprintf("url:%s", shortCode)
	return c.rdb.Set(ctx, key, originalURL, ttl).Err()
}

// Delete removes a short code from the cache.
func (c *RedisClient) Delete(ctx context.Context, shortCode string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	key := fmt.Sprintf("url:%s", shortCode)
	return c.rdb.Del(ctx, key).Err()
}

// IncrClick increments the real-time click counter in Redis.
func (c *RedisClient) IncrClick(ctx context.Context, shortCode string) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	key := fmt.Sprintf("clicks:%s", shortCode)
	return c.rdb.Incr(ctx, key).Result()
}

// GetClicks returns the cached real-time click count for a short code.
func (c *RedisClient) GetClicks(ctx context.Context, shortCode string) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, nil
	}
	key := fmt.Sprintf("clicks:%s", shortCode)
	val, err := c.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// Close closes the Redis connection.
func (c *RedisClient) Close() error {
	if c != nil && c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}
