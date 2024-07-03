package main

import (
	"html/template"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/sdrshn-nmbr/tusk/internal/handlers"
	"github.com/sdrshn-nmbr/tusk/internal/storage"
)

func main() {
	// * Initialize file storage -> can be any dir you want it to be stored in
	fs, err := storage.NewFileStorage(`C:\Users\suddu\OneDrive - purdue.edu\Documents\Tusk_Files`)
	if err != nil {
		log.Fatalf("Failed to initialize file storage: %v", err)
	}

	// * Parse templates
	tmpl, err := template.ParseGlob("web/templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// * Initialize handler
	h := handlers.NewHandler(fs, tmpl)

	// * Set up Gin router
	r := gin.Default()

	// * Load HTML templates
	r.SetHTMLTemplate(tmpl)

	// Routes
	r.GET("/", h.Index)
	r.POST("/upload", h.UploadFile)
	r.POST("/delete", h.DeleteFile)
	r.GET("/files", h.GetFileList)
	r.GET("/download", h.DownloadFile)

	// * Serve static files
	r.Static("/static", "./web/static")

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(r.Run(":8080"))
}
