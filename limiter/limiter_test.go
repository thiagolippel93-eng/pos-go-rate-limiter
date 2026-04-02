package limiter

import (
	"context"
	"testing"
	"time"

	"rate-limiter/config"
	"rate-limiter/storage"
)

type mockStorage struct {
	storage.StorageStrategy
	counts map[string]int
	blocks map[string]time.Time
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		counts: make(map[string]int),
		blocks: make(map[string]time.Time),
	}
}

func (m *mockStorage) Allow(_ context.Context, key string, maxRequests int, _ time.Duration) (bool, time.Duration, error) {
	if deadline, ok := m.blocks[key]; ok && time.Now().Before(deadline) {
		return false, time.Until(deadline), nil
	}

	if _, ok := m.blocks[key]; ok {
		delete(m.blocks, key)
		m.counts[key] = 0
	}

	m.counts[key]++
	if m.counts[key] > maxRequests {
		return false, 0, nil
	}
	return true, 0, nil
}

func (m *mockStorage) Block(_ context.Context, key string, duration time.Duration) error {
	m.blocks[key] = time.Now().Add(duration)
	return nil
}

func (m *mockStorage) IsBlocked(_ context.Context, key string) (bool, time.Duration, error) {
	if deadline, ok := m.blocks[key]; ok && time.Now().Before(deadline) {
		return true, time.Until(deadline), nil
	}
	return false, 0, nil
}

func (m *mockStorage) Close() error { return nil }

func TestTokenPrecedenceOverIP(t *testing.T) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            2,
		IPBlockTime:      30 * time.Second,
		TokenRPS:         map[string]int{"mytoken": 100},
		TokenBlockTime:   map[string]time.Duration{"mytoken": 30 * time.Second},
		DefaultBlockTime: 30 * time.Second,
	}
	rl := NewRateLimiter(store, cfg)
	defer rl.Close()

	ctx := context.Background()
	ip := "192.168.1.1"

	for i := 0; i < 100; i++ {
		result, err := rl.Check(ctx, ip, "mytoken")
		if err != nil {
			t.Fatalf("token check error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed by token limit (100)", i+1)
		}
	}

	result, _ := rl.Check(ctx, ip, "mytoken")
	if result.Allowed {
		t.Error("101st request should be blocked by token limit")
	}
}

func TestIPFallbackWithoutToken(t *testing.T) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            3,
		IPBlockTime:      30 * time.Second,
		TokenRPS:         map[string]int{"": 100},
		TokenBlockTime:   map[string]time.Duration{"": 30 * time.Second},
		DefaultBlockTime: 30 * time.Second,
	}
	rl := NewRateLimiter(store, cfg)
	defer rl.Close()

	ctx := context.Background()
	ip := "10.0.0.1"

	for i := 0; i < 3; i++ {
		result, err := rl.Check(ctx, ip, "")
		if err != nil {
			t.Fatalf("ip check error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed by IP limit (3)", i+1)
		}
	}

	result, _ := rl.Check(ctx, ip, "")
	if result.Allowed {
		t.Error("4th request should be blocked by IP limit")
	}
}

func TestTokenWithIPBlockStillUsesToken(t *testing.T) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            1,
		IPBlockTime:      30 * time.Second,
		TokenRPS:         map[string]int{"mytoken": 100},
		TokenBlockTime:   map[string]time.Duration{"mytoken": 30 * time.Second},
		DefaultBlockTime: 30 * time.Second,
	}
	rl := NewRateLimiter(store, cfg)
	defer rl.Close()

	ctx := context.Background()
	ip := "192.168.1.1"

	for i := 0; i < 100; i++ {
		result, err := rl.Check(ctx, ip, "mytoken")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !result.Allowed {
			t.Errorf("request %d should be allowed by token", i+1)
		}
	}
}

