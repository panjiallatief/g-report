package manager

import (
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	
	"encoding/csv"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"strings"
	"time"
	"it-broadcast-ops/internal/auth"
)

func RegisterRoutes(r *gin.Engine) {
	managerGroup := r.Group("/manager")
	managerGroup.Use(auth.AuthRequired(), auth.RoleRequired(models.RoleManager))
	{
	managerGroup.POST("/shifts/create", CreateShift)

	managerGroup.GET("", Dashboard)
	managerGroup.POST("/articles/:id/verify", VerifyArticle)
	managerGroup.POST("/shifts/import", ImportSchedule)
	}
}

func CreateShift(c *gin.Context) {
	staffID := c.PostForm("staff_id")
	label := c.PostForm("label")
	startStr := c.PostForm("start_time")
	endStr := c.PostForm("end_time")

	// Basic Validation
	if staffID == "" || startStr == "" || endStr == "" {
		c.Redirect(http.StatusFound, "/manager?error=MissingFields")
		return
	}

	layout := "2006-01-02T15:04" // datetime-local format
	startTime, err1 := time.Parse(layout, startStr)
	endTime, err2 := time.Parse(layout, endStr)

	if err1 != nil || err2 != nil {
		c.Redirect(http.StatusFound, "/manager?error=InvalidTime")
		return
	}

	shift := models.Shift{
		UserID:    uuid.MustParse(staffID),
		StartTime: startTime,
		EndTime:   endTime,
		Label:     label,
	}

	if err := database.DB.Create(&shift).Error; err != nil {
		c.Redirect(http.StatusFound, "/manager?error=DbError")
		return
	}

	c.Redirect(http.StatusFound, "/manager")
}

