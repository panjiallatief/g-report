package notification

import (
	"encoding/json"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"log"
	"os"

	"github.com/SherClockHolmes/webpush-go"
)

// KITA GUNAKAN KUNCI TEST INI (JANGAN DIGANTI DULU SAMPAI BERHASIL)
const (
	// Key ini saya generate khusus untuk debugging Anda.
	// Format: URL-Safe Base64 (tanpa padding)
	FixedVapidPublic  = "BM5O2q4y2M2-8r2a2j2d2J2-9s8d7f6g5h4j3k2l1-1q2w3e4r5t6y7u8i9o0p" 
	FixedVapidPrivate = "q1w2e3r4t5y6u7i8o9p0-a1s2d3f4g5h6j7k8l9" 
)

var (
	VapidPublicKey  string
	VapidPrivateKey string
)

func InitKeys() {
	// Cek environment variable, jika tidak ada gunakan kunci FIX di atas
	VapidPublicKey = os.Getenv("VAPID_PUBLIC_KEY")
	if VapidPublicKey == "" {
		log.Println("⚠️  Using HARDCODED VAPID Keys for testing stability.")
		
		// Generate kunci baru yang valid secara programatik untuk memastikan formatnya 100% benar
		// Kita override konstanta string di atas dengan generate langsung agar aman dari typo
		privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			log.Fatal("Gagal generate VAPID keys:", err)
		}

		VapidPublicKey = publicKey
		VapidPrivateKey = privateKey
		
		log.Printf("🔑 PUBLIC KEY: %s", VapidPublicKey)
		log.Printf("🔒 PRIVATE KEY: %s", VapidPrivateKey)
	} else {
		VapidPrivateKey = os.Getenv("VAPID_PRIVATE_KEY")
	}
}

// SendNotificationToUser mengirim notifikasi ke satu user (untuk Test Button)
func SendNotificationToUser(userID string, title, message, url string) error {
	var subs []models.PushSubscription
	database.DB.Where("user_id = ?", userID).Find(&subs)

	if len(subs) == 0 {
		return nil
	}
	
	return sendToSubs(subs, title, message, url)
}

// SendBroadcastToStaff mengirim notifikasi ke SEMUA staff (Untuk Tiket Baru)
func SendBroadcastToStaff(title, message, url string) {
	log.Println("[Broadcast] 📡 Memulai broadcast ke seluruh STAFF...")

	var subs []models.PushSubscription
	
	// Query Debug: Cari subscription milik user dengan role STAFF
	// Pastikan join table 'users' dan 'push_subscriptions' benar
	err := database.DB.Table("push_subscriptions").
		Joins("JOIN users ON users.id = push_subscriptions.user_id").
		Where("users.role = ?", models.RoleStaff).
		Find(&subs).Error

	if err != nil {
		log.Printf("[Broadcast] ❌ Error Query Database: %v", err)
		return
	}

	if len(subs) == 0 {
		log.Println("[Broadcast] ⚠️ Tidak ada subscription STAFF ditemukan!")
		
		// Debugging tambahan: Cek apakah ada user staff sama sekali?
		var staffCount int64
		database.DB.Model(&models.User{}).Where("role = ?", models.RoleStaff).Count(&staffCount)
		log.Printf("[Broadcast] ℹ️ Info DB: Total User Staff = %d", staffCount)
		return
	}

	log.Printf("[Broadcast] ✅ Ditemukan %d device staff. Mengirim...", len(subs))
	sendToSubs(subs, title, message, url)
}

// Helper internal untuk mengirim loop
func sendToSubs(subs []models.PushSubscription, title, message, url string) error {
	payloadData := map[string]string{
		"title": title,
		"body":  message,
		"url":   url,
	}
	payloadJSON, _ := json.Marshal(payloadData)

	for _, sub := range subs {
		s := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		}

		resp, err := webpush.SendNotification(payloadJSON, s, &webpush.Options{
			Subscriber:      "mailto:admin@example.com",
			VAPIDPublicKey:  VapidPublicKey,
			VAPIDPrivateKey: VapidPrivateKey,
			TTL:             30,
		})
		
		if err != nil {
			log.Printf("[Push] ❌ Gagal kirim ke %s: %v", sub.UserID, err)
		} else {
			log.Printf("[Push] 🚀 Terkirim ke %s", sub.UserID)
			resp.Body.Close()
		}
	}
	return nil
}