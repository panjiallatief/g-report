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
	"it-broadcast-ops/internal/auth"
	"it-broadcast-ops/internal/modules/consumer"
	"it-broadcast-ops/internal/modules/manager"
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
	return render.HTML{
		Template: tmpl,
		Name:     "base.html", 
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

	r.GET("/seed", func(c *gin.Context) {
		utils.SeedDatabase(c)
	})

	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "/auth/login")
	})

	return r
}


func loadTemplates() MultiRender {
	templates := make(map[string]*template.Template)
	
	basePath := "web/templates/layouts/base.html"
	pagesDir := "web/templates/pages"
	
	// Walk pages
	err := filepath.Walk(pagesDir, func(path string, infoTw os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if infoTw.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".html") {
			return nil
		}

		// Create a Name: e.g. "auth/login.html"
		rel, err := filepath.Rel(pagesDir, path)
		if err != nil {
			return err
		}
		// Normalize slashes for map keys
		name := filepath.ToSlash(rel) 
		
		tmpl := template.New("base.html").Funcs(template.FuncMap{
			// Add any custom funcs here if needed
		})
		
		// Parse Base
		tmpl, err = tmpl.ParseFiles(basePath)
		if err != nil {
			panic("Failed to parse base: " + err.Error())
		}
		
		// Parse Page (it will overlay "content" define)
		_, err = tmpl.ParseFiles(path)
		if err != nil {
			panic("Failed to parse page " + path + ": " + err.Error())
		}
		
		// Map keys: "auth/login.html"
		templates[name] = tmpl
		
		// Also map "pages/auth/login.html" just in case user code used that
		templates["pages/"+name] = tmpl
		
		fmt.Printf("Loaded template: %s\n", name)
		return nil
	})
	
	if err != nil {
		panic("Failed to walk templates: " + err.Error())
	}
	
	return MultiRender{Templates: templates}
}