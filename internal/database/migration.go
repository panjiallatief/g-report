package database

import (
	"io/ioutil"
	"it-broadcast-ops/internal/models"
	"log"

	"gorm.io/gorm"
)

// AutoMigrate executes the SQL schema if the user table doesn't exist
func AutoMigrate(db *gorm.DB) {
	// 1. Initial Schema Setup (Raw SQL)
	if !db.Migrator().HasTable("users") {
		log.Println("Initializing Database Schema...")
		schemaPath := "internal/models/optimize_schema.sql"
		content, err := ioutil.ReadFile(schemaPath)
		if err != nil {
			// Try relative path for tests (2 levels)
			schemaPath = "../../internal/models/optimize_schema.sql"
			content, err = ioutil.ReadFile(schemaPath)
			if err != nil {
				// Try relative path for tests (3 levels)
				schemaPath = "../../../internal/models/optimize_schema.sql"
				content, err = ioutil.ReadFile(schemaPath)
				if err != nil {
					log.Printf("Error reading schema file from %s: %v\n", schemaPath, err)
					return
				}
			}
		}
		if err := db.Exec(string(content)).Error; err != nil {
			log.Fatal("Failed to execute schema migration: ", err)
		}
		log.Println("Database Schema applied successfully!")
	}

	// 2. GORM AutoMigrate (Schema Evolution)
	// This ensures new fields (like Solution) are added even if table exists
	err := db.AutoMigrate(
		&models.Ticket{},
		&models.RoutineInstance{}, 
		&models.TicketActivity{},
		// Add other models here if they change
	)
	if err != nil {
		log.Println("GORM AutoMigrate failed: ", err)
	} else {
		log.Println("GORM AutoMigrate check completed.")
	}
}
