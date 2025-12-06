package staff

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RegisterRoutes(r *gin.Engine) {
	staffGroup := r.Group("/staff")
	staffGroup.Use(auth.AuthRequired(), auth.RoleRequired(models.RoleStaff))
	{
		staffGroup.GET("", Dashboard)
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
	// 1. Get stats
	var openTicketsCount int64
	database.DB.Model(&models.Ticket{}).Where("status IN ?", []models.TicketStatus{models.StatusOpen, models.StatusInProgress}).Count(&openTicketsCount)

	var urgentCount int64
	database.DB.Model(&models.Ticket{}).Where("priority = ? AND status != ?", models.PriorityUrgentOnAir, models.StatusResolved).Count(&urgentCount)

	// 2. Get Active Tickets (Include Handover)
	var activeTickets []models.Ticket
	// Show tickets assigned to current user OR unassigned OR handover tickets
	// For simplicity in this monolithic view, show ALL Open/InProgress/Handover tickets
	database.DB.Where("status IN ?", []models.TicketStatus{models.StatusOpen, models.StatusInProgress, models.StatusHandover}).
		Preload("Requester").
		Order("case when priority = 'URGENT_ON_AIR' then 1 else 2 end, created_at asc").
		Find(&activeTickets)

	// 3. Get Routine Tasks
	var routineInstances []models.RoutineInstance
	userIDStr, _ := c.Cookie("user_id")
	database.DB.Preload("Template").Where("assigned_user_id = ? AND status = 'PENDING'", userIDStr).Find(&routineInstances)

	// Fetch current user for Profile Picture in header
	var user models.User
	database.DB.First(&user, "id = ?", userIDStr)

	// Prepare View Data for Routines (Parse JSON)
	type RoutineView struct {
		ID       uuid.UUID
		Title    string
		DueTime  string
		Items    map[string]bool
	}
	var routineViews []RoutineView
	
	// ... loop ...

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
		"tickets":      activeTickets,
		"routines":     routineViews,
		"user":         user,
	})
}

// ... ToggleRoutineItem ...

// ... TicketDetail ...

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

func SearchBigBookJSON(c *gin.Context) {
	query := c.Query("q")
	var results []map[string]interface{}
	
	// 1. Articles
	var articles []models.KnowledgeArticle
	if query != "" {
		database.DB.Where("title ILIKE ? OR content ILIKE ?", "%"+query+"%", "%"+query+"%").Limit(5).Find(&articles)
	}
	for _, a := range articles {
		results = append(results, map[string]interface{}{
			"Title": a.Title,
			"Category": a.Category,
			"Content": a.Content,
			"Type": "Article",
		})
	}

	// 2. Candidate Tickets (Resolved)
	var tickets []models.Ticket
	if query != "" {
		database.DB.Where("status = ? AND solution != '' AND (subject ILIKE ? OR solution ILIKE ?)", models.StatusResolved, "%"+query+"%", "%"+query+"%").
			Limit(5).Find(&tickets)
	}
	for _, t := range tickets {
		results = append(results, map[string]interface{}{
			"Title": t.Subject,
			"Category": "Ticket Solution",
			"Content": t.Solution,
			"Type": "Ticket",
		})
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

	// 1. Simpan Activity (Chat)
	activity := models.TicketActivity{
		TicketID:   uuid.MustParse(id),
		ActorID:    userID,
		ActionType: "REPLY",
		Note:       message,
		CreatedAt:  time.Now(),
	}
	database.DB.Create(&activity)

	// 2. Auto-update status: Jika masih OPEN -> Ubah jadi IN_PROGRESS
	// Logika: Jika teknisi membalas, berarti tiket sedang dikerjakan (MTTA stop)
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