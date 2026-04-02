package limiter

import (
	"context"
	"fmt"
	"time"

	"rate-limiter/config"
	"rate-limiter/storage"
)

type RateLimiter struct {
	storage storage.StorageStrategy
	config  *config.Config
}

type Result struct {
	Allowed   bool
	Remaining time.Duration
}

func NewRateLimiter(store storage.StorageStrategy, cfg *config.Config) *RateLimiter {
	return &RateLimiter{
		storage: store,
		config:  cfg,
	}
}

func (rl *RateLimiter) Check(ctx context.Context, ip string, token string) (*Result, error) {
	window := time.Second

	if token != "" {
		maxRPS, blockTime := rl.getTokenConfig(token)

		allowed, remaining, err := rl.storage.Allow(ctx, fmt.Sprintf("token:%s", token), maxRPS, window)
		if err != nil {
			return nil, fmt.Errorf("token check error: %w", err)
		}

		if !allowed {
			if remaining == 0 {
				if err := rl.storage.Block(ctx, fmt.Sprintf("token:%s", token), blockTime); err != nil {
					return nil, fmt.Errorf("token block error: %w", err)
				}
				remaining = blockTime
			}
			return &Result{Allowed: false, Remaining: remaining}, nil
		}

		return &Result{Allowed: true}, nil
	}

	// Fallback to IP-based limiting
	allowed, remaining, err := rl.storage.Allow(ctx, fmt.Sprintf("ip:%s", ip), rl.config.IPRPS, window)
	if err != nil {
		return nil, fmt.Errorf("ip check error: %w", err)
	}

	if !allowed {
		if remaining == 0 {
			if err := rl.storage.Block(ctx, fmt.Sprintf("ip:%s", ip), rl.config.IPBlockTime); err != nil {
				return nil, fmt.Errorf("ip block error: %w", err)
			}
			remaining = rl.config.IPBlockTime
		}
		return &Result{Allowed: false, Remaining: remaining}, nil
	}

	return &Result{Allowed: true}, nil
}

func (rl *RateLimiter) getTokenConfig(token string) (int, time.Duration) {
	if rps, ok := rl.config.TokenRPS[token]; ok {
		blockTime := rl.config.DefaultBlockTime
		if bt, ok := rl.config.TokenBlockTime[token]; ok {
			blockTime = bt
		}
		return rps, blockTime
	}

	return rl.config.TokenRPS[""], rl.config.TokenBlockTime[""]
}

func (rl *RateLimiter) Close() error {
	return rl.storage.Close()
}
