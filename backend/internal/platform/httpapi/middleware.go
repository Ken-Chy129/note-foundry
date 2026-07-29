package httpapi

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func SecurityHeaders(next http.Handler, secureTransport bool) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		response.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("X-Frame-Options", "DENY")
		if secureTransport {
			response.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(response, request)
	})
}

type FixedWindowConfig struct {
	Limit             int
	Window            time.Duration
	Now               func() time.Time
	TrustForwardedFor bool
}

type FixedWindowLimiter struct {
	limit             int
	window            time.Duration
	now               func() time.Time
	mutex             sync.Mutex
	clients           map[string]fixedWindowClient
	lastCleanup       time.Time
	trustForwardedFor bool
}

type fixedWindowClient struct {
	count   int
	resetAt time.Time
}

func NewFixedWindowLimiter(config FixedWindowConfig) *FixedWindowLimiter {
	limit := config.Limit
	if limit <= 0 {
		limit = 10
	}
	window := config.Window
	if window <= 0 {
		window = 15 * time.Minute
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &FixedWindowLimiter{
		limit:             limit,
		window:            window,
		now:               now,
		clients:           make(map[string]fixedWindowClient),
		trustForwardedFor: config.TrustForwardedFor,
	}
}

func (limiter *FixedWindowLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		allowed, retryAfter := limiter.allow(clientAddress(request, limiter.trustForwardedFor))
		if !allowed {
			response.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
			WriteError(response, http.StatusTooManyRequests, "RATE_LIMITED", "too many authentication requests")
			return
		}
		next.ServeHTTP(response, request)
	})
}

func (limiter *FixedWindowLimiter) allow(client string) (bool, time.Duration) {
	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := limiter.now()
	if limiter.lastCleanup.IsZero() || now.Sub(limiter.lastCleanup) >= limiter.window {
		for key, entry := range limiter.clients {
			if !now.Before(entry.resetAt) {
				delete(limiter.clients, key)
			}
		}
		limiter.lastCleanup = now
	}

	entry, exists := limiter.clients[client]
	if !exists || !now.Before(entry.resetAt) {
		limiter.clients[client] = fixedWindowClient{count: 1, resetAt: now.Add(limiter.window)}
		return true, 0
	}
	if entry.count >= limiter.limit {
		return false, entry.resetAt.Sub(now)
	}
	entry.count++
	limiter.clients[client] = entry
	return true, 0
}

func clientAddress(request *http.Request, trustForwardedFor bool) string {
	if forwarded := request.Header.Get("X-Forwarded-For"); trustForwardedFor && forwarded != "" {
		address := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if address != "" {
			return address
		}
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}
