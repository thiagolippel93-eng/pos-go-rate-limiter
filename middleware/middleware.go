package middleware

import (
	"net"
	"net/http"
	"strings"

	"rate-limiter/limiter"
)

const blockedMessage = "you have reached the maximum number of requests or actions allowed within a certain time frame"

func RateLimiterMiddleware(rl *limiter.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			token := r.Header.Get("API_KEY")

			result, err := rl.Check(r.Context(), ip, token)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			if !result.Allowed {
				w.Header().Set("Retry-After", result.Remaining.String())
				http.Error(w, blockedMessage, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
