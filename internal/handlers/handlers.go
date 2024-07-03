package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/sdrshn-nmbr/tusk/internal/storage"
)

type Handler struct {
	Storage *storage.FileStorage
	Tmpl    *template.Template
}

type FileInfo struct {
	Name string
	Size string
}

func NewHandler(storage *storage.FileStorage, tmpl *template.Template) *Handler {
	return &Handler{Storage: storage, Tmpl: tmpl}
}

func (h *Handler) Index(c *gin.Context) {
	h.renderFileList(c, "layout.html")
}

func (h *Handler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.handleError(c, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	err = h.Storage.SaveFile(header.Filename, file)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	h.renderFileList(c, "file_list")
}

func (h *Handler) DeleteFile(c *gin.Context) {
	filename := c.PostForm("filename")
	err := h.Storage.DeleteFileFunc(filename)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}
	// Returning success status only - no re-render required
	c.Status(http.StatusOK)
}

func (h *Handler) GetFileList(c *gin.Context) {
	h.renderFileList(c, "file_list")
}

func (h *Handler) DownloadFile(c *gin.Context) {
	filename := c.Query("filename")
	content, err := h.Storage.GetFile(filename)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filename))
	c.Data(http.StatusOK, "application/octet-stream", content)
}

func (h *Handler) renderFileList(c *gin.Context, templateName string) {
	files, err := h.Storage.ListFiles()
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
		fileInfos = append(fileInfos, FileInfo{Name: file, Size: formatFileSize(size)})
	}

	c.HTML(http.StatusOK, templateName, gin.H{"Files": fileInfos})
}

func (h *Handler) handleError(c *gin.Context, statusCode int, err error) {
	c.String(statusCode, err.Error())
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
