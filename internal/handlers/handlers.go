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
	handler := &Handler{
		Storage: storage, Tmpl: tmpl,
	}

	return handler
}

func (h *Handler) Index(c *gin.Context) {
	files, err := h.Storage.ListFiles()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var fileInfos []FileInfo
	for _, file := range files {
		size, err := h.Storage.GetFileSize(file)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		fileInfos = append(fileInfos, FileInfo{Name: file, Size: formatFileSize(size)})
	}

	c.HTML(http.StatusOK, "layout.html", gin.H{
		"Files": fileInfos,
	})
}

func (h *Handler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, err.Error())
		return
	}
	defer file.Close()

	err = h.Storage.SaveFile(header.Filename, file)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	files, err := h.Storage.ListFiles()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var fileInfos []FileInfo
	for _, file := range files {
		size, err := h.Storage.GetFileSize(file)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		fileInfos = append(fileInfos, FileInfo{Name: file, Size: formatFileSize(size)})
	}

	c.HTML(http.StatusOK, "file_list", gin.H{
		"Files": fileInfos,
	})
}

func (h *Handler) DeleteFile(c *gin.Context) {
	filename := c.PostForm("filename")
	err := h.Storage.DeleteFile(filename)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	files, err := h.Storage.ListFiles()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var fileInfos []FileInfo
	for _, file := range files {
		size, err := h.Storage.GetFileSize(file)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		fileInfos = append(fileInfos, FileInfo{Name: file, Size: formatFileSize(size)})
	}

	c.HTML(http.StatusOK, "file_list", gin.H{
		"Files": fileInfos,
	})
}

func (h *Handler) GetFileList(c *gin.Context) {
	files, err := h.Storage.ListFiles()
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	var fileInfos []FileInfo
	for _, file := range files {
		size, err := h.Storage.GetFileSize(file)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		fileInfos = append(fileInfos, FileInfo{Name: file, Size: formatFileSize(size)})
	}

	c.HTML(http.StatusOK, "file_list", gin.H{
		"Files": fileInfos,
	})
}

func (h *Handler) DownloadFile(c *gin.Context) {
	filename := c.Query("filename")
	content, err := h.Storage.GetFile(filename)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(filename))
	c.Data(http.StatusOK, "application/octet-stream", content)
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
