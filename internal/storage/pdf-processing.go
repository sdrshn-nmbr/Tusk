package storage

import (
	"bytes"
	"io"
	"strings"
	"unicode"

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

	var text strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		pageText, err := page.GetPlainText(nil)
		if err != nil {
			return "", err
		}
		text.WriteString(pageText)
		text.WriteString("\n") // Add a newline between pages
	}

	return postProcessText(text.String()), nil
}

func postProcessText(text string) string {
	// Split the text into words, preserving spaces between words
	words := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || r == '\n'
	})

	// Rejoin words with proper spacing
	var result strings.Builder
	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(word)
	}

	return strings.TrimSpace(result.String())
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
