package notification

import (
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"net/http"

	// Import package service notification dengan alias "notifService"
	// Ini wajib karena nama package handler kita juga "notification"
	notifService "it-broadcast-ops/internal/notification"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log"
)

func RegisterRoutes(r *gin.Engine) {
	// Panggil fungsi dari package service menggunakan alias
	notifService.InitKeys()

	// Public endpoint untuk get VAPID Key (agar frontend tahu key mana yg dipakai)
	r.GET("/notifications/vapid-public-key", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"publicKey": notifService.VapidPublicKey})
	})

	// Endpoint Subscribe (Butuh Login)
	r.POST("/notifications/subscribe", auth.AuthRequired(), Subscribe)
	
	r.POST("/notifications/unsubscribe", auth.AuthRequired(), Unsubscribe)
	r.POST("/notifications/test", auth.AuthRequired(), SendTestNotification)
}

type SubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func SendTestNotification(c *gin.Context) {
    userIDStr, _ := c.Cookie("user_id")
    userID, _ := uuid.Parse(userIDStr)

   err := notifService.SendNotificationToUser(
		userID.String(), 
		"🔔 Test Notifikasi Berhasil!",
		"Sistem notifikasi Anda berjalan normal.",
		"/staff/alerts",
	)

    if err != nil {
        c.JSON(500, gin.H{"error": "Gagal mengirim notifikasi"})
        return
    }

    c.JSON(200, gin.H{"message": "Notifikasi terkirim!"})
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
		log.Printf("Updating existing subscription for user %s (Endpoint: len=%d)", userID, len(req.Endpoint))
		existing.UserID = userID
		existing.P256dh = req.Keys.P256dh
		existing.Auth = req.Keys.Auth
		database.DB.Save(&existing)
	} else {
		// Create Baru
		log.Printf("Creating NEW subscription for user %s (Endpoint: len=%d)", userID, len(req.Endpoint))
		newSub := models.PushSubscription{
			UserID:   userID,
			Endpoint: req.Endpoint,
			P256dh:   req.Keys.P256dh,
			Auth:     req.Keys.Auth,
		}
		if result := database.DB.Create(&newSub); result.Error != nil {
			log.Printf("ERROR saving subscription: %v", result.Error)
			c.JSON(500, gin.H{"error": "Failed to save subscription"})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Subscribed successfully"})
}

// [NEW] Handler Unsubscribe
func Unsubscribe(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Hapus subscription berdasarkan endpoint (spesifik per device)
	if err := database.DB.Where("endpoint = ?", req.Endpoint).Delete(&models.PushSubscription{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Unsubscribed"})
}