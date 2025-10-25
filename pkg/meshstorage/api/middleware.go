package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// RateLimiter tracks request rates per IP
type RateLimiter struct {
	requests map[string]*RequestCounter
	limit    int
	window   time.Duration
	mu       sync.RWMutex
}

// RequestCounter tracks requests for an IP
type RequestCounter struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	limiter := &RateLimiter{
		requests: make(map[string]*RequestCounter),
		limit:    requestsPerMinute,
		window:   time.Minute,
	}

	// Cleanup goroutine
	go limiter.cleanup()

	return limiter
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	counter, exists := rl.requests[ip]
	if !exists {
		rl.requests[ip] = &RequestCounter{
			count:     1,
			resetTime: time.Now().Add(rl.window),
		}
		return true
	}

	if time.Now().After(counter.resetTime) {
		counter.count = 1
		counter.resetTime = time.Now().Add(rl.window)
		return true
	}

	if counter.count >= rl.limit {
		return false
	}

	counter.count++
	return true
}

// cleanup removes old entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, counter := range rl.requests {
			if now.After(counter.resetTime) {
				delete(rl.requests, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Global rate limiter instance
var globalRateLimiter *RateLimiter

// RateLimitMiddleware applies rate limiting
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	if globalRateLimiter == nil {
		globalRateLimiter = NewRateLimiter(requestsPerMinute)
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !globalRateLimiter.Allow(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": fmt.Sprintf("Maximum %d requests per minute", requestsPerMinute),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// LoggingMiddleware logs HTTP requests
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// Process request
		c.Next()

		// Log after request
		latency := time.Since(startTime)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path

		// Color-coded status
		var statusColor string
		switch {
		case statusCode >= 500:
			statusColor = "\033[31m" // Red
		case statusCode >= 400:
			statusColor = "\033[33m" // Yellow
		case statusCode >= 300:
			statusColor = "\033[36m" // Cyan
		case statusCode >= 200:
			statusColor = "\033[32m" // Green
		default:
			statusColor = "\033[0m" // Default
		}

		resetColor := "\033[0m"

		fmt.Printf("%s%d%s | %s | %s %s | %v\n",
			statusColor,
			statusCode,
			resetColor,
			c.ClientIP(),
			method,
			path,
			latency,
		)
	}
}

// AuthMiddleware validates API keys (optional)
func AuthMiddleware(validAPIKeys map[string]bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing API key",
			})
			c.Abort()
			return
		}

		if !validAPIKeys[apiKey] {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid API key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

// SuccessResponse is a standard success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}
