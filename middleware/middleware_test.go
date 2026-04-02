package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rate-limiter/config"
	"rate-limiter/limiter"
)

type mockStorage struct {
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

func newTestLimiter(ipRPS int, tokenRPS int) (*limiter.RateLimiter, *mockStorage) {
	store := newMockStorage()
	cfg := &config.Config{
		IPRPS:            ipRPS,
		IPBlockTime:      30 * time.Second,
		TokenRPS:         map[string]int{"": tokenRPS},
		TokenBlockTime:   map[string]time.Duration{"": 30 * time.Second},
		DefaultBlockTime: 30 * time.Second,
	}
	return limiter.NewRateLimiter(store, cfg), store
}

func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

func TestMiddlewareAllowsRequestsUnderLimit(t *testing.T) {
	rl, _ := newTestLimiter(5, 100)
	defer rl.Close()

	handler := RateLimiterMiddleware(rl)(dummyHandler())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestMiddlewareBlocksAfterLimit(t *testing.T) {
	rl, _ := newTestLimiter(3, 100)
	defer rl.Close()

	handler := RateLimiterMiddleware(rl)(dummyHandler())

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}

	if rec.Body.String() != "you have reached the maximum number of requests or actions allowed within a certain time frame\n" {
		t.Errorf("unexpected body: %q", rec.Body.String())
	}
}

func TestTokenTakesPrecedenceOverIP(t *testing.T) {
	rl, _ := newTestLimiter(2, 10)
	defer rl.Close()

	handler := RateLimiterMiddleware(rl)(dummyHandler())

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "172.16.0.1:1234"
		req.Header.Set("API_KEY", "mytoken")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d with token: expected 200, got %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "172.16.0.1:1234"
	req.Header.Set("API_KEY", "mytoken")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}

func TestIPLimitStillAppliesWithoutToken(t *testing.T) {
	rl, _ := newTestLimiter(1, 100)
	defer rl.Close()

	handler := RateLimiterMiddleware(rl)(dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.100.1:5678"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.100.1:5678"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", rec.Code)
	}
}
