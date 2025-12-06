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
	DefaultVapidPublicKey  = "BKP_B_q-P2y_q-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a" // Contoh dummy pendek, diganti di init
	DefaultVapidPrivateKey = "your-private-key"
)

var (
	VapidPublicKey  string
	VapidPrivateKey string
)

func InitKeys() {
	// Coba load dari ENV, kalau tidak ada pakai hardcoded DEV keys
	// Note: Key di bawah ini digenerate untuk keperluan demo agar langsung jalan.
	// Jangan dipakai di production sungguhan.
	VapidPublicKey = os.Getenv("VAPID_PUBLIC_KEY")
	if VapidPublicKey == "" {
		VapidPublicKey = "BBNcwOmf-2q_Gq-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a-Z_q-a" 
		// Menggunakan key valid random untuk demo:
		VapidPublicKey = "BCmC6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6_6" // Placeholder, see logic below
		
		// Generate real keys on startup for this demo session if env missing
		privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
		if err == nil {
			VapidPrivateKey = privateKey
			VapidPublicKey = publicKey
			log.Println("Generated Ephemeral VAPID Keys for this session:")
			log.Println("Public:", VapidPublicKey)
		} else {
			log.Fatal("Failed to generate VAPID keys")
		}
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
		log.Println("No staff subscriptions found.")
		return
	}

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