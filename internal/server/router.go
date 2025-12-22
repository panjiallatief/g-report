package server

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/modules/consumer"
	"it-broadcast-ops/internal/modules/manager"
	"it-broadcast-ops/internal/modules/public"
	"it-broadcast-ops/internal/modules/staff"
	"it-broadcast-ops/internal/modules/notification"
	"it-broadcast-ops/internal/utils"
)

// Custom Render to support multiple distinct page templates inheriting from base
type MultiRender struct {
	Templates map[string]*template.Template
}


func (r MultiRender) Instance(name string, data interface{}) render.Render {
	tmpl, ok := r.Templates[name]
	if !ok {
		panic("Template not found: " + name)
	}
	
	target := "base.html"
	// [FIX] Logika Partial yang Aman Cross-Platform (Windows/Linux)
	// Karena 'name' sudah dipaksa pakai "/" (ToSlash), kita parse manual string-nya
	// agar tidak tergantung path separator OS server.
	if strings.Contains(name, "_partial") {
		if idx := strings.LastIndex(name, "/"); idx != -1 {
			target = name[idx+1:] // Ambil nama file setelah slash terakhir
		} else {
			target = name
		}
	}

	return render.HTML{
		Template: tmpl,
		Name:     target, 
		Data:     data,
	}
}

func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.Static("/static", "./web/static")
	r.Static("/uploads", "./web/uploads")
    
    // [FIX] Serve Service Worker di Root Path agar Scope-nya global (mencakup /staff, /consumer, dll)
	r.StaticFile("/sw.js", "./web/static/sw.js")

	r.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusNotFound, "pages/error/404.html", gin.H{
			"title": "Page Not Found",
		})
	})

	renderer := loadTemplates()
	r.HTMLRender = renderer

	// Routes
	auth.RegisterRoutes(r)
	consumer.RegisterRoutes(r)
	staff.RegisterRoutes(r)
	manager.RegisterRoutes(r)
	notification.RegisterRoutes(r)
	public.RegisterRoutes(r) // Public emergency form (no auth) 

	r.GET("/seed", func(c *gin.Context) {
		utils.SeedDatabase(c)
	})

	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/auth/login")
	})

	// Swagger API Documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return r
}

func loadTemplates() MultiRender {
	templates := make(map[string]*template.Template)
	
	basePath := "web/templates/layouts/base.html"
	pagesDir := "web/templates/pages"
	
	err := filepath.Walk(pagesDir, func(path string, infoTw os.FileInfo, err error) error {
		if err != nil { return err }
		if infoTw.IsDir() { return nil }
		if !strings.HasSuffix(path, ".html") { return nil }

		rel, err := filepath.Rel(pagesDir, path)
		if err != nil { return err }
		name := filepath.ToSlash(rel) 
		
		isPartial := strings.Contains(name, "_partial")
		
		var tmpl *template.Template
		
		if isPartial {
			// Template name = filename (e.g., "ticket_list_partial.html")
			tmpl = template.New(filepath.Base(path)).Funcs(template.FuncMap{})
		} else {
			tmpl = template.New("base.html").Funcs(template.FuncMap{})
			tmpl.ParseFiles(basePath)
		}
		
		_, err = tmpl.ParseFiles(path)
		if err != nil {
			panic("Failed to parse page " + path + ": " + err.Error())
		}
		
		templates[name] = tmpl
		templates["pages/"+name] = tmpl // Alias
		
		fmt.Printf("Loaded template: %s (Partial: %v)\n", name, isPartial)
		return nil
	})
	
	if err != nil {
		panic("Failed to walk templates: " + err.Error())
	}
	
	return MultiRender{Templates: templates}
}