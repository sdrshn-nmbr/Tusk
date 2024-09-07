// handlers.go
package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"github.com/sdrshn-nmbr/tusk/internal/storage"
	"github.com/markbates/goth/gothic"
)

type Handler struct {
	Storage  *storage.MongoStorage
	Embedder *ai.Embedder
	Model    *ai.Model
	tmpl     *template.Template
}

type FileInfo struct {
	Name      string
	Size      string
	Directory string
}

type DirectoryInfo struct {
	Name string
	Path string
}

func NewHandler(storage *storage.MongoStorage, embedder *ai.Embedder, tmpl *template.Template) *Handler {
	return &Handler{
		Storage:  storage,
		Embedder: embedder,
		tmpl:     tmpl,
	}
}

func (h *Handler) Index(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	currentDirectory := c.DefaultQuery("directory", "/")
	h.renderFileList(c, "layout.html", currentDirectory)
}

func (h *Handler) UploadFile(c *gin.Context) {
	userID := c.GetString("user_id")
	directory := c.PostForm("directory")
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("Error getting file from form: %+v", err)
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		log.Printf("Error opening file: %+v", err)
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	defer openedFile.Close()

	// Create a bytes.Buffer to read the file content
	var buf bytes.Buffer
	_, err = io.Copy(&buf, openedFile)
	if err != nil {
		log.Printf("Error reading file content: %+v", err)
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	// Create a new io.Reader from the buffer
	reader := bytes.NewReader(buf.Bytes())

	err = h.Storage.SaveFile(file.Filename, reader, h.Embedder, userID, directory)
	if err != nil {
		if err.Error() == "file is not a PDF" {
			h.handleError(c, http.StatusBadRequest, err)
		} else {
			log.Printf("Error saving file: %+v", err)
			h.handleError(c, http.StatusInternalServerError, err)
		}
		return
	}

	h.renderFileList(c, "file_list", directory)
}

func (h *Handler) DeleteFile(c *gin.Context) {
	userID := c.GetString("user_id")
	filename := c.PostForm("filename")
	err := h.Storage.DeleteFileFunc(filename, userID)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}
	// Returning success status only - no re-render required
	c.Status(http.StatusOK)
}

func (h *Handler) GetFileList(c *gin.Context) {
	// userID := c.GetString("user_id")
	h.renderFileList(c, "file_list", "/")
}

func (h *Handler) DownloadFile(c *gin.Context) {
	userID := c.GetString("user_id")
	filename := c.Query("filename")
	content, err := h.Storage.GetFile(filename, userID)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filename))
	c.Data(http.StatusOK, "application/octet-stream", content)
}

