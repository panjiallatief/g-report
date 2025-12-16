package staff

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	redisClient "it-broadcast-ops/internal/redis"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func History(c *gin.Context) {
	userIDStr, err := c.Cookie("user_id")
    if err != nil || userIDStr == "" {
        c.Redirect(http.StatusFound, "/auth/login")
        return
    }
    userID, _ := uuid.Parse(userIDStr)

	var urgentCount int64
	database.DB.Model(&models.Ticket{}).
		Where("priority = ? AND status != ?", models.PriorityUrgentOnAir, models.StatusResolved).
		Count(&urgentCount)
	
	var ticketIDs []uuid.UUID
	database.DB.Model(&models.TicketActivity{}).
		Where("actor_id = ? AND action_type = ?", userID, "RESOLVE").
		Pluck("ticket_id", &ticketIDs)

	var resolvedTickets []models.Ticket
	if len(ticketIDs) > 0 {
		database.DB.Where("id IN ?", ticketIDs).
			Preload("Requester").
			Order("resolved_at desc").
			Find(&resolvedTickets)
	}

	c.HTML(http.StatusOK, "staff/history.html", gin.H{
		"title":       "Riwayat Pekerjaan",
		"tickets":     resolvedTickets,
		"urgentCount": urgentCount,
	})
}

func RegisterRoutes(r *gin.Engine) {
	staffGroup := r.Group("/staff")
	staffGroup.Use(auth.AuthRequired(), auth.RoleRequired(models.RoleStaff))
	{
		staffGroup.GET("", Dashboard)
		staffGroup.GET("/history", History)
		staffGroup.GET("/tickets/list", TicketList) // HTMX Partial
		staffGroup.GET("/tickets/:id", TicketDetail)
		staffGroup.POST("/tickets/:id/handover", HandoverTicket)
		staffGroup.POST("/tickets/:id/resolve", ResolveTicket)
		staffGroup.POST("/routine/:id/toggle", ToggleRoutineItem)
		staffGroup.POST("/tickets/:id/reply", ReplyTicket) 
		staffGroup.GET("/bigbook", BigBook)
		staffGroup.GET("/bigbook/search", SearchBigBookJSON)
		staffGroup.GET("/articles/:id", ArticleDetail)
		staffGroup.GET("/profile", Profile)
		staffGroup.GET("/alerts", Alerts)
		staffGroup.POST("/profile/update", UpdateProfile)
	}
}

func Dashboard(c *gin.Context) {
	var openTicketsCount int64
	database.DB.Model(&models.Ticket{}).Where("status IN ?", []models.TicketStatus{models.StatusOpen, models.StatusInProgress}).Count(&openTicketsCount)

	var urgentCount int64
	database.DB.Model(&models.Ticket{}).Where("priority = ? AND status != ?", models.PriorityUrgentOnAir, models.StatusResolved).Count(&urgentCount)

	// Fetch Active Tickets (Sama logic dengan TicketList)
	var activeTickets []models.Ticket
	database.DB.Where("status IN ?", []models.TicketStatus{models.StatusOpen, models.StatusInProgress, models.StatusHandover}).
		Preload("Requester").
		Order("case when priority = 'URGENT_ON_AIR' then 1 else 2 end, created_at asc").
		Find(&activeTickets)

	userIDStr, err := c.Cookie("user_id")
    if err != nil || userIDStr == "" {
        c.Redirect(http.StatusFound, "/auth/login")
        return
    }
var routineInstances []models.RoutineInstance
    database.DB.Preload("Template").Where("assigned_user_id = ? AND status = 'PENDING'", userIDStr).Find(&routineInstances)

	var user models.User
	database.DB.First(&user, "id = ?", userIDStr)

	type RoutineView struct {
		ID       uuid.UUID
		Title    string
		DueTime  string
		Items    map[string]bool
	}
	var routineViews []RoutineView
	
	for _, r := range routineInstances {
		var items map[string]bool
		if len(r.ChecklistState) > 0 {
			json.Unmarshal(r.ChecklistState, &items)
		}
		
		routineViews = append(routineViews, RoutineView{
			ID:      r.ID,
			Title:   r.Template.Title,
			DueTime: r.DueAt.Format("15:04"),
			Items:   items,
		})
	}

	c.HTML(http.StatusOK, "staff/dashboard.html", gin.H{
		"title":        "Staff Dashboard",
		"ticketCount":  openTicketsCount,
		"urgentCount":  urgentCount,
		"tickets":      activeTickets, // Kirim tiket awal agar tidak kosong saat load pertama
		"routines":     routineViews,
		"user":         user,
	})
}

// [FIX] Menggunakan Partial Template agar tidak stack/bertumpuk
func TicketList(c *gin.Context) {
	var activeTickets []models.Ticket
	database.DB.Where("status IN ?", []models.TicketStatus{models.StatusOpen, models.StatusInProgress, models.StatusHandover}).
		Preload("Requester").
		Order("case when priority = 'URGENT_ON_AIR' then 1 else 2 end, created_at asc").
		Find(&activeTickets)

    // Render file partial yang baru dibuat
    c.HTML(http.StatusOK, "staff/ticket_list_partial.html", gin.H{
        "tickets": activeTickets,
    })
}

