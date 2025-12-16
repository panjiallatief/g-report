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
	"strconv"
	"fmt"
	"log"
	"encoding/json"
	"it-broadcast-ops/internal/auth"
)

func RegisterRoutes(r *gin.Engine) {
	managerGroup := r.Group("/manager")
	managerGroup.Use(auth.AuthRequired(), auth.RoleRequired(models.RoleManager))
	{
		managerGroup.GET("", Dashboard)
		
		// Reports & Templates (NEW)
		managerGroup.GET("/reports/export", ExportReport)
		managerGroup.GET("/shifts/template", DownloadShiftTemplate)

		// Shift Routes
		managerGroup.POST("/shifts/create", CreateShift)
		managerGroup.POST("/shifts/import", ImportSchedule)
		
		// Routine Routes
		managerGroup.POST("/routines/create", CreateRoutine)
		managerGroup.POST("/routines/:id/delete", DeleteRoutine)
		managerGroup.POST("/routines/:id/toggle-active", ToggleRoutine)

		// Big Book Routes
		managerGroup.GET("/articles/:id/json", GetArticleJSON) 
		managerGroup.POST("/articles/create", CreateArticle)
		managerGroup.POST("/articles/:id/verify", VerifyArticle)
		managerGroup.POST("/articles/:id/deny", DenyArticle)
		managerGroup.POST("/articles/:id/update", UpdateArticle)
		managerGroup.POST("/articles/:id/delete", DeleteArticle)
		
		// Ticket Conversion
		managerGroup.POST("/tickets/:id/convert", ConvertTicketToArticle)
		managerGroup.POST("/tickets/:id/deny", DenyTicket) 
	}
}
// [NEW] Handler untuk Download Report Tiket ke CSV (Excel Compatible)
func ExportReport(c *gin.Context) {
	// 1. Set Header agar browser menganggap ini file download
	filename := fmt.Sprintf("operational_report_%s.csv", time.Now().Format("20060102_1504"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	// Add BOM for Excel to recognize UTF-8
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	// 2. Inisialisasi CSV Writer
	writer := csv.NewWriter(c.Writer)
	
	// 3. Tulis Header CSV
	// Header ini akan menjadi kolom di Excel
	headers := []string{
		"Ticket No", "Subject", "Category", "Location", 
		"Status", "Priority", "Requester", "Processed By", // Updated: Menampilkan siapa yang resolve/handover
		"Created At", "Resolved At", "Response Time (Mins)", "Duration (Mins)", "Solution",
	}
	if err := writer.Write(headers); err != nil {
		c.JSON(500, gin.H{"error": "Failed to write header"})
		return
	}

	// 4. Query Data Tiket (Bisa ditambah filter by date query param jika perlu)
	var tickets []models.Ticket
	database.DB.Preload("Requester").
		Order("created_at desc").
		Find(&tickets)

	// 5. Loop dan Tulis Baris Data
	for _, t := range tickets {
		// Format Waktu
		createdAt := t.CreatedAt.Format("2006-01-02 15:04")
		resolvedAt := "-"
		duration := "0"
		responseTime := "0"

		if t.ResolvedAt != nil {
			resolvedAt = t.ResolvedAt.Format("2006-01-02 15:04")
			dur := t.ResolvedAt.Sub(t.CreatedAt).Minutes()
			duration = fmt.Sprintf("%.0f", dur)
		}

		if t.FirstResponseAt != nil {
			res := t.FirstResponseAt.Sub(t.CreatedAt).Minutes()
			responseTime = fmt.Sprintf("%.0f", res)
		}

		// LOGIKA PENCARIAN PROCESSED BY
		processedBy := "-"
		
		// Skenario 1: Tiket Selesai (Cari Resolver)
		if t.Status == models.StatusResolved || t.Status == models.StatusClosed {
			var act models.TicketActivity
			// Cari aktivitas RESOLVE terakhir untuk tiket ini
			if err := database.DB.Preload("Actor").
				Where("ticket_id = ? AND action_type = ?", t.ID, "RESOLVE").
				Order("created_at desc").
				First(&act).Error; err == nil {
				processedBy = act.Actor.FullName
			}
		} else if t.Status == models.StatusHandover {
			// Skenario 2: Tiket Handover (Cari Pengoper)
			var act models.TicketActivity
			// Cari aktivitas HANDOVER terakhir
			if err := database.DB.Preload("Actor").
				Where("ticket_id = ? AND action_type = ?", t.ID, "HANDOVER").
				Order("created_at desc").
				First(&act).Error; err == nil {
				processedBy = "Handover by " + act.Actor.FullName
			}
		}
		// Skenario 3: Open/In-Progress -> processedBy tetap "-"

		record := []string{
			fmt.Sprintf("#%d", t.TicketNumber),
			t.Subject,
			t.Category,
			string(t.Location),
			string(t.Status),
			string(t.Priority),
			t.Requester.FullName,
			processedBy,
			createdAt,
			resolvedAt,
			responseTime,
			duration,
			t.Solution,
		}

		if err := writer.Write(record); err != nil {
			// Log error, but stream might be broken already
			break 
		}
	}

	writer.Flush()
}

// [NEW] Handler untuk Download Template Shift CSV
func DownloadShiftTemplate(c *gin.Context) {
	c.Header("Content-Disposition", "attachment; filename=shift_import_template.csv")
	c.Header("Content-Type", "text/csv")

	writer := csv.NewWriter(c.Writer)
	
	// Header Wajib
	writer.Write([]string{"email", "label", "start_time", "end_time"})
	
	// Contoh Data (Agar user paham formatnya)
	// Format Time: 2006-01-02 15:04 (Sesuai parser di ImportSchedule)
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	writer.Write([]string{"staff@example.com", "Shift Pagi", tomorrow + " 07:00", tomorrow + " 15:00"})
	writer.Write([]string{"staff2@example.com", "Shift Siang", tomorrow + " 14:00", tomorrow + " 22:00"})
	
	writer.Flush()
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

	// [FIX] Menggunakan Timezone Asia/Jakarta agar input 01:00 tetap 01:00 WIB
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fallback jika server tidak punya data timezone (misal: windows/alpine minimal)
		loc = time.FixedZone("WIB", 7*3600) 
	}

	layout := "2006-01-02T15:04" // datetime-local format
	startTime, err1 := time.ParseInLocation(layout, startStr, loc)
	endTime, err2 := time.ParseInLocation(layout, endStr, loc)

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

type ChartData struct {
	Labels   []string `json:"labels"`   // ["Mon", "Tue", "Wed"...]
	Incoming []int64  `json:"incoming"` // [5, 12, 8...]
	Resolved []int64  `json:"resolved"` // [4, 10, 7...]
}

func Dashboard(c *gin.Context) {

	period := c.DefaultQuery("period", "this_month")
	var startDate time.Time
	now := time.Now()

	if period == "last_month" {
		startDate = now.AddDate(0, -1, 0) // Mundur 1 bulan
	} else {
		startDate = now.AddDate(0, 0, -30) // Default 30 hari terakhir
	}
	
	// MTTA
	var mttaPtr *float64
	database.DB.Model(&models.Ticket{}).
		Select("AVG(EXTRACT(EPOCH FROM (first_response_at - created_at))/60)").
		Where("first_response_at IS NOT NULL AND created_at >= ?", startDate).
		Scan(&mttaPtr)
	mtta := 0.0
	if mttaPtr != nil { mtta = *mttaPtr }

	// MTTR
	var mttrPtr *float64
	database.DB.Model(&models.Ticket{}).
		Select("AVG(EXTRACT(EPOCH FROM (resolved_at - created_at))/60)").
		Where("resolved_at IS NOT NULL AND created_at >= ?", startDate).
		Scan(&mttrPtr)
	mttr := 0.0
	if mttrPtr != nil { mttr = *mttrPtr }

	// FCR Calculation
	var totalResolved int64
	var fcrCount int64
	database.DB.Model(&models.Ticket{}).Where("status = ? AND created_at >= ?", models.StatusResolved, startDate).Count(&totalResolved)
	database.DB.Model(&models.Ticket{}).Where("status = ? AND is_handover = ? AND created_at >= ?", models.StatusResolved, false, startDate).Count(&fcrCount)
	
	fcrRate := 0.0
	if totalResolved > 0 {
		fcrRate = (float64(fcrCount) / float64(totalResolved)) * 100
	}

	chartData := ChartData{
		Labels:   make([]string, 7),
		Incoming: make([]int64, 7),
		Resolved: make([]int64, 7),
	}

	for i := 6; i >= 0; i-- {
		dayDate := now.AddDate(0, 0, -i)
		dayStart := time.Date(dayDate.Year(), dayDate.Month(), dayDate.Day(), 0, 0, 0, 0, dayDate.Location())
		dayEnd := dayStart.Add(24 * time.Hour)
		
		idx := 6 - i
		chartData.Labels[idx] = dayDate.Format("Mon") // "Mon", "Tue"

		// Count Incoming
		database.DB.Model(&models.Ticket{}).
			Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
			Count(&chartData.Incoming[idx])

		// Count Resolved
		database.DB.Model(&models.Ticket{}).
			Where("resolved_at >= ? AND resolved_at < ?", dayStart, dayEnd).
			Count(&chartData.Resolved[idx])
	}

	// SLA Compliance Stats
	type SLAMetric struct {
		Priority         string
		TotalTickets     int
		CompliantTickets int
		ComplianceRate   float64
	}
	
    var slaMetrics []interface{}
	// Removed redeclared variables here (allStaff, routineTemplates) to fix error

	// Query for Urgent (15 mins)
	var urgentStats SLAMetric
	database.DB.Raw(`
		SELECT 
			'URGENT_ON_AIR' as priority,
			COUNT(*) as total_tickets,
			COUNT(*) FILTER (WHERE EXTRACT(EPOCH FROM (resolved_at - created_at))/60 <= 15) as compliant_tickets
		FROM tickets 
		WHERE priority = 'URGENT_ON_AIR' AND status IN ('RESOLVED', 'CLOSED')
	`).Scan(&urgentStats)
	if urgentStats.TotalTickets > 0 {
		urgentStats.ComplianceRate = (float64(urgentStats.CompliantTickets) / float64(urgentStats.TotalTickets)) * 100
	}
	slaMetrics = append(slaMetrics, urgentStats)

	// Query for Normal (8 Hours = 480 mins)
	var normalStats SLAMetric
	database.DB.Raw(`
		SELECT 
			'NORMAL' as priority,
			COUNT(*) as total_tickets,
			COUNT(*) FILTER (WHERE EXTRACT(EPOCH FROM (resolved_at - created_at))/60 <= 480) as compliant_tickets
		FROM tickets 
		WHERE priority IN ('NORMAL', 'HIGH') AND status IN ('RESOLVED', 'CLOSED')
	`).Scan(&normalStats)
	if normalStats.TotalTickets > 0 {
		normalStats.ComplianceRate = (float64(normalStats.CompliantTickets) / float64(normalStats.TotalTickets)) * 100
	}
	slaMetrics = append(slaMetrics, normalStats)

	// CATEGORY DISTRIBUTION (Pie Chart Data)
	type CategoryCount struct {
		Category string
		Count    int64
	}
	var categoryDistribution []CategoryCount
	database.DB.Model(&models.Ticket{}).
		Select("category, COUNT(*) as count").
		Where("created_at >= ?", startDate).
		Group("category").
		Scan(&categoryDistribution)

	categoryLabels := []string{}
	categoryCounts := []int64{}
	for _, c := range categoryDistribution {
		label := c.Category
		if label == "" {
			label = "Uncategorized"
		}
		categoryLabels = append(categoryLabels, label)
		categoryCounts = append(categoryCounts, c.Count)
	}

	// PRIORITY DISTRIBUTION (Pie Chart Data)
	type PriorityCount struct {
		Priority string
		Count    int64
	}
	var priorityDistribution []PriorityCount
	database.DB.Model(&models.Ticket{}).
		Select("priority, COUNT(*) as count").
		Where("created_at >= ?", startDate).
		Group("priority").
		Scan(&priorityDistribution)

	priorityLabels := []string{}
	priorityCounts := []int64{}
	for _, p := range priorityDistribution {
		priorityLabels = append(priorityLabels, string(p.Priority))
		priorityCounts = append(priorityCounts, p.Count)
	}

	// STATUS DISTRIBUTION (Pie Chart Data)
	type StatusCount struct {
		Status string
		Count  int64
	}
	var statusDistribution []StatusCount
	database.DB.Model(&models.Ticket{}).
		Select("status, COUNT(*) as count").
		Where("created_at >= ?", startDate).
		Group("status").
		Scan(&statusDistribution)

	statusLabels := []string{}
	statusCounts := []int64{}
	for _, s := range statusDistribution {
		statusLabels = append(statusLabels, string(s.Status))
		statusCounts = append(statusCounts, s.Count)
	}

	// 5. BIG BOOK DATA
	var articleCount, newArticlesCount int64
	database.DB.Model(&models.KnowledgeArticle{}).Count(&articleCount)
	database.DB.Model(&models.KnowledgeArticle{}).Where("is_verified = ?", false).Count(&newArticlesCount)

	// List Pending (Approval Queue)
	var pendingArticles []models.KnowledgeArticle
	database.DB.Preload("Author").Where("is_verified = ?", false).Find(&pendingArticles)

	// List Published
	var publishedArticles []models.KnowledgeArticle
	database.DB.Where("is_verified = ?", true).Order("views_count desc").Find(&publishedArticles)

	// --- [LOGIKA PENTING] CANDIDATE TICKETS ---
	// Ambil tiket yang: 1. Resolved, 2. Punya solusi, 3. BELUM diconvert jadi artikel
	var candidateTickets []models.Ticket
	database.DB.Preload("Requester").
		Where("status = ? AND solution != '' AND is_converted_to_article = ?", models.StatusResolved, false).
		Order("resolved_at desc").
		Limit(5).
		Find(&candidateTickets)
	

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
	// Removed 'now := time.Now()' here to fix no new variables error
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
	nowTime := time.Now()
	// Check strictly active now
	database.DB.Preload("User").
		Where("start_time <= ? AND end_time >= ?", nowTime, nowTime).
		Order("start_time desc").
		First(&activeShift)

	hasActiveShift := activeShift.ID != uuid.Nil

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
		// Tickets Solved - count RESOLVE activities by this user
		var solvedCount int64
		database.DB.Model(&models.TicketActivity{}).
			Where("actor_id = ? AND action_type = ?", user.ID, "RESOLVE").
			Count(&solvedCount)
		
		// Big Book Contributions
		var bbCount int64
		database.DB.Model(&models.KnowledgeArticle{}).
			Where("author_id = ?", user.ID).
			Count(&bbCount)

		// Calculate MTTA for this staff
		// MTTA = Average time from ticket creation to first response by THIS staff
		var mttaMinutes *float64
		database.DB.Raw(`
			SELECT AVG(EXTRACT(EPOCH FROM (ta.created_at - t.created_at))/60) as mtta_minutes
			FROM ticket_activities ta
			JOIN tickets t ON ta.ticket_id = t.id
			WHERE ta.actor_id = ?
			AND ta.action_type IN ('REPLY', 'IN_PROGRESS', 'STATUS_CHANGE')
			AND ta.created_at = (
				SELECT MIN(ta2.created_at) 
				FROM ticket_activities ta2 
				WHERE ta2.ticket_id = t.id 
				AND ta2.actor_id = ?
			)
		`, user.ID, user.ID).Scan(&mttaMinutes)

		mttaStr := "N/A"
		if mttaMinutes != nil && *mttaMinutes > 0 {
			if *mttaMinutes < 60 {
				mttaStr = fmt.Sprintf("%.0fm", *mttaMinutes)
			} else {
				mttaStr = fmt.Sprintf("%.1fh", *mttaMinutes/60)
			}
		}

		// Calculate MTTR for this staff
		// MTTR = Average time from ticket creation to resolution for tickets resolved by THIS staff
		var mttrMinutes *float64
		database.DB.Raw(`
			SELECT AVG(EXTRACT(EPOCH FROM (t.resolved_at - t.created_at))/60) as mttr_minutes
			FROM tickets t
			JOIN ticket_activities ta ON ta.ticket_id = t.id
			WHERE ta.actor_id = ?
			AND ta.action_type = 'RESOLVE'
			AND t.resolved_at IS NOT NULL
		`, user.ID).Scan(&mttrMinutes)

		mttrStr := "N/A"
		if mttrMinutes != nil && *mttrMinutes > 0 {
			if *mttrMinutes < 60 {
				mttrStr = fmt.Sprintf("%.0fm", *mttrMinutes)
			} else {
				mttrStr = fmt.Sprintf("%.1fh", *mttrMinutes/60)
			}
		}

		// Rating calculation - based on performance metrics
		// Simple formula: Base 3.0 + bonuses for good metrics
		rating := 3.0
		if solvedCount > 0 {
			rating += 0.5 // Bonus for having solved tickets
		}
		if solvedCount >= 10 {
			rating += 0.5 // Bonus for solving 10+ tickets
		}
		if bbCount > 0 {
			rating += 0.5 // Bonus for Big Book contributions
		}
		if mttaMinutes != nil && *mttaMinutes < 15 {
			rating += 0.5 // Bonus for fast response
		}
		if rating > 5.0 {
			rating = 5.0 // Cap at 5.0
		}

		staffPerformance = append(staffPerformance, StaffStat{
			StaffName:     user.FullName,
			AvatarURL:     user.AvatarURL,
			Role:          "IT Support",
			TicketsSolved: solvedCount,
			MTTA:          mttaStr,
			MTTR:          mttrStr,
			BigBookContrib: bbCount,
			Rating:        rating,
		})
	}
	
	// 7. ALL STAFF (For New Shift Modal Dropdown)
	var allStaff []models.User
	database.DB.Where("role = ?", models.RoleStaff).Find(&allStaff)

	// 8. ROUTINE TEMPLATES
	var routineTemplates []models.RoutineTemplate
	database.DB.Find(&routineTemplates)

	c.HTML(http.StatusOK, "manager/dashboard.html", gin.H{
		"title":             "Manager Dashboard",
		"mtta":              int(mtta),
		"mttr":              int(mttr),
		"fcr":               int(fcrRate),
		"period":            period, // Kirim balik ke UI untuk set selected option
		"chartData":         chartData, // Data Chart
		// Big Book Data
		"articleCount": articleCount, 
		"newArticlesCount": newArticlesCount,
		"pendingArticles": pendingArticles,
		"publishedArticles": publishedArticles,
		"candidateTickets": candidateTickets, // Data tiket yang belum diconvert
		"upcomingShifts":    upcomingShifts,
		"activeShift":       activeShift,
		"hasActiveShift":    hasActiveShift,
		"staffPerformance":  staffPerformance,
		"allStaff":          allStaff,
		"slaMetrics":        slaMetrics,
		"routineTemplates":  routineTemplates,
		// Pie Chart Data
		"categoryLabels":  categoryLabels,
		"categoryCounts":  categoryCounts,
		"priorityLabels":  priorityLabels,
		"priorityCounts":  priorityCounts,
		"statusLabels":    statusLabels,
		"statusCounts":    statusCounts,
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
	layout := "02/01/2006 15:04"
	var successCount int

	// [FIX] Menggunakan Timezone Asia/Jakarta untuk import juga
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}

	for i, record := range records {
		if i == 0 {
			continue
		} // Skip header
		if len(record) < 4 {
			continue
		}

		email := strings.TrimSpace(record[0])
		label := strings.TrimSpace(record[1])
		startStr := strings.TrimSpace(record[2])
		endStr := strings.TrimSpace(record[3])

		// Find User
		var user models.User
		if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
			// Skip if user not found or error
			log.Printf("Import failed for row %d (%s): User not found", i, email)
			continue
		}

		// [FIX] Pakai ParseInLocation
		startTime, err1 := time.ParseInLocation(layout, startStr, loc)
		endTime, err2 := time.ParseInLocation(layout, endStr, loc)
		if err1 != nil || err2 != nil {
			log.Printf("Import failed for row %d: Invalid Time Format. Start: %v, End: %v", i, err1, err2)
			continue
		}

		// Create Shift
		shift := models.Shift{
			UserID:    user.ID,
			StartTime: startTime,
			EndTime:   endTime,
			Label:     label,
		}
		if err := database.DB.Create(&shift).Error; err != nil {
			log.Printf("Import failed for row %d: DB Error %v", i, err)
		} else {
			successCount++
		}
	}

	log.Printf("Imported %d shifts successfully.", successCount)
	c.Redirect(http.StatusFound, "/manager")
}



// 1. Get Data JSON untuk Modal Edit
func GetArticleJSON(c *gin.Context) {
	id := c.Param("id")
	var article models.KnowledgeArticle
	if err := database.DB.Preload("Author").First(&article, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
		return
	}
	// Return field JSON yang sesuai dengan AlpineJS di frontend
	c.JSON(http.StatusOK, gin.H{
		"id":       article.ID,
		"title":    article.Title,
		"category": article.Category,
		"content":  article.Content,
		"author":   article.Author.FullName,
		"date":     article.CreatedAt.Format("02 Jan 2006"),
	})
}

// 2. Convert Ticket -> Article
func ConvertTicketToArticle(c *gin.Context) {
	ticketID := c.Param("id")
	var ticket models.Ticket
	if err := database.DB.First(&ticket, "id = ?", ticketID).Error; err != nil {
		c.Redirect(http.StatusFound, "/manager?error=TicketNotFound")
		return
	}

	userIDStr, _ := c.Cookie("user_id")
	managerID, _ := uuid.Parse(userIDStr)

	// Buat Artikel Baru
	article := models.KnowledgeArticle{
		Title:      ticket.Subject,
		Category:   ticket.Category,
		Content:    ticket.Solution, // Solusi tiket otomatis jadi konten artikel
		AuthorID:   managerID,       // Di-publish oleh Manager
		IsVerified: true,            // Langsung verified karena masuk jalur cepat
		CreatedAt:  time.Now(),
	}

	if err := database.DB.Create(&article).Error; err != nil {
		// handle error
	} else {
		// PENTING: Tandai tiket sudah dikonversi agar hilang dari list kandidat
		database.DB.Model(&ticket).Update("is_converted_to_article", true)
	}

	c.Redirect(http.StatusFound, "/manager")
}
// 3. Create Manual Article
func CreateArticle(c *gin.Context) {
	title := c.PostForm("title")
	category := c.PostForm("category")
	content := c.PostForm("content")
	userIDStr, _ := c.Cookie("user_id")
	userID, _ := uuid.Parse(userIDStr)

	article := models.KnowledgeArticle{
		Title:      title,
		Category:   category,
		Content:    content,
		AuthorID:   userID,
		IsVerified: true, // Artikel buatan manager langsung publish
		CreatedAt:  time.Now(),
	}
	database.DB.Create(&article)
	c.Redirect(http.StatusFound, "/manager")
}

// 4. Update Article
func UpdateArticle(c *gin.Context) {
	id := c.Param("id")
	var article models.KnowledgeArticle
	if err := database.DB.First(&article, "id = ?", id).Error; err == nil {
		article.Title = c.PostForm("title")
		article.Category = c.PostForm("category")
		article.Content = c.PostForm("content")
		database.DB.Save(&article)
	}
	c.Redirect(http.StatusFound, "/manager")
}

// 5. Verify (Approve) Article from Staff
func VerifyArticle(c *gin.Context) {
	id := c.Param("id")
	database.DB.Model(&models.KnowledgeArticle{}).Where("id = ?", id).Update("is_verified", true)
	c.Redirect(http.StatusFound, "/manager")
}


// DenyArticle: Menolak artikel yang masih draft/pending. 
// Logikanya adalah menghapus artikel tersebut dari tabel knowledge_articles.
func DenyArticle(c *gin.Context) {
	id := c.Param("id")
	// Hapus artikel berdasarkan ID
	if err := database.DB.Delete(&models.KnowledgeArticle{}, "id = ?", id).Error; err != nil {
		// Anda bisa menambahkan logging error di sini
		c.Redirect(http.StatusFound, "/manager?error=DeleteFailed")
		return
	}
	c.Redirect(http.StatusFound, "/manager")
}

func DenyTicket(c *gin.Context) {
	id := c.Param("id")
	
	// Update flag is_converted_to_article menjadi true agar tidak muncul lagi di dashboard
	// Kita anggap "Deny" pada tiket kandidat berarti "Abaikan saran ini"
	if err := database.DB.Model(&models.Ticket{}).Where("id = ?", id).Update("is_converted_to_article", true).Error; err != nil {
		c.Redirect(http.StatusFound, "/manager?error=UpdateFailed")
		return
	}

	c.Redirect(http.StatusFound, "/manager")
}


// DeleteArticle: Menghapus artikel yang sudah dipublikasikan.
// Logikanya sama dengan Deny, yaitu menghapus dari database.
func DeleteArticle(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&models.KnowledgeArticle{}, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/manager?error=DeleteFailed")
		return
	}
	c.Redirect(http.StatusFound, "/manager")
}


