package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestMiddleware(logger *slog.Logger, origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		started := time.Now()
		id := uuid.NewString()
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		defer func() {
			logger.Info("http_request", "request_id", id, "method", c.Request.Method, "route", c.FullPath(), "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
		}()
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Writer.Header().Add("Vary", "Origin")
			if !allowed[origin] {
				Fail(c, NewError(403, "ORIGIN_NOT_ALLOWED", "Nguồn yêu cầu không được phép."))
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			c.Header("Access-Control-Max-Age", "600")
			c.AbortWithStatus(204)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				Fail(c, NewError(500, "INTERNAL_ERROR", "Không thể xử lý yêu cầu lúc này."))
			}
		}()
		c.Next()
	}
}

type rateEntry struct {
	started time.Time
	count   int
}
type Limiter struct {
	mu          sync.Mutex
	entries     map[string]rateEntry
	max         int
	lastCleanup time.Time
}

func NewLimiter(max int) *Limiter { return &Limiter{entries: map[string]rateEntry{}, max: max} }
func (l *Limiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now()
		key := c.ClientIP()
		l.mu.Lock()
		if now.Sub(l.lastCleanup) >= time.Minute {
			for k, e := range l.entries {
				if now.Sub(e.started) >= time.Minute {
					delete(l.entries, k)
				}
			}
			l.lastCleanup = now
		}
		e, found := l.entries[key]
		if !found && len(l.entries) >= 10000 {
			l.mu.Unlock()
			Fail(c, NewError(503, "BUSY", "Vui lòng thử lại sau."))
			return
		}
		if now.Sub(e.started) >= time.Minute {
			e = rateEntry{started: now}
		}
		e.count++
		l.entries[key] = e
		l.mu.Unlock()
		if e.count > l.max {
			c.Header("Retry-After", strconv.Itoa(max(1, int(time.Until(e.started.Add(time.Minute)).Seconds()))))
			Fail(c, NewError(429, "RATE_LIMITED", "Quá nhiều yêu cầu. Vui lòng thử lại sau."))
			return
		}
		c.Next()
	}
}
