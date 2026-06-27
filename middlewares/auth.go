package middlewares

import (
	"net/http"

	"academyprometheus/backend/config"
	"academyprometheus/backend/models"
	"academyprometheus/backend/services"
	"academyprometheus/backend/structs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AuthGuard(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	authService := services.NewAuthService(db, cfg)

	return func(c *gin.Context) {
		if db == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}

		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}

		claims, err := authService.ParseAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}
		if err := authService.EnsureTokenAllowed(c.Request.Context(), claims.ID); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}

		var user models.User
		if err := db.WithContext(c.Request.Context()).First(&user, claims.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}
		if err := authService.EnsureTokenVersion(user, claims); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}

		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Next()
	}
}

// OptionalAuth attaches a valid current user while keeping public forms usable by guests.
func OptionalAuth(db *gorm.DB, cfg config.Config) gin.HandlerFunc {
	authService := services.NewAuthService(db, cfg)
	return func(c *gin.Context) {
		if db == nil {
			c.Next()
			return
		}
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			c.Next()
			return
		}
		claims, err := authService.ParseAccessToken(token)
		if err != nil || authService.EnsureTokenAllowed(c.Request.Context(), claims.ID) != nil {
			c.Next()
			return
		}
		var user models.User
		if err := db.WithContext(c.Request.Context()).First(&user, claims.UserID).Error; err == nil && authService.EnsureTokenVersion(user, claims) == nil {
			c.Set("user", user)
			c.Set("user_id", user.ID)
		}
		c.Next()
	}
}

func RoleGuard(roles ...string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, role := range roles {
		allowed[role] = true
	}

	return func(c *gin.Context) {
		value, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}

		user, ok := value.(models.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, structs.Response{Success: false, Message: "Unauthorized"})
			return
		}

		if allowed["admin"] && user.IsAdmin {
			c.Next()
			return
		}
		if allowed["student"] && user.IsStudent {
			c.Next()
			return
		}
		if allowed["instructor"] && user.IsInstructor {
			c.Next()
			return
		}
		if allowed["company"] && user.IsCompany {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, structs.Response{Success: false, Message: "Forbidden"})
	}
}
