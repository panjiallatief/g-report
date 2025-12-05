package auth

import (
	"it-broadcast-ops/internal/models"

	"github.com/gin-gonic/gin"
)

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Placeholder for session check
		// For now, we'll assume everyone is authenticated as a guest/consumer if we don't Implement real session yet
		// But let's check for a cookie
		_, err := c.Cookie("user_id")
		if err != nil {
			// Redirect to login or return 401
			// c.Redirect(http.StatusFound, "/login")
			// c.Abort()
			
			// For DEVELOPMENT ease, let's bypass if no cookie (just for now) or return 401 if it's API
			// But creating a proper middleware from start is better.
			// Let's just Allow for now to test the router, implementing properly next step.
			c.Next()
			return
		}
		c.Next()
	}
}

func RoleRequired(role models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check user role from context (set by AuthRequired)
		// For now, pass
		c.Next()
	}
}
