package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter is a simple per-IP token-bucket limiter used to restrict
// brute-force attempts on authentication endpoints.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
	rate    float64 // token per detik
	burst   float64
}

type rateBucket struct {
	tokens   float64
	lastSeen time.Time
}

func NewRateLimiter(perMinute, burst int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*rateBucket),
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
	}
}

func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			writeJSONError(w, http.StatusTooManyRequests, "too many attempts, please try again shortly")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *RateLimiter) allow(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Discard old buckets periodically to keep the map bounded.
	if len(l.buckets) > 10_000 {
		for k, b := range l.buckets {
			if now.Sub(b.lastSeen) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
	}

	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &rateBucket{tokens: l.burst}
		l.buckets[key] = bucket
	}

	bucket.tokens += now.Sub(bucket.lastSeen).Seconds() * l.rate
	if bucket.tokens > l.burst {
		bucket.tokens = l.burst
	}
	bucket.lastSeen = now

	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
