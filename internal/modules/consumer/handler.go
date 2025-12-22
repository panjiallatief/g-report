package consumer

import (
	"io"
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"it-broadcast-ops/internal/notification"
	redisClient "it-broadcast-ops/internal/redis"
	"log"
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

		// Endpoints for Ticket Chat
		consumerGroup.GET("/tickets/:id/details", GetTicketDetailJSON)
		consumerGroup.POST("/tickets/:id/reply", ReplyTicket)
		consumerGroup.GET("/tickets/:id/stream", TicketChatStream) // SSE for real-time
	}
}

// Dashboard godoc
// @Summary      Consumer dashboard
// @Description  Display consumer dashboard with recent tickets
// @Tags         Consumer
// @Produce      html
// @Security     CookieAuth
// @Success      200  {string}  string  "HTML page"
// @Router       /consumer [get]
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

// SearchBigBook godoc
// @Summary      Search knowledge base
// @Description  Search Big Book articles with Redis caching
// @Tags         Consumer
// @Produce      json
// @Security     CookieAuth
// @Param        q  query  string  false  "Search query"
// @Success      200  {array}  models.KnowledgeArticle
// @Router       /consumer/bigbook [get]
func SearchBigBook(c *gin.Context) {
	query := c.Query("q")
	cacheKey := "cache:bigbook:" + query
	
	// Try cache first (if Redis is connected)
	var articles []models.KnowledgeArticle
	if redisClient.IsConnected() {
		if err := redisClient.Get(cacheKey, &articles); err == nil {
			log.Println("[Cache] HIT for bigbook:", query)
			c.JSON(200, articles)
			return
		}
		log.Println("[Cache] MISS for bigbook:", query)
	}
	
	// Cache miss or Redis unavailable - query database
	db := database.DB.Model(&models.KnowledgeArticle{}).Where("is_verified = ?", true)

	if query != "" {
		db = db.Where("title ILIKE ? OR content ILIKE ?", "%"+query+"%", "%"+query+"%")
	} else {
		db = db.Order("title ASC")
	}

	db.Limit(50).Find(&articles)
	
	// Store in cache (5 minutes TTL)
	if redisClient.IsConnected() {
		redisClient.Set(cacheKey, articles, 5*time.Minute)
	}
	
	c.JSON(200, articles)
}


// ArticleDetail godoc
// @Summary      View article detail
// @Description  Display knowledge article details and increment view count
// @Tags         Consumer
// @Produce      html
// @Security     CookieAuth
// @Param        id  path  string  true  "Article ID"
// @Success      200  {string}  string  "HTML page"
// @Failure      404  {string}  string  "Article not found"
// @Router       /consumer/articles/{id} [get]
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

// CreateTicket godoc
// @Summary      Create new ticket
// @Description  Submit a new support ticket
// @Tags         Consumer
// @Accept       multipart/form-data
// @Produce      html
// @Security     CookieAuth
// @Param        location     formData  string  true   "Location"
// @Param        category     formData  string  true   "Category"
// @Param        subject      formData  string  true   "Subject"
// @Param        description  formData  string  true   "Description"
// @Param        urgency      formData  string  false  "Urgency"
// @Param        proof_image  formData  file    false  "Proof image"
// @Success      302  {string}  string  "Redirect to dashboard"
// @Failure      500  {object}  object  "Server error"
// @Router       /consumer/ticket [post]
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


// GetTicketDetailJSON godoc
// @Summary      Get ticket details
// @Description  Get ticket details and chat history as JSON
// @Tags         Consumer
// @Produce      json
// @Security     CookieAuth
// @Param        id  path  string  true  "Ticket ID"
// @Success      200  {object}  object  "Ticket and activities"
// @Failure      404  {object}  object  "Ticket not found"
// @Router       /consumer/tickets/{id}/details [get]
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

// ReplyTicket godoc
// @Summary      Reply to ticket
// @Description  Add a reply message to a ticket
// @Tags         Consumer
// @Accept       x-www-form-urlencoded
// @Produce      html
// @Security     CookieAuth
// @Param        id       path      string  true  "Ticket ID"
// @Param        message  formData  string  true  "Reply message"
// @Success      302  {string}  string  "Redirect to dashboard"
// @Router       /consumer/tickets/{id}/reply [post]
func ReplyTicket(c *gin.Context) {
	id := c.Param("id")
	message := c.PostForm("message")
	userIDStr, _ := c.Cookie("user_id")
	userID, _ := uuid.Parse(userIDStr)

	if message == "" {
		c.Redirect(http.StatusFound, "/consumer")
		return
	}

	// Get user info for the activity view
	var user models.User
	database.DB.First(&user, "id = ?", userID)

	activity := models.TicketActivity{
		TicketID:   uuid.MustParse(id),
		ActorID:    userID,
		ActionType: "REPLY",
		Note:       message,
		CreatedAt:  time.Now(),
	}
	database.DB.Create(&activity)

	// Publish to Redis for real-time updates
	if redisClient.IsConnected() {
		activityView := map[string]interface{}{
			"ActorName":   user.FullName,
			"ActorAvatar": user.AvatarURL,
			"ActionType":  "REPLY",
			"Note":        message,
			"Time":        activity.CreatedAt.Format("02 Jan 15:04"),
			"IsMe":        false, // Will be determined client-side
			"ActorID":     userID.String(),
		}
		redisClient.Publish("chat:"+id, activityView)
		log.Println("[Redis] Published chat message for ticket:", id)
	}

	c.Redirect(http.StatusFound, "/consumer")
}

// TicketChatStream godoc
// @Summary      Ticket chat SSE stream
// @Description  Server-Sent Events stream for real-time chat updates
// @Tags         Consumer
// @Produce      text/event-stream
// @Security     CookieAuth
// @Param        id  path  string  true  "Ticket ID"
// @Success      200  {string}  string  "SSE stream"
// @Router       /consumer/tickets/{id}/stream [get]
func TicketChatStream(c *gin.Context) {
	tickemID := c.Param("id")

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Check if Redis is available
	if !redisClient.IsConnected() {
		c.SSEvent("error", "Redis not available")
		return
	}

	// Subscribe to ticket channel
	sub := redisClient.Subscribe("chat:" + tickemID)
	defer sub.Close()

	ch := sub.Channel()

	// Send initial connection event
	c.SSEvent("connected", "Listening for chat updates")
	c.Writer.Flush()

	// Listen for messages
	for {
		select {
		case msg := <-ch:
			c.SSEvent("message", msg.Payload)
			c.Writer.Flush()
		case <-c.Request.Context().Done():
			// Client disconnected
			log.Println("[SSE] Client disconnected from ticket:", tickemID)
			return
		case <-time.After(30 * time.Second):
			// Send heartbeat to keep connection alive
			c.SSEvent("heartbeat", "ping")
			if _, err := io.WriteString(c.Writer, ""); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
