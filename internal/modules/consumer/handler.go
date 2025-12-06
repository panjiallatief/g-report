package consumer

import (
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"it-broadcast-ops/internal/notification"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RegisterRoutes(r *gin.Engine) {
	consumerGroup := r.Group("/consumer")
	consumerGroup.Use(auth.AuthRequired(), auth.RoleRequired(models.RoleConsumer))
	{
		consumerGroup.GET("", Dashboard)
		consumerGroup.GET("/bigbook", SearchBigBook)
		consumerGroup.GET("/articles/:id", ArticleDetail)
		consumerGroup.POST("/ticket", CreateTicket)

		// New Endpoints for Ticket Chat
		consumerGroup.GET("/tickets/:id/details", GetTicketDetailJSON)
		consumerGroup.POST("/tickets/:id/reply", ReplyTicket)
	}
}

func Dashboard(c *gin.Context) {
	var tickets []models.Ticket
	
	// Get User ID from cookie
	userIDStr, err := c.Cookie("user_id")
	if err == nil && userIDStr != "" {
		// Verify UUID format to prevent SQL errors
		if _, err := uuid.Parse(userIDStr); err == nil {
			database.DB.Where("requester_id = ?", userIDStr).Order("created_at desc").Limit(5).Find(&tickets)
		}
	}

	c.HTML(http.StatusOK, "consumer/dashboard.html", gin.H{
		"title": "Home",
		"tickets": tickets,
	})
}

// SearchBigBook updated to return alphabetical list if query is empty
func SearchBigBook(c *gin.Context) {
	query := c.Query("q")
	var articles []models.KnowledgeArticle
	
	db := database.DB.Model(&models.KnowledgeArticle{}).Where("is_verified = ?", true)

	if query != "" {
		// Jika ada search, cari berdasarkan judul atau konten
		db = db.Where("title ILIKE ? OR content ILIKE ?", "%"+query+"%", "%"+query+"%")
	} else {
		// Jika kosong, urutkan A-Z (Alphabetical)
		db = db.Order("title ASC")
	}

	// Limit results to keep payload light
	db.Limit(50).Find(&articles)
	
	c.JSON(200, articles)
}


func ArticleDetail(c *gin.Context) {
	id := c.Param("id")
	var article models.KnowledgeArticle
	if result := database.DB.Preload("Author").First(&article, "id = ?", id); result.Error != nil {
		c.Data(404, "text/html; charset=utf-8", []byte("<h1>Article not found</h1>"))
		return
	}

	// Increment View Count
	database.DB.Model(&article).UpdateColumn("views_count", article.ViewsCount+1)

	c.HTML(http.StatusOK, "consumer/article_detail.html", gin.H{
		"title":   article.Title,
		"article": article,
	})
}

func CreateTicket(c *gin.Context) {
	userIDStr, _ := c.Cookie("user_id")
	userID, _ := uuid.Parse(userIDStr)

	var proofURL string
    file, err := c.FormFile("proof_image")
    if err == nil {
        // 1. Validasi Ukuran (Max 5MB)
        if file.Size > 5*1024*1024 {
             // Opsional: Handle error file terlalu besar
        } else {
            // 2. Buat nama file unik: evidence_<timestamp>.<ext>
            ext := filepath.Ext(file.Filename)
            filename := fmt.Sprintf("evidence_%d%s", time.Now().UnixNano(), ext)
            
            // 3. Pastikan folder uploads/tickets ada
            saveDir := "web/uploads/tickets"
            os.MkdirAll(saveDir, 0755)
            
            savePath := filepath.Join(saveDir, filename)
            
            // 4. Simpan file
            if err := c.SaveUploadedFile(file, savePath); err == nil {
                // Simpan URL relatif untuk akses web
                proofURL = "/uploads/tickets/" + filename
            }
        }
    }
	category := c.PostForm("category")
	validCategories := map[string]bool{
		"AUDIO": true, "VIDEO": true, "IT_NETWORK": true, "SOFTWARE": true, "ELECTRICAL": true,
	}
	if !validCategories[category] {
		category = "IT_NETWORK" // Default fallback jika invalid
	}
	
	ticket := models.Ticket{
		Location:    models.LocationEnum(c.PostForm("location")), // Simplified mapping
		// Urgency -> Priority mapping needs care, for now assume compatible strings or map manually
		Priority:    models.PriorityNormal, // Default
		Category:    category,
		Subject:     c.PostForm("subject"),
		Description: c.PostForm("description"),
		ProofImageURL: proofURL, // <--- Masukkan URL gambar di sini
		RequesterID:   userID,
		Status:        models.StatusOpen,
        CreatedAt:     time.Now(), // Pastikan created_at terisi
	}
	
	if c.PostForm("urgency") == "ON_AIR_EMERGENCY" {
		ticket.Priority = models.PriorityUrgentOnAir
	} else if c.PostForm("urgency") == "PRE_PRODUCTION" { // Tambahan jika ada opsi ini
        ticket.Priority = models.PriorityHigh
    }

	if err := database.DB.Create(&ticket).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create ticket: " + err.Error()})
		return
	}

	
	// [PUSH NOTIFICATION TRIGGER]
	if ticket.Priority == models.PriorityUrgentOnAir {
		go notification.SendBroadcastToStaff(
			"🔥 URGENT: " + string(ticket.Location),
			ticket.Subject + " (ON AIR ISSUE)",
			"/staff/tickets/" + ticket.ID.String(),
		)
	} else {
		// Notif biasa (Opsional)
		go notification.SendBroadcastToStaff(
			"New Ticket: " + string(ticket.Location),
			ticket.Subject,
			"/staff/tickets/" + ticket.ID.String(),
		)
	}

	// Redirect back to dashboard with success message (or just reload)
	c.Redirect(http.StatusFound, "/consumer")
}