func CreateRoutine(c *gin.Context) {
	title := c.PostForm("title")
	deadlineStr := c.PostForm("deadline_minutes")
	
	// Cron Construction Inputs
	freq := c.PostForm("frequency") // DAILY, WEEKLY, MONTHLY
	timeStr := c.PostForm("time")   // HH:MM
	dayWeek := c.PostForm("day_week") // 0-6
	dayMonth := c.PostForm("day_month") // 1-31

	// Checklist
	checklistItems := c.PostFormArray("checklist_items")

	// Get User (Manager)
	userIDStr, err := c.Cookie("user_id")
	if err != nil {
		c.Redirect(http.StatusFound, "/auth/login")
		return
	}

	if title == "" || freq == "" || timeStr == "" {
		c.Redirect(http.StatusFound, "/manager?error=MissingFields")
		return
	}

	// 1. Construct Cron
	// Format: Minute Hour Day Month DayOfWeek
	// Time: HH:MM
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		c.Redirect(http.StatusFound, "/manager?error=InvalidTime")
		return
	}
	hour := parts[0]
	minute := parts[1]

	var cron string
	switch freq {
	case "DAILY":
		cron = fmt.Sprintf("%s %s * * *", minute, hour)
	case "WEEKLY":
		if dayWeek == "" { dayWeek = "1" } // Default Monday
		cron = fmt.Sprintf("%s %s * * %s", minute, hour, dayWeek)
	case "MONTHLY":
		if dayMonth == "" { dayMonth = "1" }
		cron = fmt.Sprintf("%s %s %s * *", minute, hour, dayMonth)
	default:
		cron = fmt.Sprintf("%s %s * * *", minute, hour)
	}

	deadline := 30
	if d, err := strconv.Atoi(deadlineStr); err == nil {
		deadline = d
	}

	// 2. Marshal Checklist
	checklistJSON, _ := json.Marshal(checklistItems)

	tpl := models.RoutineTemplate{
		Title:           title,
		CronSchedule:    cron,
		DeadlineMinutes: deadline,
		IsActive:        true,
		CreatedBy:       uuid.MustParse(userIDStr),
		ChecklistItems:  checklistJSON,
	}
	
	database.DB.Create(&tpl)
	c.Redirect(http.StatusFound, "/manager")
}


// NEW: DeleteRoutine
func DeleteRoutine(c *gin.Context) {
	id := c.Param("id")
	// Only delete the template, or soft delete if you prefer
	database.DB.Delete(&models.RoutineTemplate{}, "id = ?", id)
	c.Redirect(http.StatusFound, "/manager")
}

// NEW: ToggleRoutine
func ToggleRoutine(c *gin.Context) {
	id := c.Param("id")
	// Toggle the active state
	parsedID, err := uuid.Parse(id)
	if err != nil {
		c.Redirect(http.StatusFound, "/manager?error=InvalidID")
		return
	}

	var routine models.RoutineTemplate
	if err := database.DB.Select("is_active").Where("id = ?", parsedID).First(&routine).Error; err != nil {
		c.Redirect(http.StatusFound, "/manager?error=RoutineNotFound")
		return
	}

	// Toggle the active state
	newActiveState := !routine.IsActive
	database.DB.Model(&models.RoutineTemplate{}).Where("id = ?", parsedID).Update("is_active", newActiveState)
	c.Redirect(http.StatusFound, "/manager")
}