func HandoverTicket(c *gin.Context) {
	id := c.Param("id")
	note := c.PostForm("note")

	userIDStr, _ := c.Cookie("user_id")
	userID, _ := uuid.Parse(userIDStr)

	// Create Ticket Activity (Handover)
	activity := models.TicketActivity{
		TicketID:   uuid.MustParse(id),
		ActorID:    userID,
		ActionType: "HANDOVER",
		Note:       note,
	}
	database.DB.Create(&activity)

	// Update Ticket Status only
	database.DB.Model(&models.Ticket{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      models.StatusHandover,
		"is_handover": true,
	})

	c.Redirect(http.StatusFound, "/staff")
}

func ToggleRoutineItem(c *gin.Context) {
	id := c.Param("id")
	item := c.Query("item")
	
	var routine models.RoutineInstance
	if err := database.DB.First(&routine, "id = ?", id).Error; err != nil {
		c.Status(404)
		return
	}
	
	var items map[string]bool
	json.Unmarshal(routine.ChecklistState, &items)
	
	// Toggle
	items[item] = !items[item] // If not exists, becomes true (which is weird, but items should exist from seed)
	
	// Save back
	newState, _ := json.Marshal(items)
	
    // Check if all true
    allDone := true
    for _, done := range items {
        if !done {
            allDone = false
            break
        }
    }
    status := "PENDING"
	var completedAt *time.Time
    if allDone {
        status = "COMPLETED"
		now := time.Now()
		completedAt = &now
    }

	database.DB.Model(&routine).Updates(map[string]interface{}{
        "checklist_state": newState,
        "status": status,
		"completed_at": completedAt,
    })
	
	c.Status(200) // HTMX request, no need to redirect full page if just toggling
}

func TicketDetail(c *gin.Context) {
	id := c.Param("id")
	var ticket models.Ticket
	if err := database.DB.Preload("Requester").First(&ticket, "id = ?", id).Error; err != nil {
		c.String(404, "Ticket not found")
		return
	}
	
	// Fetch History/Activities
	var activities []models.TicketActivity
	database.DB.Preload("Actor").
		Where("ticket_id = ?", id).
		Order("created_at asc").
		Find(&activities)
	
	c.HTML(http.StatusOK, "staff/ticket_detail.html", gin.H{
		"ticket":     ticket,
		"activities": activities,
	})
}


func ResolveTicket(c *gin.Context) {
	id := c.Param("id")
	solution := c.PostForm("solution")
	now := time.Now()
	
	// Create Activity for Resolution
	userIDStr, _ := c.Cookie("user_id")
	userID, _ := uuid.Parse(userIDStr)
	
	activity := models.TicketActivity{
		TicketID:   uuid.MustParse(id),
		ActorID:    userID,
		ActionType: "RESOLVE",
		Note:       "Ticket Resolved. Solution: " + solution,
	}
	database.DB.Create(&activity)

	database.DB.Model(&models.Ticket{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      models.StatusResolved,
		"resolved_at": now,
		"solution":    solution,
	})
	
	c.Redirect(http.StatusFound, "/staff")
}

func BigBook(c *gin.Context) {
	query := c.Query("q")
	var articles []models.KnowledgeArticle
	
	db := database.DB.Model(&models.KnowledgeArticle{}).Preload("Author")
	if query != "" {
		db = db.Where("title ILIKE ? OR content ILIKE ?", "%"+query+"%", "%"+query+"%")
	}
	db.Find(&articles)
	
	// Also fetch Resolved Tickets with Solutions as "Candidates"
	var candidates []models.Ticket
	if query != "" {
		database.DB.Model(&models.Ticket{}).
			Where("status = ? AND solution != '' AND (subject ILIKE ? OR solution ILIKE ?)", models.StatusResolved, "%"+query+"%", "%"+query+"%").
			Limit(5).Find(&candidates)
	} else {
		database.DB.Model(&models.Ticket{}).
			Where("status = ? AND solution != ''", models.StatusResolved).
			Limit(5).Find(&candidates)
	}

	c.HTML(http.StatusOK, "staff/bigbook.html", gin.H{
		"title":      "Big Book (Knowledge Base)",
		"articles":   articles,
		"candidates": candidates,
		"query":      query,
	})
}

func Profile(c *gin.Context) {
	userIDStr, _ := c.Cookie("user_id")
	var user models.User
	database.DB.First(&user, "id = ?", userIDStr)
	
	c.HTML(http.StatusOK, "staff/profile.html", gin.H{
		"title": "My Profile",
		"user":  user,
		"now":   time.Now().Unix(),
	})
}

