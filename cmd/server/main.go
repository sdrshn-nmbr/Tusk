package main

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/google"
	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"github.com/sdrshn-nmbr/tusk/internal/handlers"
	"github.com/sdrshn-nmbr/tusk/internal/middleware"
	"github.com/sdrshn-nmbr/tusk/internal/storage"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// load all secrets from config
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config not initialized properly: %v", err)
	}

	// Initialize MongoDB storage
	ms, err := storage.NewMongoStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB storage: %v", err)
	}

	// Run migration
	if err := ms.MigrateMissingFileSizes(); err != nil {
		log.Printf("Error migrating file sizes: %v", err)
	}

	// Initialize embedder
	embedder := ai.NewEmbedder(cfg)

	// Parse templates
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}
	tmpl, err := template.ParseGlob(filepath.Join(wd, "web", "templates", "*.html"))

	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
	log.Printf("Templates parsed successfully. Number of templates: %d", len(tmpl.Templates()))
	for _, t := range tmpl.Templates() {
		log.Printf("Template name: %s", t.Name())
	}

	// Initialize handler with MongoDB storage and embedder
	h := handlers.NewHandler(ms, embedder, tmpl)

	// Set up Gin router
	r := gin.Default()
	r.MaxMultipartMemory = 32 << 20 // 32 MiB

	// Load HTML templates
	r.SetHTMLTemplate(tmpl)
	log.Println("HTML templates loaded into Gin")

	// Set up Goth for authentication
	key := "your-secret-key" // Replace with a secure secret key
	maxAge := 86400 * 30     // 30 days
	isProd := false          // Set to true in production
	store := sessions.NewCookieStore([]byte(key))
	store.MaxAge(maxAge)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = isProd
	gothic.Store = store

	goth.UseProviders(
		google.New(cfg.GoogleClientID, cfg.GoogleClientSecret, "http://localhost:8080/auth/google/callback"),
		github.New(cfg.GithubClientID, cfg.GithubClientSecret, "http://localhost:8080/auth/github/callback"),
	)

	// Update the root route
	r.GET("/", middleware.AuthRequired(), func(c *gin.Context) {
		h.Index(c)
	})

	// Update the login route
	r.GET("/login", func(c *gin.Context) {
		session, _ := gothic.Store.Get(c.Request, "user-session")
		if session.Values["user_id"] != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		h.Login(c)
	})

	// Auth routes
	r.GET("/auth/:provider", h.BeginAuth)
	r.GET("/auth/:provider/callback", h.CompleteAuth)
	r.GET("/logout", h.Logout) // Add this line

	// Use middleware for protected routes
	r.POST("/upload", middleware.AuthRequired(), h.UploadFile)
	r.POST("/delete", middleware.AuthRequired(), h.DeleteFile)
	r.GET("/files", middleware.AuthRequired(), h.GetFileList)
	r.GET("/download", middleware.AuthRequired(), h.DownloadFile)
	r.GET("/generate-search", middleware.AuthRequired(), h.GenerateSearch)

	// Serve static files
	r.Static("/static", "./web/static")

	log.Println("Server starting on :8080")
	log.Fatal(r.Run(":8080"))
}
