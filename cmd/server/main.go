package main

import (
	"html/template"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"github.com/sdrshn-nmbr/tusk/internal/handlers"
	"github.com/sdrshn-nmbr/tusk/internal/storage"
)

func main() {
	// load all secrets from config
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal("Config not initialized properly")
	}

	// Initialize MongoDB storage
	ms, err := storage.NewMongoStorage(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MongoDB storage: %+v", err)
	}

	// Initialize embedder
	embedder := ai.NewEmbedder(cfg)

	// Parse templates
	tmpl, err := template.ParseGlob("web/templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %+v", err)
	}

	// Initialize handler with MongoDB storage and embedder
	h := handlers.NewHandler(ms, embedder, tmpl)

	// Set up Gin router
	r := gin.Default()
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	// Load HTML templates
	r.SetHTMLTemplate(tmpl)

	// Routes
	r.GET("/", h.Index)
	r.POST("/upload", h.UploadFile)
	r.POST("/delete", h.DeleteFile)
	r.GET("/files", h.GetFileList)
	r.GET("/download", h.DownloadFile)
	r.GET("/generate-search", h.GenerateSearch)
	
	// Serve static files
	r.Static("/static", "./web/static")

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(r.Run(":8080"))
}
