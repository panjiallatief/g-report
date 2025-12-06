package notification

import (
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"net/http"
	// "it-broadcast-ops/internal/notification"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RegisterRoutes(r *gin.Engine) {
	// Init Keys saat aplikasi start
	InitKeys()

	// Public endpoint untuk get VAPID Key (agar frontend tahu key mana yg dipakai)
	r.GET("/notifications/vapid-public-key", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"publicKey": VapidPublicKey})
	})

	// Endpoint Subscribe (Butuh Login)
	r.POST("/notifications/subscribe", auth.AuthRequired(), Subscribe)
}

type SubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func Subscribe(c *gin.Context) {
	var req SubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userIDStr, _ := c.Cookie("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid User ID"})
		return
	}

	// Cek apakah subscription dengan endpoint ini sudah ada
	var existing models.PushSubscription
	if err := database.DB.Where("endpoint = ?", req.Endpoint).First(&existing).Error; err == nil {
		// Update user jika endpoint sama tapi user beda (login di device sama)
		existing.UserID = userID
		existing.P256dh = req.Keys.P256dh
		existing.Auth = req.Keys.Auth
		database.DB.Save(&existing)
	} else {
		// Create Baru
		newSub := models.PushSubscription{
			UserID:   userID,
			Endpoint: req.Endpoint,
			P256dh:   req.Keys.P256dh,
			Auth:     req.Keys.Auth,
		}
		database.DB.Create(&newSub)
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Subscribed successfully"})
}