func Alerts(c *gin.Context) {
	var urgentTickets []models.Ticket
	database.DB.Where("priority = ? AND status != ?", models.PriorityUrgentOnAir, models.StatusResolved).
		Preload("Requester").
		Order("created_at desc").
		Find(&urgentTickets)

	c.HTML(http.StatusOK, "staff/alerts.html", gin.H{
		"title":   "Alerts - Urgent Tickets",
		"tickets": urgentTickets,
	})
}

// SearchBigBookJSON updated for alphabetical default list
func SearchBigBookJSON(c *gin.Context) {
	query := c.Query("q")
	var results []map[string]interface{}
	
	// 1. Articles
	var articles []models.KnowledgeArticle
	db := database.DB.Model(&models.KnowledgeArticle{})

	if query != "" {
		db = db.Where("title ILIKE ? OR content ILIKE ?", "%"+query+"%", "%"+query+"%")
	} else {
		// Default sort alphabetical
		db = db.Order("title ASC")
	}
	
	// Limit to 20 to avoid heavy load on dropdown
	db.Limit(20).Find(&articles)

	for _, a := range articles {
		results = append(results, map[string]interface{}{
			"ID": a.ID,
			"Title": a.Title,
			"Category": a.Category,
			"Content": a.Content,
			"Type": "Article",
		})
	}

	// 2. Candidate Tickets (Hanya jika ada query search, supaya list A-Z artikel tidak tercampur bising)
	if query != "" {
		var tickets []models.Ticket
		database.DB.Where("status = ? AND solution != '' AND (subject ILIKE ? OR solution ILIKE ?)", models.StatusResolved, "%"+query+"%", "%"+query+"%").
			Limit(5).Find(&tickets)
		
		for _, t := range tickets {
			results = append(results, map[string]interface{}{
				"ID": t.ID,
				"Title": t.Subject,
				"Category": "Ticket Solution",
				"Content": t.Solution,
				"Type": "Ticket",
			})
		}
	}
	
	c.JSON(200, results)
}


func UpdateProfile(c *gin.Context) {
	userIDStr, _ := c.Cookie("user_id")
	var user models.User
	if err := database.DB.First(&user, "id = ?", userIDStr).Error; err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	fullname := c.PostForm("fullname")
	if fullname != "" {
		user.FullName = fullname
	}

	file, err := c.FormFile("avatar")
	if err == nil {
		// Validation: Max 2MB
		if file.Size > 2*1024*1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 2MB)"})
			return
		}

		// Handle Avatar Upload
		ext := filepath.Ext(file.Filename)
		// If blob is sent as "avatar.jpg", ext will be ".jpg"
		if ext == "" {
			ext = ".jpg" 
		}
		
		// Use timestamp to assume uniqueness/busting cache? Or just UUID
		filename := fmt.Sprintf("%s_%d%s", user.ID.String(), time.Now().Unix(), ext)
		savePath := filepath.Join("web/uploads/avatars", filename)
		
		if err := c.SaveUploadedFile(file, savePath); err == nil {
			// URL path (served via /uploads route)
			user.AvatarURL = "/uploads/avatars/" + filename
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
			return
		}
	}

	database.DB.Save(&user)
	c.Status(http.StatusOK)
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

	c.HTML(http.StatusOK, "staff/article_detail.html", gin.H{
		"title":   article.Title,
		"article": article,
	})
}


func ReplyTicket(c *gin.Context) {
	id := c.Param("id")
	message := c.PostForm("message")
	
	if message == "" {
		c.Redirect(http.StatusFound, "/staff/tickets/"+id)
		return
	}

	userIDStr, _ := c.Cookie("user_id")
	userID, _ := uuid.Parse(userIDStr)

	// Get user info for the activity view
	var user models.User
	database.DB.First(&user, "id = ?", userID)

	// 1. Save Activity (Chat)
	activity := models.TicketActivity{
		TicketID:   uuid.MustParse(id),
		ActorID:    userID,
		ActionType: "REPLY",
		Note:       message,
		CreatedAt:  time.Now(),
	}
	database.DB.Create(&activity)

	// 2. Publish to Redis for real-time updates
	if redisClient.IsConnected() {
		activityView := map[string]interface{}{
			"ActorName":   user.FullName,
			"ActorAvatar": user.AvatarURL,
			"ActionType":  "REPLY",
			"Note":        message,
			"Time":        activity.CreatedAt.Format("02 Jan 15:04"),
			"IsMe":        false, // Determined client-side
			"ActorID":     userID.String(),
		}
		redisClient.Publish("chat:"+id, activityView)
		log.Println("[Redis] Staff published chat message for ticket:", id)
	}

	// 3. Auto-update status: If still OPEN -> Change to IN_PROGRESS
	var ticket models.Ticket
	database.DB.First(&ticket, "id = ?", id)
	
	if ticket.Status == models.StatusOpen {
		database.DB.Model(&ticket).Updates(map[string]interface{}{
			"status":            models.StatusInProgress,
			"first_response_at": time.Now(),
		})
	}

	c.Redirect(http.StatusFound, "/staff/tickets/"+id)
}