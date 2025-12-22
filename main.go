package main

import (
	"log"
	"os"
	
	"github.com/joho/godotenv"
	"it-broadcast-ops/internal/database"
	redisClient "it-broadcast-ops/internal/redis"
	"it-broadcast-ops/internal/server"
	_ "it-broadcast-ops/docs" // Swagger docs
)

// @title           G-Report API
// @version         1.0
// @description     IT Broadcast Operations & Helpdesk System API. Provides endpoints for ticket management, knowledge base (Big Book), shift scheduling, and real-time notifications.

// @contact.name    IT Support
// @license.name    MIT

// @host            localhost:8080
// @BasePath        /

// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name user_id

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found")
	}

	// Connect to Database
	database.Connect()

	// Connect to Redis (optional - app works without it)
	if err := redisClient.Init(); err != nil {
		log.Println("⚠️  Redis not available. Chat will use polling instead of real-time.")
	}

	// Setup Router
	r := server.NewRouter()

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// Check for SSL certificates in multiple locations
	// Priority: cert/ folder (real certs) > root folder (self-signed)
	certFile := ""
	keyFile := ""
	
	// Check cert/ folder first (for real certificates like b-universe)
	if _, err := os.Stat("cert/fullchain.pem"); err == nil {
		if _, err := os.Stat("cert/privkey.pem"); err == nil {
			certFile = "cert/fullchain.pem"
			keyFile = "cert/privkey.pem"
		}
	}
	
	// Fallback to root folder (self-signed)
	if certFile == "" {
		if _, err := os.Stat("server.crt"); err == nil {
			if _, err := os.Stat("server.key"); err == nil {
				certFile = "server.crt"
				keyFile = "server.key"
			}
		}
	}
	
	if certFile != "" && keyFile != "" {
		log.Println("🔐 SSL Certificates found. Starting server in HTTPS mode...")
		log.Printf("   Using: %s, %s", certFile, keyFile)
		log.Println("🌐 Access at https://localhost:" + port)
		
		if err := r.RunTLS(":"+port, certFile, keyFile); err != nil {
			log.Fatal("Server failed to start (HTTPS): ", err)
		}
		return
	}

	// Fallback ke HTTP biasa jika tidak ada sertifikat
	log.Println("🔓 No SSL certificates found. Starting server in HTTP mode...")
	log.Println("⚠️  Service Workers require HTTPS or localhost. For mobile testing, consider using ngrok or generating certs.")
	log.Println("Server started on :" + port)
	
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}