func TestDifferentTokensIndependent(t *testing.T) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            2,
		IPBlockTime:      30 * time.Second,
		TokenRPS:         map[string]int{"token1": 5, "token2": 10},
		TokenBlockTime:   map[string]time.Duration{"token1": 30 * time.Second, "token2": 30 * time.Second},
		DefaultBlockTime: 30 * time.Second,
	}
	rl := NewRateLimiter(store, cfg)
	defer rl.Close()

	ctx := context.Background()
	ip := "10.0.0.1"

	for i := 0; i < 5; i++ {
		result, _ := rl.Check(ctx, ip, "token1")
		if !result.Allowed {
			t.Errorf("token1 request %d should be allowed", i+1)
		}
	}

	for i := 0; i < 10; i++ {
		result, _ := rl.Check(ctx, ip, "token2")
		if !result.Allowed {
			t.Errorf("token2 request %d should be allowed", i+1)
		}
	}

	result, _ := rl.Check(ctx, ip, "token1")
	if result.Allowed {
		t.Error("token1 should be blocked after 5 requests")
	}

	result, _ = rl.Check(ctx, ip, "token2")
	if result.Allowed {
		t.Error("token2 should be blocked after 10 requests")
	}
}

func TestIPBlockingReturnsCorrectRemaining(t *testing.T) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            1,
		IPBlockTime:      10 * time.Second,
		TokenRPS:         map[string]int{"": 10},
		TokenBlockTime:   map[string]time.Duration{"": 10 * time.Second},
		DefaultBlockTime: 10 * time.Second,
	}
	rl := NewRateLimiter(store, cfg)
	defer rl.Close()

	ctx := context.Background()
	ip := "192.168.1.1"

	result, _ := rl.Check(ctx, ip, "")
	if result.Allowed != true {
		t.Error("first request should be allowed")
	}

	result, _ = rl.Check(ctx, ip, "")
	if result.Allowed {
		t.Error("second request should be blocked")
	}
	if result.Remaining != 10*time.Second {
		t.Errorf("expected remaining 10s, got %v", result.Remaining)
	}
}

func TestTokenBlockingReturnsCorrectRemaining(t *testing.T) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            10,
		IPBlockTime:      10 * time.Second,
		TokenRPS:         map[string]int{"mytoken": 2},
		TokenBlockTime:   map[string]time.Duration{"mytoken": 5 * time.Second},
		DefaultBlockTime: 10 * time.Second,
	}
	rl := NewRateLimiter(store, cfg)
	defer rl.Close()

	ctx := context.Background()
	ip := "192.168.1.1"

	result, _ := rl.Check(ctx, ip, "mytoken")
	if !result.Allowed {
		t.Error("1st request should be allowed")
	}

	result, _ = rl.Check(ctx, ip, "mytoken")
	if !result.Allowed {
		t.Error("2nd request should be allowed")
	}

	result, _ = rl.Check(ctx, ip, "mytoken")
	if result.Allowed {
		t.Error("3rd request should be blocked")
	}
	if result.Remaining != 5*time.Second {
		t.Errorf("expected remaining 5s, got %v", result.Remaining)
	}
}

func TestBlockTimeRespected(t *testing.T) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            1,
		IPBlockTime:      100 * time.Millisecond,
		TokenRPS:         map[string]int{"": 10},
		TokenBlockTime:   map[string]time.Duration{"": 100 * time.Millisecond},
		DefaultBlockTime: 100 * time.Millisecond,
	}
	rl := NewRateLimiter(store, cfg)
	defer rl.Close()

	ctx := context.Background()
	ip := "10.0.0.1"

	rl.Check(ctx, ip, "")
	rl.Check(ctx, ip, "")

	result, _ := rl.Check(ctx, ip, "")
	if result.Allowed {
		t.Error("should be blocked")
	}

	time.Sleep(150 * time.Millisecond)

	result, _ = rl.Check(ctx, ip, "")
	if !result.Allowed {
		t.Error("should be unblocked after block time")
	}
}
