package main

import (
	"fmt"
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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting application...")

	// Log current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current directory: %v", err)
	} else {
		log.Printf("Current working directory: %s", currentDir)
	}

	// load all secrets from config
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Config not initialized properly: %v", err)
	}
	log.Println("Config loaded successfully")

	// Initialize MongoDB storage
	ms, err := storage.NewMongoStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB storage: %v", err)
	}
	log.Println("MongoDB storage initialized")

	// Run migration
	if err := ms.MigrateMissingFileSizes(); err != nil {
		log.Printf("Error migrating file sizes: %v", err)
	} else {
		log.Println("File size migration completed successfully")
	}

	// Initialize embedder
	embedder := ai.NewEmbedder(cfg)
	log.Println("Embedder initialized")

	// Parse templates
	templatesDir := filepath.Join(currentDir, "web", "templates")
	log.Printf("Looking for templates in: %s", templatesDir)

	// List contents of templates directory
	files, err := ioutil.ReadDir(templatesDir)
	if err != nil {
		log.Printf("Error reading templates directory: %v", err)
	} else {
		log.Println("Files in templates directory:")
		for _, file := range files {
			log.Printf("- %s", file.Name())
		}
	}

	tmpl, err := template.ParseGlob(filepath.Join(templatesDir, "*.html"))
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}
	log.Printf("Templates parsed successfully. Number of templates: %d", len(tmpl.Templates()))
	for _, t := range tmpl.Templates() {
		log.Printf("Template name: %s", t.Name())
	}

	// Initialize handler with MongoDB storage and embedder
	h := handlers.NewHandler(ms, embedder, tmpl)
	log.Println("Handler initialized")

	// Set up Gin router
	r := gin.Default()
	r.MaxMultipartMemory = 32 << 20 // 32 MiB
	log.Println("Gin router set up")

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
	log.Println("Goth authentication set up")

	goth.UseProviders(
		google.New(cfg.GoogleClientID, cfg.GoogleClientSecret, "http://localhost:8080/auth/google/callback"),
		github.New(cfg.GithubClientID, cfg.GithubClientSecret, "http://localhost:8080/auth/github/callback"),
	)
	log.Println("Goth providers configured")

	// Set up routes
	r.GET("/", middleware.AuthRequired(), func(c *gin.Context) {
		h.Index(c)
	})
	r.GET("/login", func(c *gin.Context) {
		session, _ := gothic.Store.Get(c.Request, "user-session")
		if session.Values["user_id"] != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		h.Login(c)
	})
	r.GET("/auth/:provider", h.BeginAuth)
	r.GET("/auth/:provider/callback", h.CompleteAuth)
	r.GET("/logout", h.Logout)
	r.POST("/upload", middleware.AuthRequired(), h.UploadFile)
	r.POST("/delete", middleware.AuthRequired(), h.DeleteFile)
	r.GET("/files", middleware.AuthRequired(), h.GetFileList)
	r.GET("/download", middleware.AuthRequired(), h.DownloadFile)
	r.GET("/generate-search", middleware.AuthRequired(), h.GenerateSearch)
	log.Println("Routes configured")

	// Serve static files
	staticDir := filepath.Join(currentDir, "web", "static")
	log.Printf("Serving static files from: %s", staticDir)
	r.Static("/static", staticDir)

	// List contents of static directory
	staticFiles, err := ioutil.ReadDir(staticDir)
	if err != nil {
		log.Printf("Error reading static directory: %v", err)
	} else {
		log.Println("Files in static directory:")
		for _, file := range staticFiles {
			log.Printf("- %s", file.Name())
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	log.Fatal(r.Run(fmt.Sprintf(":%s", port)))
}
