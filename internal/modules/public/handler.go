package public

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"it-broadcast-ops/internal/database"
	"it-broadcast-ops/internal/models"
	"it-broadcast-ops/internal/notification"
	redisClient "it-broadcast-ops/internal/redis"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Rate limit: max tickets per hour per IP
const maxTicketsPerHour = 3

func RegisterRoutes(r *gin.Engine) {
	r.GET("/report", ShowReportForm)
	r.POST("/report", SubmitReport)
	r.GET("/report/success", ShowSuccess)
	r.GET("/report/qrcode", ShowQRCode)
	r.GET("/report/history/:token", ShowTicketHistory)
}

// ShowReportForm displays the public ticket submission form
func ShowReportForm(c *gin.Context) {
	c.HTML(http.StatusOK, "public/report.html", gin.H{
		"title": "Lapor Masalah - Quick Report",
	})
}

// SubmitReport handles anonymous ticket submission with rate limiting
func SubmitReport(c *gin.Context) {
	clientIP := c.ClientIP()

	// Rate limiting check (using Redis)
	if redisClient.IsConnected() {
		rateLimitKey := "ratelimit:report:" + clientIP
		var count int
		if err := redisClient.Get(rateLimitKey, &count); err == nil && count >= maxTicketsPerHour {
			c.HTML(http.StatusTooManyRequests, "public/report.html", gin.H{
				"title": "Lapor Masalah",
				"error": "Terlalu banyak laporan dari IP ini. Coba lagi dalam 1 jam.",
			})
			return
		}
	}

	// Get form data and trim whitespace
	name := strings.TrimSpace(c.PostForm("name"))
	email := strings.TrimSpace(c.PostForm("email"))
	phone := strings.TrimSpace(c.PostForm("phone"))
	location := c.PostForm("location")
	urgency := c.PostForm("urgency")
	subject := strings.TrimSpace(c.PostForm("subject"))
	description := strings.TrimSpace(c.PostForm("description"))

	// Debug logging
	log.Printf("[Public Report] Received: name=%s, email=%s, phone=%s, subject=%s, desc_len=%d", 
		name, email, phone, subject, len(description))

	// Validation
	errors := validateReportForm(name, email, phone, subject, description)
	if len(errors) > 0 {
		log.Printf("[Public Report] Validation errors: %v", errors)
		c.HTML(http.StatusBadRequest, "public/report.html", gin.H{
			"title":  "Lapor Masalah",
			"errors": errors,
			"form": gin.H{
				"name": name, "email": email, "phone": phone,
				"location": location, "urgency": urgency,
				"subject": subject, "description": description,
			},
		})
		return
	}

	// Handle file upload
	var proofURL string
	file, err := c.FormFile("proof_image")
	if err == nil && file.Size <= 5*1024*1024 {
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("public_%d%s", time.Now().UnixNano(), ext)
		saveDir := "web/uploads/tickets"
		os.MkdirAll(saveDir, 0755)
		savePath := filepath.Join(saveDir, filename)
		if err := c.SaveUploadedFile(file, savePath); err == nil {
			proofURL = "/uploads/tickets/" + filename
		}
	}

	// Find or create guest user by email
	var guestUser models.User
	result := database.DB.Where("email = ?", email).First(&guestUser)
	if result.Error != nil {
		// Create new guest user
		guestUser = models.User{
			Email:        email,
			FullName:     name,
			Role:         models.RoleConsumer,
			PasswordHash: "", // No password for guest - they use email link
		}
		database.DB.Create(&guestUser)
	}

	// Generate history access token
	historyToken := generateToken()

	// Create ticket
	priority := models.PriorityNormal
	if urgency == "ON_AIR_EMERGENCY" {
		priority = models.PriorityUrgentOnAir
	}

	ticket := models.Ticket{
		Location:      models.LocationEnum(location),
		Priority:      priority,
		Category:      "IT_NETWORK", // Default for quick reports
		Subject:       subject,
		Description:   description + "\n\n---\nReported by: " + name + "\nPhone: " + phone + "\nEmail: " + email,
		ProofImageURL: proofURL,
		RequesterID:   guestUser.ID,
		Status:        models.StatusOpen,
		CreatedAt:     time.Now(),
	}
	database.DB.Create(&ticket)

	// Increment rate limit counter
	if redisClient.IsConnected() {
		rateLimitKey := "ratelimit:report:" + clientIP
		var count int
		redisClient.Get(rateLimitKey, &count)
		redisClient.Set(rateLimitKey, count+1, time.Hour)
	}

	// Send notification to staff
	if priority == models.PriorityUrgentOnAir {
		go notification.SendBroadcastToStaff(
			"🔥 URGENT PUBLIC: "+string(ticket.Location),
			ticket.Subject+" (QR EMERGENCY)",
			"/staff/tickets/"+ticket.ID.String(),
		)
	} else {
		go notification.SendBroadcastToStaff(
			"📱 Public Report: "+string(ticket.Location),
			ticket.Subject,
			"/staff/tickets/"+ticket.ID.String(),
		)
	}

	log.Printf("[Public Report] New ticket from %s (%s) - %s", name, email, subject)

	// Redirect to success page with token
	c.Redirect(http.StatusFound, "/report/success?token="+historyToken+"&email="+email)
}

// ShowSuccess displays confirmation after submission
func ShowSuccess(c *gin.Context) {
	email := c.Query("email")
	c.HTML(http.StatusOK, "public/success.html", gin.H{
		"title": "Laporan Terkirim",
		"email": email,
	})
}

// ShowQRCode displays QR code for printing
func ShowQRCode(c *gin.Context) {
	// Get base URL from request
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + c.Request.Host + "/report"

	c.HTML(http.StatusOK, "public/qrcode.html", gin.H{
		"title":   "QR Code - Quick Report",
		"formURL": baseURL,
	})
}

// ShowTicketHistory shows tickets for a user (via email link)
func ShowTicketHistory(c *gin.Context) {
	token := c.Param("token")
	// For now, just redirect to login - full implementation needs token storage
	c.Redirect(http.StatusFound, "/auth/login?msg=Please+login+to+view+history&token="+token)
}

// Helper functions
func validateReportForm(name, email, phone, subject, description string) []string {
	var errors []string

	if len(name) < 2 {
		errors = append(errors, "Nama minimal 2 karakter")
	}
	if !isValidEmail(email) {
		errors = append(errors, "Format email tidak valid")
	}
	if len(phone) < 10 || len(phone) > 15 {
		errors = append(errors, "Nomor telepon harus 10-15 digit")
	}
	if len(subject) < 5 {
		errors = append(errors, "Subjek minimal 5 karakter")
	}
	if len(description) < 10 {
		errors = append(errors, "Deskripsi minimal 10 karakter")
	}

	return errors
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func generateToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
