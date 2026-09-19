package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"app/internal/domain"
	"app/internal/httpx"
)

// LoginRateLimiter membatasi percobaan login gagal per IP.
type LoginRateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	attempts map[string]*attemptWindow
}

type attemptWindow struct {
	count   int
	resetAt time.Time
}

func NewLoginRateLimiter(limitPerMinute int) *LoginRateLimiter {
	if limitPerMinute <= 0 {
		limitPerMinute = 6
	}
	return &LoginRateLimiter{
		limit:    limitPerMinute,
		window:   time.Minute,
		attempts: make(map[string]*attemptWindow),
	}
}

func (l *LoginRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if l.blocked(ip) {
			httpx.MapDomainError(c, domain.ErrTerlaluBanyakPercobaan)
			return
		}
		c.Next()
		if c.Writer.Status() == 401 {
			l.recordFailure(ip)
		}
		if c.Writer.Status() == 200 {
			l.reset(ip)
		}
	}
}

func (l *LoginRateLimiter) blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	w, ok := l.attempts[ip]
	if !ok {
		return false
	}
	if time.Now().After(w.resetAt) {
		delete(l.attempts, ip)
		return false
	}
	return w.count >= l.limit
}

func (l *LoginRateLimiter) recordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	w, ok := l.attempts[ip]
	if !ok || now.After(w.resetAt) {
		l.attempts[ip] = &attemptWindow{count: 1, resetAt: now.Add(l.window)}
		return
	}
	w.count++
}

func (l *LoginRateLimiter) reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, ip)
}
