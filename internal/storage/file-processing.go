package storage

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

func init() {
	err := license.SetMeteredKey("5adb58fab7ae295b061fda4390cbb5b363d1089f89c515c4ef64d078c8ad2e5a")
	if err != nil {
		panic(err)
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

func ChunkText(text string, chunkSize int) []string {
	t0 := time.Now()

	overlap := 50
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var chunks []string
	currentChunk := strings.Builder{}
	currentLength := 0

	for _, word := range words {
		if currentLength+len(word)+1 > chunkSize && currentLength > 0 {
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))

			// Reset for next chunk, keeping overlap
			overlapStart := len(chunks[len(chunks)-1]) - overlap
			if overlapStart < 0 {
				overlapStart = 0
			}
			currentChunk.Reset()
			currentChunk.WriteString(chunks[len(chunks)-1][overlapStart:])
			currentChunk.WriteString(" ")
			currentLength = currentChunk.Len()
		}

		currentChunk.WriteString(word)
		currentChunk.WriteString(" ")
		currentLength += len(word) + 1
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}
	
	log.Printf("\n\n\nTIME TAKEN FOR CHUNK TEXT: %d\n\n\n", time.Since(t0))

	return chunks
}
