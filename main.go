package main

import (
	"log"
	"os"
	
	"github.com/joho/godotenv"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/server"
)

func main() {
	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found")
	}

	// Connect to Database
	database.Connect()

	// Setup Router
	r := server.NewRouter()

	log.Println("Server started on :" + os.Getenv("APP_PORT"))
	if err := r.Run(":" + os.Getenv("APP_PORT")); err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