func Dashboard(c *gin.Context) {
	// 1. KPI Calculations (Naive implementation for MVP)
	// MTTA: Avg time from CreatedAt to FirstResponseAt
	var mttaPtr *float64
	database.DB.Model(&models.Ticket{}).Select("AVG(EXTRACT(EPOCH FROM (first_response_at - created_at))/60)").
		Where("first_response_at IS NOT NULL").Scan(&mttaPtr)
	
	mtta := 0.0
	if mttaPtr != nil {
		mtta = *mttaPtr
	}

	// MTTR: Avg time from CreatedAt to ResolvedAt
	var mttrPtr *float64
	database.DB.Model(&models.Ticket{}).Select("AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))/60)").
		Where("resolved_at IS NOT NULL").Scan(&mttrPtr)

	mttr := 0.0
	if mttrPtr != nil {
		mttr = *mttrPtr
	}
	
	// FCR: (Tickets Resolved without Handover / Total Resolved) * 100
	var totalResolved int64
	var fcrCount int64
	database.DB.Model(&models.Ticket{}).Where("status = ?", models.StatusResolved).Count(&totalResolved)
	database.DB.Model(&models.Ticket{}).Where("status = ? AND is_handover = ?", models.StatusResolved, false).Count(&fcrCount)
	
	fcrRate := 0.0
	if totalResolved > 0 {
		fcrRate = (float64(fcrCount) / float64(totalResolved)) * 100
	}

	// Big Book Stats
	var articleCount int64
	var newArticlesCount int64
	database.DB.Model(&models.KnowledgeArticle{}).Count(&articleCount)
	database.DB.Model(&models.KnowledgeArticle{}).Where("is_verified = ?", false).Count(&newArticlesCount) // Pending

	// 2. Pending Articles for Verification
	var pendingArticles []models.KnowledgeArticle
	database.DB.Preload("Author").Where("is_verified = ?", false).Find(&pendingArticles)

	// 3. Upcoming Schedule (Next 7 Days)
	type ShiftView struct {
		DateStr     string // "Mon, 02 Jan"
		TimeStr     string // "07:00 - 15:00"
		StaffName   string
		Label       string // "Shift Pagi", "WFH"
		AvatarURL   string
		StatusClass string // For styling
	}
	var upcomingShifts []ShiftView
	
	var dbShifts []models.Shift
	now := time.Now()
	nextWeek := now.Add(7 * 24 * time.Hour) // Show next 7 days
	
	// Fetch shifts starting from today (start of day to capture current) until next week
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	
	database.DB.Preload("User").
		Where("start_time >= ? AND start_time <= ?", startOfDay, nextWeek).
		Order("start_time asc").
		Find(&dbShifts)
	
	for _, shift := range dbShifts {
		// Determine styling based on Label or Time
		statusClass := "bg-blue-100 text-blue-600" // Default On Duty
		if strings.Contains(strings.ToUpper(shift.Label), "WFH") {
			statusClass = "bg-purple-100 text-purple-600"
		} else if strings.Contains(strings.ToUpper(shift.Label), "OFF") {
			statusClass = "bg-slate-100 text-slate-500"
		}
		
		upcomingShifts = append(upcomingShifts, ShiftView{
			DateStr:     shift.StartTime.Format("Mon, 02 Jan"),
			TimeStr:     shift.StartTime.Format("15:04") + " - " + shift.EndTime.Format("15:04"),
			StaffName:   shift.User.FullName,
			Label:       shift.Label,
			AvatarURL:   shift.User.AvatarURL,
			StatusClass: statusClass,
		})
	}

	// 4. ACTIVE SHIFT (Currently On Duty)
	var activeShift models.Shift
	var hasActiveShift bool
	if err := database.DB.Preload("User").
		Where("start_time <= ? AND end_time >= ?", now, now).
		First(&activeShift).Error; err == nil {
		hasActiveShift = true
	}

	// 5. PUBLISHED ARTICLES (Top 5 by views)
	var publishedArticles []models.KnowledgeArticle
	database.DB.Where("is_verified = ?", true).Order("views_count desc").Limit(5).Find(&publishedArticles)

	// 6. STAFF PERFORMANCE (Aggregation)
	type StaffStat struct {
		StaffName     string
		AvatarURL     string
		Role          string
		TicketsSolved int64
		MTTA          string
		MTTR          string
		BigBookContrib int64
		Rating        float64
	}
	var staffPerformance []StaffStat
	
	var staffUsers []models.User
	database.DB.Where("role = ?", models.RoleStaff).Find(&staffUsers)

	for _, user := range staffUsers {
		// Tickets Solved (via Activity or Assignee if available. Using Activity 'RESOLVED' for now)
		var solvedCount int64
		database.DB.Model(&models.TicketActivity{}).
			Where("actor_id = ? AND action_type = ?", user.ID, "RESOLVED").
			Count(&solvedCount)
		
		// Big Book Contributions
		var bbCount int64
		database.DB.Model(&models.KnowledgeArticle{}).
			Where("author_id = ?", user.ID).
			Count(&bbCount)

		staffPerformance = append(staffPerformance, StaffStat{
			StaffName:     user.FullName,
			AvatarURL:     user.AvatarURL,
			Role:          "IT Support", // Static for now
			TicketsSolved: solvedCount,
			MTTA:          "N/A", // Complex calc skipped for MVP
			MTTR:          "N/A", // Complex calc skipped for MVP
			BigBookContrib: bbCount,
			Rating:        4.5, // Mock rating
		})
	}
	
	// 7. ALL STAFF (For New Shift Modal Dropdown)
	var allStaff []models.User
	database.DB.Where("role = ?", models.RoleStaff).Find(&allStaff)

	c.HTML(http.StatusOK, "manager/dashboard.html", gin.H{
		"title": "Manager Dashboard",
		"mtta": int(mtta),
		"mttr": int(mttr),
		"fcr": int(fcrRate),
		"articleCount": articleCount,
		"pendingArticles": pendingArticles,
		"newArticlesCount": newArticlesCount,
		"upcomingShifts": upcomingShifts,
		"activeShift": activeShift,
		"hasActiveShift": hasActiveShift,
		"publishedArticles": publishedArticles,
		"staffPerformance": staffPerformance,
		"allStaff": allStaff,
	})
}

func ImportSchedule(c *gin.Context) {
	// 1. Get File
	file, err := c.FormFile("schedule")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot open file"})
		return
	}
	defer f.Close()

	// 2. Parse CSV
	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CSV format"})
		return
	}

	// 3. Process Records
	// Format: email, label, start_time, end_time
	// Time Format: 2006-01-02 15:04
	layout := "2006-01-02 15:04"
	var successCount int

	for i, record := range records {
		if i == 0 { continue } // Skip header
		if len(record) < 4 { continue }

		email := strings.TrimSpace(record[0])
		label := strings.TrimSpace(record[1])
		startStr := strings.TrimSpace(record[2])
		endStr := strings.TrimSpace(record[3])

		// Find User
		var user models.User
		if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
			// Skip if user not found
			continue
		}

		startTime, err1 := time.Parse(layout, startStr)
		endTime, err2 := time.Parse(layout, endStr)
		if err1 != nil || err2 != nil {
			continue
		}

		// Create Shift
		shift := models.Shift{
			UserID:    user.ID,
			StartTime: startTime,
			EndTime:   endTime,
			Label:     label,
		}
		database.DB.Create(&shift)
		successCount++
	}

	c.Redirect(http.StatusFound, "/manager")
}

func VerifyArticle(c *gin.Context) {
	id := c.Param("id")
	database.DB.Model(&models.KnowledgeArticle{}).Where("id = ?", id).Update("is_verified", true)
	c.Redirect(http.StatusFound, "/manager")
}
