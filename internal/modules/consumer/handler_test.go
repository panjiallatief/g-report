package consumer

import (
	"it-broadcast-ops/internal/models"
	"it-broadcast-ops/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"
    "time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDashboard_Access(t *testing.T) {
	db := testutil.SetupTestDB()

	// Seed User
	userID := uuid.New()
	user := models.User{
		ID: userID,
		Email: "consumer@example.com",
		Role:  models.RoleConsumer,
		FullName: "Consumer Test",
	}
	db.Create(&user)
    
    // Seed Ticket for Dashboard
    ticket := models.Ticket{
        RequesterID: userID,
        Subject: "Test Ticket",
        Status: models.StatusOpen,
        CreatedAt: time.Now(),
    }
    db.Create(&ticket)

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.LoadHTMLGlob("../../../web/templates/**/*")
	
	// Mock Auth Middleware or manually set cookie in request
	// For simplicity, we are testing the handler logic, assuming middleware passes.
    // However, Dashboard handler reads cookie "user_id".
    
    r.GET("/consumer", Dashboard)

	req, _ := http.NewRequest("GET", "/consumer", nil)
	// Inject Cookie manually
	cookie := &http.Cookie{
		Name:  "user_id",
		Value: userID.String(),
	}
	req.AddCookie(cookie)
	
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
    // Check if body contains ticket subject
    assert.Contains(t, w.Body.String(), "Test Ticket")
}

func TestSearchBigBook(t *testing.T) {
    db := testutil.SetupTestDB()
    
    // Seed Article
    article := models.KnowledgeArticle{
        Title: "Wifi Troubleshooting",
        Content: "Restart router",
        IsVerified: true,
    }
    db.Create(&article)
    
    gin.SetMode(gin.TestMode)
    r := gin.Default()
    r.GET("/consumer/bigbook", SearchBigBook)
    
    // 1. Test Search
    req, _ := http.NewRequest("GET", "/consumer/bigbook?q=Wifi", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Body.String(), "Wifi Troubleshooting")
    
    // 2. Test Empty Search (List all)
    reqEmpty, _ := http.NewRequest("GET", "/consumer/bigbook", nil)
    wEmpty := httptest.NewRecorder()
    r.ServeHTTP(wEmpty, reqEmpty)
    
    assert.Equal(t, http.StatusOK, wEmpty.Code)
    assert.Contains(t, wEmpty.Body.String(), "Wifi Troubleshooting")
}