func (h *Handler) GenerateSearch(c *gin.Context) {
	userID := c.GetString("user_id")
	directory := c.Query("directory")
	query := c.Query("q")
	ctx := c.Request.Context()

	embedding, err := h.Embedder.GenerateEmbedding(query)
	if err != nil {
		log.Printf("Failed to generate embedding: %+v", err)
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	chunks, err := h.Storage.VectorSearch(embedding, 500, 5, userID, directory)
	if err != nil {
		log.Printf("Failed to perform vector search: %+v", err)
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	chunkStr := new(bytes.Buffer)
	for _, chunk := range chunks {
		// fmt.Fprintf(chunkStr, "Document %d: \n%s\n\n", i, chunk.Content)
		fmt.Fprintf(chunkStr, "\n%s\n\n", chunk.Content)
	}

	// queryandchunks := fmt.Sprintf("%s\n Query: %s", chunkStr.String(), query)

	cfg, err := config.NewConfig()
	if err != nil {
		log.Printf("Failed to load config: %+v", err)
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	sysPrompt :=
		`You are an AI assistant that helps users with their queries. Do NOT mention the documents anywhere in your response - make it sound as natural as possible.`

	model, err := ai.NewModel(cfg, sysPrompt)
	if err != nil {
		log.Printf("Failed to create model: %+v", err)
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}
	defer model.Close()

	responseChan, errorChan := model.GenerateResponse(ctx, query, nil, chunkStr.String())
	// responseChan, errorChan := model.GenerateResponsePplx(ctx, query)

	modelResponse := new(bytes.Buffer)
	timeout := time.After(30 * time.Second)

	for {
		select {
		case response, ok := <-responseChan:
			if !ok {
				// Response channel closed, all data received
				c.JSON(http.StatusOK, gin.H{
					"query":   query,
					"results": modelResponse.String(),
				})
				return
			}
			modelResponse.WriteString(response)

		case err, ok := <-errorChan:
			if !ok {
				// Error channel closed without error
				if modelResponse.Len() == 0 {
					c.JSON(http.StatusOK, gin.H{
						"query":   query,
						"results": "No results found.",
					})
				} else {
					c.JSON(http.StatusOK, gin.H{
						"query":   query,
						"results": modelResponse.String(),
					})
				}
				return
			}
			log.Printf("Error generating response: %+v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate response"})
			return

		case <-ctx.Done():
			log.Printf("Request cancelled by client")
			c.JSON(http.StatusRequestTimeout, gin.H{"error": "Request timed out"})
			return

		case <-timeout:
			log.Printf("Request timed out after 30 seconds")
			if modelResponse.Len() == 0 {
				c.JSON(http.StatusOK, gin.H{
					"query":   query,
					"results": "The request timed out. Please try again.",
				})
			} else {
				c.JSON(http.StatusOK, gin.H{
					"query":   query,
					"results": modelResponse.String(),
				})
			}
			return
		}
	}
}

func (h *Handler) renderFileList(c *gin.Context, templateName, currentDirectory string) {
	userID := c.GetString("user_id")
	files, err := h.Storage.ListFiles(userID, currentDirectory)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	var fileInfos []FileInfo
	for _, file := range files {
		size, err := h.Storage.GetFileSize(file)
		if err != nil {
			h.handleError(c, http.StatusInternalServerError, err)
			return
		}
		fileInfos = append(fileInfos, FileInfo{Name: file, Size: formatFileSize(size), Directory: currentDirectory})
	}

	// Get subdirectories
	subdirectories, err := h.Storage.ListSubdirectories(userID, currentDirectory)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	var directoryInfos []DirectoryInfo
	for _, dir := range subdirectories {
		directoryInfos = append(directoryInfos, DirectoryInfo{Name: filepath.Base(dir), Path: dir})
	}

	c.HTML(http.StatusOK, templateName, gin.H{
		"Files":             fileInfos,
		"Directories":       directoryInfos,
		"CurrentDirectory":  currentDirectory,
		"ParentDirectory":   filepath.Dir(currentDirectory),
	})
}

func (h *Handler) handleError(c *gin.Context, statusCode int, err error) {
	log.Printf("Error occurred: %+v", err)

	c.HTML(statusCode, "error.html", gin.H{
		"ErrorMessage": err.Error(),
		"StatusCode":   statusCode,
	})
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func (h *Handler) Login(c *gin.Context) {
	log.Println("Entering Login handler")
	log.Printf("Template: %v", h.tmpl.DefinedTemplates())
	c.HTML(http.StatusOK, "login", gin.H{
		"title": "Login - Tusk",
		"debug": "This is a debug message",
	})
	log.Println("Login template rendered successfully")
}

func (h *Handler) BeginAuth(c *gin.Context) {
	provider := c.Param("provider")
	log.Printf("BeginAuth called with provider: %s", provider)
	
	if provider == "" {
		log.Println("Provider is empty")
		c.String(http.StatusBadRequest, "You must select a provider")
		return
	}
	
	q := c.Request.URL.Query()
	q.Add("provider", provider)
	c.Request.URL.RawQuery = q.Encode()
	
	log.Printf("Starting auth process for provider: %s", provider)
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

func (h *Handler) Logout(c *gin.Context) {
	session, _ := gothic.Store.Get(c.Request, "user-session")
	session.Values["user_id"] = nil
	session.Options.MaxAge = -1
	err := session.Save(c.Request, c.Writer)
	if err != nil {
		log.Printf("Error saving session: %v", err)
	}
	gothic.Logout(c.Writer, c.Request)
	c.Redirect(http.StatusFound, "/login")
}

func (h *Handler) CompleteAuth(c *gin.Context) {
	user, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"ErrorMessage": fmt.Sprintf("Error during authentication: %v", err),
			"StatusCode":   http.StatusInternalServerError,
		})
		return
	}
	session, _ := gothic.Store.Get(c.Request, "user-session")
	session.Values["user_id"] = user.UserID
	session.Save(c.Request, c.Writer)
	c.Redirect(http.StatusFound, "/")
}

func (h *Handler) CreateDirectory(c *gin.Context) {
	userID := c.GetString("user_id")
	directoryPath := c.PostForm("directory")

	if directoryPath == "" {
		h.handleError(c, http.StatusBadRequest, fmt.Errorf("directory path is required"))
		return
	}

	err := h.Storage.CreateDirectory(userID, directoryPath)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Directory created successfully"})
}
