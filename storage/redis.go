package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(addr, password string, db int) (*RedisStorage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	return &RedisStorage{client: client}, nil
}

func (r *RedisStorage) Allow(ctx context.Context, key string, maxRequests int, window time.Duration) (bool, time.Duration, error) {
	blocked, remaining, err := r.IsBlocked(ctx, key)
	if err != nil {
		return false, 0, err
	}
	if blocked {
		return false, remaining, nil
	}

	countKey := fmt.Sprintf("rl:count:%s", key)
	blockKey := fmt.Sprintf("rl:block:%s", key)

	pipe := r.client.Pipeline()
	incrCmd := pipe.Incr(ctx, countKey)
	pipe.Expire(ctx, countKey, window)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("redis pipeline error: %w", err)
	}

	count := int(incrCmd.Val())
	if count > maxRequests {
		return false, 0, nil
	}
	_ = blockKey // block key is used in Block method
	return true, 0, nil
}

func (r *RedisStorage) Block(ctx context.Context, key string, duration time.Duration) error {
	blockKey := fmt.Sprintf("rl:block:%s", key)
	return r.client.Set(ctx, blockKey, "1", duration).Err()
}

func (r *RedisStorage) IsBlocked(ctx context.Context, key string) (bool, time.Duration, error) {
	blockKey := fmt.Sprintf("rl:block:%s", key)

	ttl, err := r.client.TTL(ctx, blockKey).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis TTL error: %w", err)
	}

	if ttl > 0 {
		return true, ttl, nil
	}

	return false, 0, nil
}

func (r *RedisStorage) Close() error {
	return r.client.Close()
}
