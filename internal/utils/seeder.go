package utils

import (
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"time"
)

func SeedDatabase(c *gin.Context) {
	password, _ := HashPassword("123456")

	// 1. Manager
	manager := models.User{
		Email:        "manager@example.com",
		PasswordHash: password,
		FullName:     "Manager Operasional",
		Role:         models.RoleManager,
	}
	if err := database.DB.Where("email = ?", manager.Email).FirstOrCreate(&manager).Error; err != nil {
		c.String(500, "Error seeding manager: "+err.Error())
		return
	}

	// 2. IT Staff (Generic)
	it := models.User{
		Email:        "it@example.com",
		PasswordHash: password,
		FullName:     "Staff IT Broadcast",
		Role:         models.RoleStaff,
	}
	if err := database.DB.Where("email = ?", it.Email).FirstOrCreate(&it).Error; err != nil {
		c.String(500, "Error seeding IT staff: "+err.Error())
		return
	}

	// 2.1 Specific IT Staff (Pagi & Malam)
	// IT Pagi
	idPagi, _ := uuid.Parse("2b9c3662-b756-49e1-9ae0-2a96858b39b8")
	itPagi := models.User{
		ID:           idPagi,
		Email:        "pagi@example.com",
		PasswordHash: password,
		FullName:     "Staff Pagi",
		Role:         models.RoleStaff,
	}
	if err := database.DB.Where("id = ?", idPagi).FirstOrCreate(&itPagi).Error; err != nil {
		c.String(500, "Error seeding IT Pagi: "+err.Error())
		return
	}

	// IT Malam
	idMalam, _ := uuid.Parse("2c839ed4-1418-4293-971b-8138a856f56c")
	itMalam := models.User{
		ID:           idMalam,
		Email:        "malam@example.com",
		PasswordHash: password,
		FullName:     "Staff Malam",
		Role:         models.RoleStaff,
	}
	if err := database.DB.Where("id = ?", idMalam).FirstOrCreate(&itMalam).Error; err != nil {
		c.String(500, "Error seeding IT Malam: "+err.Error())
		return
	}

	// 2.2 Shifts for Pagi & Malam (Today)
	today := time.Now().Truncate(24 * time.Hour)
	// Pagi: 07:00 - 15:00
	shiftPagi := models.Shift{
		UserID:    idPagi,
		StartTime: today.Add(7 * time.Hour),
		EndTime:   today.Add(15 * time.Hour),
		Label:     "Shift Pagi",
	}
	database.DB.FirstOrCreate(&shiftPagi, models.Shift{UserID: idPagi, StartTime: shiftPagi.StartTime})

	// Malam: 15:00 - 23:00
	shiftMalam := models.Shift{
		UserID:    idMalam,
		StartTime: today.Add(15 * time.Hour),
		EndTime:   today.Add(23 * time.Hour),
		Label:     "Shift Malam",
	}
	database.DB.FirstOrCreate(&shiftMalam, models.Shift{UserID: idMalam, StartTime: shiftMalam.StartTime})


	// 3. Consumer
	consumer := models.User{
		Email:        "user@example.com",
		PasswordHash: password,
		FullName:     "User Default",
		Role:         models.RoleConsumer,
	}
	if err := database.DB.Where("email = ?", consumer.Email).FirstOrCreate(&consumer).Error; err != nil {
		c.String(500, "Error seeding consumer: "+err.Error())
		return
	}

	// 4. Routine Templates & Instances
	// ... (Previous Routine Code) ...
	checklist := []byte(`["Cek Audio Mic 1-4", "Test Koneksi Internet", "Cek Lampu Studio", "Pastikan OBS Ready"]`)
	template := models.RoutineTemplate{
		Title:           "Pra-Siaran Studio 1",
		CronSchedule:    "0 7 * * *",
		DeadlineMinutes: 60,
		ChecklistItems:  checklist,
		CreatedBy:       manager.ID,
	}
	if err := database.DB.Where("title = ?", template.Title).FirstOrCreate(&template).Error; err != nil {
		c.String(500, "Error seeding template: "+err.Error())
		return
	}

	// 5. Knowledge Articles
	// Verified
	verifiedArticle := models.KnowledgeArticle{
		Title:      "Panduan Audio Mixing Studio 1",
		Content:    "Pastikan gain tidak clipping (merah). Gunakan compression ratio 4:1 untuk vokal.",
		Category:   "AUDIO",
		AuthorID:   itPagi.ID,
		IsVerified: true,
	}
	database.DB.FirstOrCreate(&verifiedArticle, models.KnowledgeArticle{Title: verifiedArticle.Title})

	// Unverified (Pending Manager Approval)
	unverifiedArticle := models.KnowledgeArticle{
		Title:      "Cara Reset Router Lantai 2",
		Content:    "Tekan tombol reset 10 detik. Password default: admin/admin.",
		Category:   "IT_NETWORK",
		AuthorID:   itMalam.ID,
		IsVerified: false,
	}
	database.DB.FirstOrCreate(&unverifiedArticle, models.KnowledgeArticle{Title: unverifiedArticle.Title})

	// Instance for IT Pagi (Assigned to morning staff)
	now := time.Now()
	due := now.Add(1 * time.Hour)
	initialState := []byte(`{"Cek Audio Mic 1-4": false, "Test Koneksi Internet": false, "Cek Lampu Studio": false, "Pastikan OBS Ready": false}`)
	
	instance := models.RoutineInstance{
		TemplateID:     template.ID,
		AssignedUserID: itPagi.ID, // Assign to Pagi
		ChecklistState: initialState,
		GeneratedAt:    now,
		DueAt:          due,
		Status:         "PENDING",
	}
	// Always create a fresh instance for demo purposes if not exists
	var count int64
	database.DB.Model(&models.RoutineInstance{}).Where("assigned_user_id = ? AND status = 'PENDING'", itPagi.ID).Count(&count)
	if count == 0 {
		database.DB.Create(&instance)
	}

	c.String(200, "Database Seeded! Accounts: manager, it, pagi, malam, user (Password: 123456)")
}
