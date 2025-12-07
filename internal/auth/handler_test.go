package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"it-broadcast-ops/internal/models"
	"it-broadcast-ops/internal/testutil"
	"it-broadcast-ops/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLogin_Success(t *testing.T) {
	// 1. Setup DB
	db := testutil.SetupTestDB()
	
	// Seed User
	hashed, _ := utils.HashPassword("123456")
	user := models.User{
		Email:        "test@example.com",
		PasswordHash: hashed,
		Role:         models.RoleStaff,
		FullName:     "Test Staff",
	}
	db.Create(&user)

	// Setup Gin
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.LoadHTMLGlob("../../web/templates/**/*") // Adjust path relative to this test file
	RegisterRoutes(r)

	// 2. Create Request
	formData := url.Values{}
	formData.Set("email", "test@example.com")
	formData.Set("password", "123456")

	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// 3. Execute
	r.ServeHTTP(w, req)

	// 4. Assert
	assert.Equal(t, http.StatusFound, w.Code) 
	
	// Check Cookie
	cookie := w.Result().Cookies()
	foundCookie := false
	for _, c := range cookie {
		if c.Name == "user_id" {
			foundCookie = true
			break
		}
	}
	assert.True(t, foundCookie, "Cookie user_id must be set")
}

func TestLogin_Failure(t *testing.T) {
	testutil.SetupTestDB() // Clean DB

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.LoadHTMLGlob("../../web/templates/**/*")
	RegisterRoutes(r)

	formData := url.Values{}
	formData.Set("email", "wrong@example.com")
	formData.Set("password", "wrongpass")

	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
