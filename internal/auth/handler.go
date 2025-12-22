package auth

import (
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"it-broadcast-ops/internal/utils"
	"log"
	"net/http"
	"strings"

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

// ShowLoginForm godoc
// @Summary      Show login form
// @Description  Display the login page
// @Tags         Auth
// @Produce      html
// @Success      200  {string}  string  "HTML page"
// @Router       /auth/login [get]
func ShowLoginForm(c *gin.Context) {
	c.HTML(http.StatusOK, "pages/auth/login.html", gin.H{
		"title": "Login",
	})
}

// Login godoc
// @Summary      User login
// @Description  Authenticate user with email/username and password
// @Tags         Auth
// @Accept       x-www-form-urlencoded
// @Produce      html
// @Param        email     formData  string  true  "Email or Username"
// @Param        password  formData  string  true  "Password"
// @Success      302  {string}  string  "Redirect to dashboard"
// @Failure      401  {string}  string  "Invalid credentials"
// @Router       /auth/login [post]
func Login(c *gin.Context) {
	identifier := c.PostForm("email") // Can be email or username
	password := c.PostForm("password")

	if identifier == "" || password == "" {
		c.HTML(http.StatusUnauthorized, "pages/auth/login.html", gin.H{
			"title": "Login",
			"error": "Email/Username dan password harus diisi",
		})
		return
	}

	// Extract username from email if needed (e.g., john.doe@example.com -> john.doe)
	username := strings.Split(identifier, "@")[0]
	username = strings.ToLower(username)

	var user models.User
	var authenticated bool

	// Check if we're in development mode
	isDev := IsDevelopmentMode()

	// First, try to find existing user in database (by email or username prefix)
	userExists := database.DB.Where("LOWER(email) LIKE ?", username+"%").First(&user).Error == nil

	// Development mode: try local password first if user exists with password
	if isDev && userExists && user.PasswordHash != "" {
		if utils.CheckPasswordHash(password, user.PasswordHash) {
			authenticated = true
			log.Printf("[AUTH] User %s authenticated via local password (dev mode)", username)
		}
	}

	// If local auth failed or not in dev mode, try LDAP
	if !authenticated && IsLDAPEnabled() {
		fullName, department, err := AuthenticateLDAP(username, password)
		if err != nil {
			log.Printf("[AUTH] LDAP authentication failed for %s: %v", username, err)
			// Show error to user
			c.HTML(http.StatusUnauthorized, "pages/auth/login.html", gin.H{
				"title": "Login",
				"error": "Invalid email/username atau password",
			})
			return
		}

		authenticated = true

		// Sync user to database (upsert)
		if userExists {
			// Update existing user's full name if different
			if user.FullName != fullName && fullName != "" {
				database.DB.Model(&user).Update("full_name", fullName)
				log.Printf("[AUTH] Updated full name for user %s: %s", username, fullName)
			}
		} else {
			// Create new user from LDAP
			email := username + "@" + GetLDAPConfig().Domain
			user = models.User{
				Email:        email,
				PasswordHash: "", // No local password for LDAP users
				FullName:     fullName,
				Role:         models.RoleConsumer, // Default role, admin can change later
				IsActive:     true,
			}
			if err := database.DB.Create(&user).Error; err != nil {
				log.Printf("[AUTH] Failed to create user %s in database: %v", username, err)
				c.HTML(http.StatusInternalServerError, "pages/auth/login.html", gin.H{
					"title": "Login",
					"error": "Gagal menyimpan data user",
				})
				return
			}
			log.Printf("[AUTH] Created new user from LDAP: %s (ID: %s, Department: %s)", username, user.ID, department)
		}
	}

	// If still not authenticated (LDAP not enabled and local auth failed)
	if !authenticated {
		// Try one more time with direct email match for backward compatibility
		if err := database.DB.Where("email = ?", identifier).First(&user).Error; err == nil {
			if utils.CheckPasswordHash(password, user.PasswordHash) {
				authenticated = true
				log.Printf("[AUTH] User %s authenticated via direct email match", identifier)
			}
		}
	}

	if !authenticated {
		c.HTML(http.StatusUnauthorized, "pages/auth/login.html", gin.H{
			"title": "Login",
			"error": "Invalid email/username atau password",
		})
		return
	}

	// Check if user is active
	if !user.IsActive {
		c.HTML(http.StatusUnauthorized, "pages/auth/login.html", gin.H{
			"title": "Login",
			"error": "Akun tidak aktif. Hubungi Admin.",
		})
		return
	}

	// Set Cookie
	c.SetCookie("user_id", user.ID.String(), 3600*8, "/", "", false, true)
	c.SetCookie("user_role", string(user.Role), 3600*8, "/", "", false, true)

	log.Printf("[AUTH] User %s logged in successfully (Role: %s)", user.Email, user.Role)

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

// Logout godoc
// @Summary      User logout
// @Description  Log out user and clear session cookies
// @Tags         Auth
// @Produce      html
// @Success      302  {string}  string  "Redirect to login"
// @Router       /auth/logout [get]
func Logout(c *gin.Context) {
	c.SetCookie("user_id", "", -1, "/", "", false, true)
	c.SetCookie("user_role", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/auth/login")
}

