package middlewares

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; object-src 'none'")
		c.Next()
	}
}

func StrictCORS(cfg config.Config) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = randomHex(16)
		}
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func RateLimit(limit int) gin.HandlerFunc {
	type bucket struct {
		count   int
		resetAt time.Time
	}

	var mu sync.Mutex
	buckets := map[string]bucket{}
	if limit <= 0 {
		limit = 600
	}
	window := time.Minute

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		current := buckets[ip]
		if now.After(current.resetAt) {
			current = bucket{count: 0, resetAt: now.Add(window)}
		}
		current.count++
		buckets[ip] = current
		mu.Unlock()

		if current.count > limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"code":    "rate_limited",
				"message": "Too many requests",
			})
			return
		}

		c.Next()
	}
}

func AdminNotFoundGuard(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	authService := services.NewAuthService(db, cfg)

	return func(c *gin.Context) {
		if db == nil || cfg.JWTSecret == "" {
			abortNotFound(c)
			return
		}

		cookie, err := c.Cookie("access_token")
		if err != nil || cookie == "" {
			abortNotFound(c)
			return
		}

		claims, err := authService.ParseAccessToken(cookie)
		if err != nil {
			abortNotFound(c)
			return
		}

		if err := authService.EnsureTokenAllowed(c.Request.Context(), claims.ID); err != nil {
			abortNotFound(c)
			return
		}

		var user models.User
		if err := db.WithContext(c.Request.Context()).
			Where("id = ? AND is_admin = ?", claims.UserID, true).
			First(&user).Error; err != nil {
			abortNotFound(c)
			return
		}
		if err := authService.EnsureTokenVersion(user, claims); err != nil {
			abortNotFound(c)
			return
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("is_admin", true)
		c.Next()
	}
}

func abortNotFound(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"success": false,
		"message": "Not found",
	})
}

func randomHex(size int) string {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes)
}
