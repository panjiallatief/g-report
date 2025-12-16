package main

import (
	"log"
	"os"
	
	"github.com/joho/godotenv"
	"it-broadcast-ops/internal/database"
	redisClient "it-broadcast-ops/internal/redis"
	"it-broadcast-ops/internal/server"
)

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

	// Cek apakah sertifikat SSL ada (digunakan untuk mode HTTPS)
	certFile := "server.crt"
	keyFile := "server.key"

	if _, err := os.Stat(certFile); err == nil {
		if _, err := os.Stat(keyFile); err == nil {
			log.Println("🔐 SSL Certificates found. Starting server in HTTPS mode...")
			log.Println("🌐 Access at https://localhost:" + port)
			
			if err := r.RunTLS(":"+port, certFile, keyFile); err != nil {
				log.Fatal("Server failed to start (HTTPS): ", err)
			}
			return
		}
	}

	// Fallback ke HTTP biasa jika tidak ada sertifikat
	log.Println("🔓 No SSL certificates found. Starting server in HTTP mode...")
	log.Println("⚠️  Service Workers require HTTPS or localhost. For mobile testing, consider using ngrok or generating certs.")
	log.Println("Server started on :" + port)
	
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}