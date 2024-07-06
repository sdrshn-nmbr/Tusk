package storage

import (
	"bytes"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

func extractTextFromPDF(content io.Reader) (string, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(content)
	if err != nil {
		return "", err
	}

	reader, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return "", err
	}

	var text string
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		pageText, err := page.GetPlainText(nil)
		if err != nil {
			return "", err
		}
		text += pageText
	}

	return text, nil
}

func chunkText(text string, chunkSize int) []string {
	var chunks []string
	words := strings.Fields(text)
	currentChunk := ""

	for _, word := range words {
		if len(currentChunk)+len(word)+1 > chunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, strings.TrimSpace(currentChunk))
			currentChunk = ""
		}
		if len(currentChunk) > 0 {
			currentChunk += " "
		}
		currentChunk += word
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk))
	}

	return chunks
}
