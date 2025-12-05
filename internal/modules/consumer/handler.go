package consumer

import (
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"net/http"

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

func SearchBigBook(c *gin.Context) {
	query := c.Query("q")
	var articles []models.KnowledgeArticle
	
	if query != "" {
		database.DB.Where("title ILIKE ? OR content ILIKE ?", "%"+query+"%", "%"+query+"%").Limit(10).Find(&articles)
	}
	
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

	ticket := models.Ticket{
		Location:    models.LocationEnum(c.PostForm("location")), // Simplified mapping
		// Urgency -> Priority mapping needs care, for now assume compatible strings or map manually
		Priority:    models.PriorityNormal, // Default
		Category:    c.PostForm("category"),
		Subject:     c.PostForm("subject"),
		Description: c.PostForm("description"),
		RequesterID: userID,
		Status:      models.StatusOpen,
	}
	
	if c.PostForm("urgency") == "ON_AIR_EMERGENCY" {
		ticket.Priority = models.PriorityUrgentOnAir
	}

	if err := database.DB.Create(&ticket).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create ticket: " + err.Error()})
		return
	}

	// Redirect back to dashboard with success message (or just reload)
	c.Redirect(http.StatusFound, "/consumer")
}
