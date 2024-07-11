package storage

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/extractor"
	"github.com/unidoc/unipdf/v3/model"
)

const (
	chunkSize = 2048
	overlap   = 50
)

func init() {
	// Make sure to load your metered License API key prior to using the library.
	// If you need a key, you can sign up and create a free one at https://cloud.unidoc.io
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
