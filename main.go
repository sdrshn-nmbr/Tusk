package main

import (
	"embed"
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
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed web/templates/*
var templateFS embed.FS

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

	// Parse templates using embedded file system
	tmpl, err := parseTemplates()
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
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

	// Update callback URLs for production
	callbackURL := "https://" + os.Getenv("FLY_APP_NAME") + ".fly.dev/auth/%s/callback"
	goth.UseProviders(
		google.New(cfg.GoogleClientID, cfg.GoogleClientSecret, fmt.Sprintf(callbackURL, "google")),
		github.New(cfg.GithubClientID, cfg.GithubClientSecret, fmt.Sprintf(callbackURL, "github")),
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

	// Use PORT environment variable provided by Fly.io
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	// log.Fatal(r.Run(":" + port))
	log.Fatal(r.Run("0.0.0.0:" + port))
}

func parseTemplates() (*template.Template, error) {
    tmpl := template.New("")
    err := fs.WalkDir(templateFS, "web/templates", func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        if d.IsDir() {
            return nil
        }
        if filepath.Ext(path) != ".html" {
            return nil
        }
        b, err := templateFS.ReadFile(path)
        if err != nil {
            return err
        }
        name := filepath.ToSlash(path[len("web/templates/"):])
        _, err = tmpl.New(name).Parse(string(b))
        return err
    })
    return tmpl, err
}