// NEW: Get Ticket Details & Activities (JSON for AlpineJS)
func GetTicketDetailJSON(c *gin.Context) {
	id := c.Param("id")
	var ticket models.Ticket
	
	// Fetch Ticket
	if err := database.DB.Preload("Requester").First(&ticket, "id = ?", id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Ticket not found"})
		return
	}

	// Fetch Activities (Chat History)
	var activities []models.TicketActivity
	database.DB.Preload("Actor").
		Where("ticket_id = ?", id).
		Order("created_at asc").
		Find(&activities)

	// Custom JSON Response to simplify frontend handling
	type ActivityView struct {
		ActorName   string
		ActorAvatar string
		ActionType  string
		Note        string
		Time        string
		IsMe        bool
	}
	
	var activityViews []ActivityView
	userIDStr, _ := c.Cookie("user_id")

	// Add Initial Description as first "Chat"
	activityViews = append(activityViews, ActivityView{
		ActorName:   ticket.Requester.FullName,
		ActorAvatar: ticket.Requester.AvatarURL,
		ActionType:  "CREATED",
		Note:        ticket.Description,
		Time:        ticket.CreatedAt.Format("15:04"),
		IsMe:        ticket.RequesterID.String() == userIDStr,
	})

	for _, act := range activities {
		activityViews = append(activityViews, ActivityView{
			ActorName:   act.Actor.FullName,
			ActorAvatar: act.Actor.AvatarURL,
			ActionType:  act.ActionType,
			Note:        act.Note,
			Time:        act.CreatedAt.Format("02 Jan 15:04"),
			IsMe:        act.ActorID.String() == userIDStr,
		})
	}

	c.JSON(200, gin.H{
		"ticket": ticket,
		"activities": activityViews,
	})
}

// NEW: Reply to Ticket (Consumer Side)
func ReplyTicket(c *gin.Context) {
	id := c.Param("id")
	message := c.PostForm("message")
	userIDStr, _ := c.Cookie("user_id")
	userID, _ := uuid.Parse(userIDStr)

	if message == "" {
		c.Redirect(http.StatusFound, "/consumer")
		return
	}

	activity := models.TicketActivity{
		TicketID:   uuid.MustParse(id),
		ActorID:    userID,
		ActionType: "REPLY",
		Note:       message,
		CreatedAt:  time.Now(),
	}
	database.DB.Create(&activity)

	// Jika tiket sudah resolved tapi user membalas, mungkin perlu di-reopen?
	// Untuk sekarang biarkan statusnya tetap, notifikasi akan masuk ke staff.
	
	c.Redirect(http.StatusFound, "/consumer")
}
