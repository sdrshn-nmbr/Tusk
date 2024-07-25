package storage

import (
	"bytes"
	"errors"
	"io"
	"log"
	"strings"

	"github.com/sdrshn-nmbr/tusk/internal/config"
	"github.com/unidoc/unioffice/document"
	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

const (
	chunkSize = 2048
	overlap   = 50
)

func init() {
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatal("Config not initialized properly")
	}

	// Initialize Unidoc license
	err = license.SetMeteredKey(cfg.UnidocAPIKey)
	if err != nil {
		log.Fatalf("Failed to set Unidoc license: %v", err)
	}
}

func extractTextFromPDF(content io.Reader) (string, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(content)
	if err != nil {
		return "", err
	}

	pdfReader, err := model.NewPdfReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return "", err
	}

	isEncrypted, err := pdfReader.IsEncrypted()
	if err != nil {
		return "", err
	}

	if isEncrypted {
		return "", errors.New("PDF is encrypted")
	}

	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder
	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			return "", err
		}

		ex, err := extractor.New(page)
		if err != nil {
			return "", err
		}

		text, err := ex.ExtractText()
		if err != nil {
			return "", err
		}

		textBuilder.WriteString(text)
	}

	return textBuilder.String(), nil
}

func extractTextFromImage(content string) (string, error){
	return content[:10], nil
}

func extractTextFromDOCX(content io.Reader) (string, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return "", err
	}
	
	doc, err := document.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder
	for _, para := range doc.Paragraphs() {
		for _, run := range para.Runs() {
			textBuilder.WriteString(run.Text())
		}
		textBuilder.WriteString("\n")
	}

	return textBuilder.String(), nil
}

func ChunkText(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	chunks := make([]string, 0, len(text)/(chunkSize-overlap)+1)
	currentChunk := strings.Builder{}
	currentChunk.Grow(chunkSize + overlap)

	for _, word := range words {
		if currentChunk.Len()+len(word)+1 > chunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))

			// Reset for next chunk, keeping overlap
			overlapStart := currentChunk.Len() - overlap
			if overlapStart < 0 {
				overlapStart = 0
			}
			overlap := currentChunk.String()[overlapStart:]
			currentChunk.Reset()
			currentChunk.Grow(chunkSize + len(overlap))
			currentChunk.WriteString(overlap)
			currentChunk.WriteByte(' ')
		}

		currentChunk.WriteString(word)
		currentChunk.WriteByte(' ')
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}
