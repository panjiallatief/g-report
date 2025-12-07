Panduan Strategi Testing untuk IT Broadcast Ops (G-Report)

Untuk arsitektur Monolith Modular dengan Go, Gin, dan GORM, strategi terbaik adalah Testing Pyramid yang dimodifikasi. Fokus utama harus pada Integration Testing karena sebagian besar logika bisnis Anda terikat langsung dengan database.

1. Unit Testing (Pengujian Logika Murni)

Tujuan: Menguji fungsi-fungsi kecil yang independen (tidak butuh database atau server HTTP).
Target File: internal/utils/, helper functions, kalkulasi SLA.

Contoh Kasus: Menguji fungsi hashing password di internal/utils/crypto.go.

Buat file baru: internal/utils/crypto_test.go

package utils

import (
	"testing"
	"[github.com/stretchr/testify/assert](https://github.com/stretchr/testify/assert)" // Library populer untuk assert
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "rahasia123"

	// 1. Test Hash
	hash, err := HashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// 2. Test Check (Correct)
	match := CheckPasswordHash(password, hash)
	assert.True(t, match, "Password harus cocok dengan hash")

	// 3. Test Check (Wrong)
	matchWrong := CheckPasswordHash("salah123", hash)
	assert.False(t, matchWrong, "Password salah tidak boleh cocok")
}


2. Integration Testing (Handler + Database) [PALING PENTING]

Tujuan: Memastikan Gin handler berfungsi, routing benar, dan query GORM ke database berhasil.
Target File: internal/auth/handler.go, internal/modules/*/handler.go.

Karena Anda menggunakan variabel global database.DB, strategi terbaik adalah menggunakan Test Database terpisah (misal: it_broadcast_test) agar tidak merusak data asli.

A. Setup Helper

Buat file internal/testutil/setup.go (folder baru) untuk setup environment test.

package testutil

import (
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SetupTestDB menghubungkan ke database khusus testing
func SetupTestDB() *gorm.DB {
    // Pastikan DSN mengarah ke database TEST, bukan produksi
	dsn := "host=localhost user=postgres password=123456 dbname=it_broadcast_test port=5432 sslmode=disable"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	
	// Override global DB variable agar handler menggunakan DB test
	database.DB = db 
	
	// Reset Data (Clean slate) setiap kali test jalan
	db.Migrator().DropTable(&models.User{}, &models.Ticket{}) // Hapus tabel lama
	database.AutoMigrate(db) // Buat tabel baru via GORM AutoMigrate yang ada di package database
	
	return db
}


B. Contoh Test Login Handler

Buat file internal/auth/handler_test.go.

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

	"[github.com/gin-gonic/gin](https://github.com/gin-gonic/gin)"
	"[github.com/stretchr/testify/assert](https://github.com/stretchr/testify/assert)"
)

func TestLogin_Success(t *testing.T) {
	// 1. Setup DB & Router
	db := testutil.SetupTestDB()
	
	// Seed User Dummy ke DB Test
	hashed, _ := utils.HashPassword("123456")
	user := models.User{
		Email:        "test@example.com",
		PasswordHash: hashed,
		Role:         models.RoleStaff,
		FullName:     "Test Staff",
	}
	db.Create(&user)

	// Setup Gin Engine untuk Testing
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	// Load template path yang benar relative terhadap lokasi file test ini
	r.LoadHTMLGlob("../../web/templates/**/*") 
	RegisterRoutes(r)

	// 2. Buat Request HTTP (Simulasi Form Post dari Browser)
	formData := url.Values{}
	formData.Set("email", "test@example.com")
	formData.Set("password", "123456")

	req, _ := http.NewRequest("POST", "/auth/login", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// 3. Eksekusi Request
	r.ServeHTTP(w, req)

	// 4. Assertions (Verifikasi Hasil)
	assert.Equal(t, http.StatusFound, w.Code) // Harapannya 302 Redirect jika sukses
	
	// Cek apakah Cookie user_id terset
	cookie := w.Result().Cookies()
	foundCookie := false
	for _, c := range cookie {
		if c.Name == "user_id" {
			foundCookie = true
			break
		}
	}
	assert.True(t, foundCookie, "Cookie user_id harus ada setelah login sukses")
}


3. End-to-End (E2E) Testing

Tujuan: Menguji alur UI dari sudut pandang user (mengklik tombol, mengisi form, interaksi HTMX/AlpineJS).
Tools: Playwright (Direkomendasikan) atau Cypress.

Karena Anda menggunakan HTMX, test ini berguna untuk memastikan interaksi frontend berjalan lancar di browser sesungguhnya.

Contoh Skenario E2E:

Buka browser ke halaman /auth/login.

Ketik email & password manager.

Klik tombol "Login".

Tunggu redirect.

Pastikan elemen "Dashboard Manager" muncul di layar.

Rekomendasi Urutan Pengerjaan

Prioritas 1: Integration Tests (Handler & DB).
Karena logika bisnis Anda ada di dalam handler (contoh: internal/modules/consumer/handler.go bagian CreateTicket yang menyimpan ke DB dan mengirim notifikasi), testing di level ini memberikan jaminan keamanan paling tinggi.

Prioritas 2: Unit Tests (Utils).
Pastikan fungsi bantu (utils) berjalan benar.

Prioritas 3: UI Tests.
Lakukan ini jika fitur backend sudah stabil.

Tips Tambahan

Library Testify: Gunakan github.com/stretchr/testify untuk assertion yang lebih mudah dibaca daripada if != manual.

Test Database: Jangan pernah menjalankan test pada database development lokal Anda. Selalu gunakan database khusus test (_test) yang datanya dihapus-tulis setiap kali test berjalan.