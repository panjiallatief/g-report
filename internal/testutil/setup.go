package testutil

import (
	"it-broadcast-ops/internal/database"
	"log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SetupTestDB menghubungkan ke database khusus testing
func SetupTestDB() *gorm.DB {
    // Pastikan DSN mengarah ke database TEST, bukan produksi
	dsn := "host=localhost user=postgres password=123456 dbname=it_broadcast_test port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect to test database")
	}
	
	// Override global DB variable agar handler menggunakan DB test
	database.DB = db 
	
	// Reset Data (Clean slate) setiap kali test jalan
	// Reset Data (Clean slate) setiap kali test jalan
	// Hapus seluruh schema public untuk memastikan bersih total
    if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
        log.Println("Error dropping schema:", err)
    }
    if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
        log.Println("Error creating schema:", err)
    }
    
    // Optional: Restore default grants
    db.Exec("GRANT ALL ON SCHEMA public TO postgres")
    db.Exec("GRANT ALL ON SCHEMA public TO public")
	
    log.Println("Schema initialized.")
	
	// AutoMigrate (Running SQL Schema + GORM)
	database.AutoMigrate(db) 
	
	return db
}
