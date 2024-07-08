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

func ChunkText(text string, chunkSize int) []string {
	var chunks []string
	words := strings.Fields(text)
	currentChunk := make([]string, 0, chunkSize/6) // Estimate average word length of 5
	overlap := 100                                 // Number of characters to overlap between chunks
	currentLength := 0
	overlapStartIndex := 0

	for _, word := range words {
		if currentLength+len(word)+1 > chunkSize && len(currentChunk) > 0 {
			chunks = append(chunks, strings.Join(currentChunk, " "))

			// Find the start index for the overlap
			overlapLength := 0
			for i := len(currentChunk) - 1; i >= 0; i-- {
				overlapLength += len(currentChunk[i]) + 1 // +1 for space
				if overlapLength >= overlap {
					overlapStartIndex = i
					break
				}
			}

			// Reset currentChunk to the overlapping portion
			currentChunk = currentChunk[overlapStartIndex:]
			currentLength = overlapLength - 1 // -1 to not count the space before the first word
		}

		currentChunk = append(currentChunk, word)
		currentLength += len(word) + 1 // +1 for space
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, " "))
	}

	return chunks
}
