package storage

import (
	"context"
	"time"
)

// StorageStrategy defines the interface for rate limiter persistence.
// Implementations can use Redis, in-memory, or any other backend.
type StorageStrategy interface {
	// Allow checks if a request identified by `key` is allowed given
	// `maxRequests` per `window` (typically 1 second).
	// Returns true if allowed, false if rate limit exceeded.
	// Also returns the remaining block time if blocked.
	Allow(ctx context.Context, key string, maxRequests int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)

	// Block sets a block on the given key for the specified duration.
	Block(ctx context.Context, key string, duration time.Duration) error

	// IsBlocked checks if the given key is currently blocked.
	// Returns true and the remaining block duration, or false if not blocked.
	IsBlocked(ctx context.Context, key string) (blocked bool, remaining time.Duration, err error)

	// Close cleans up any resources.
	Close() error
}
