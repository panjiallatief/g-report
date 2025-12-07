package notification

import (
	"encoding/json"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"log"
	"os"

	"github.com/SherClockHolmes/webpush-go"
)

// Default VAPID Keys for Development (Should be in .env in production)
// You can generate new ones using: webpush-go.GenerateVAPIDKeys()
const (
	DefaultVapidPublicKey  = "BOxM_47kkdGUPaHr9MYzecqhuK1QZ2lD-31sAkjkKyf1R-1yWp6VJZL5LY630MUrmQplp0RzLAkMB6cvqYoJmiU"
	DefaultVapidPrivateKey = "8TRcnVroyyQuv2bB3Zrmjk8IS3QMVBjbYAu8xR8ODhs"
)

var (
	VapidPublicKey  string
	VapidPrivateKey string
)

func InitKeys() {
	// Coba load dari ENV, kalau tidak ada pakai hardcoded DEV keys
	VapidPublicKey = os.Getenv("VAPID_PUBLIC_KEY")
	if VapidPublicKey == "" {
		VapidPublicKey = DefaultVapidPublicKey
		VapidPrivateKey = DefaultVapidPrivateKey
		
		log.Println("Using Fixed Development VAPID Keys")
	} else {
		VapidPrivateKey = os.Getenv("VAPID_PRIVATE_KEY")
	}
}

// SendBroadcastToStaff mengirim notifikasi ke SEMUA user dengan role STAFF
func SendBroadcastToStaff(title, message, url string) {
	// 1. Ambil semua subscription milik STAFF
	var subs []models.PushSubscription
	database.DB.Joins("JOIN users ON users.id = push_subscriptions.user_id").
		Where("users.role = ?", models.RoleStaff).
		Find(&subs)

	if len(subs) == 0 {
		log.Println("No staff subscriptions found in database.")
		return
	}

	log.Printf("Found %d staff subscriptions. Preparing to send push...", len(subs))

	// 2. Siapkan Payload JSON
	payloadData := map[string]string{
		"title": title,
		"body":  message,
		"url":   url,
	}
	payloadJSON, _ := json.Marshal(payloadData)

	// 3. Kirim ke setiap endpoint
	for _, sub := range subs {
		s := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		}

		// Send Notification
		resp, err := webpush.SendNotification(payloadJSON, s, &webpush.Options{
			Subscriber:      "mailto:admin@example.com",
			VAPIDPublicKey:  VapidPublicKey,
			VAPIDPrivateKey: VapidPrivateKey,
			TTL:             30,
		})
		
		if err != nil {
			log.Printf("Failed to send push to user %s: %v", sub.UserID, err)
			// Optional: Hapus subscription jika endpoint gone (410/404)
		} else {
			resp.Body.Close()
		}
	}
	log.Printf("Push notification sent to %d staff devices.", len(subs))
}