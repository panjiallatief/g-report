package auth

import (
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"it-broadcast-ops/internal/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)


func RegisterRoutes(r *gin.Engine) {
	authGroup := r.Group("/auth")
	{
		authGroup.GET("/login", ShowLoginForm)
		authGroup.POST("/login", Login)
		authGroup.GET("/logout", Logout)
	}
}

func ShowLoginForm(c *gin.Context) {
	c.HTML(http.StatusOK, "pages/auth/login.html", gin.H{
		"title": "Login",
	})
}

func Login(c *gin.Context) {
	email := c.PostForm("email")
	password := c.PostForm("password")

	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.HTML(http.StatusUnauthorized, "pages/auth/login.html", gin.H{
			"title": "Login",
			"error": "Invalid email or password",
		})
		return
	}

	if !utils.CheckPasswordHash(password, user.PasswordHash) {
		c.HTML(http.StatusUnauthorized, "pages/auth/login.html", gin.H{
			"title": "Login",
			"error": "Invalid email or password",
		})
		return
	}

	// Set Cookie
	c.SetCookie("user_id", user.ID.String(), 3600*8, "/", "", false, true)
	c.SetCookie("user_role", string(user.Role), 3600*8, "/", "", false, true)

	// Redirect based on role
	switch user.Role {
	case models.RoleManager:
		c.Redirect(http.StatusFound, "/manager")
	case models.RoleStaff:
		c.Redirect(http.StatusFound, "/staff")
	default:
		c.Redirect(http.StatusFound, "/consumer")
	}
}

func Logout(c *gin.Context) {
	c.SetCookie("user_id", "", -1, "/", "", false, true)
	c.SetCookie("user_role", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/auth/login")
}
