package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/sdrshn-nmbr/tusk/internal/ai"
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
		log.Fatalf("Failed to set Unidoc license: %+v", err)
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

func extractTextFromImage(imgContent []byte) (string, error) {
	log.Println("Starting extractTextFromImage function")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	cfg, err := config.NewConfig()
	if err != nil {
		log.Printf("Failed to load config: %+v", err)
		return "", err
	}
	log.Println("Config loaded successfully")

	sysPrompt :=
		`You are an AI assistant that extracts and summarizes text from images, acting essentially as an OCR model.
		Once you are done extracting all text, if there is more information than just text from the images, describe it in as much detail as possible.`

	log.Println("Creating new AI model")
	model, err := ai.NewModel(cfg, sysPrompt)
	if err != nil {
		log.Printf("Failed to create model: %+v", err)
		return "", err
	}
	defer model.Close()
	log.Println("AI model created successfully")

	query := "Extract and summarize any text visible in this image."

	log.Println("Generating response from AI model")
	responseChan, errorChan := model.GenerateResponse(ctx, query, imgContent)

	modelResponse := new(bytes.Buffer)
	timeout := time.After(30 * time.Second)

	log.Println("Entering response processing loop")
	for {
		select {
		case response, ok := <-responseChan:
			if !ok {
				log.Println("Response channel closed")
				if modelResponse.Len() > 0 {
					log.Println("Returning accumulated response")
					return modelResponse.String(), nil
				}
				return "", fmt.Errorf("response channel closed unexpectedly")
			}
			log.Println("Received response chunk, appending to buffer")
			modelResponse.WriteString(response)

		case err, ok := <-errorChan:
			if !ok {
				log.Println("Error channel closed")
				if modelResponse.Len() > 0 {
					log.Println("Returning accumulated response despite error channel closure")
					log.Print("\n\n\n==============================================")
					log.Print("RESPONSE")
					log.Print("==============================================\n\n\n")
					log.Printf("%s\n\n\n", modelResponse.String())
					return modelResponse.String(), nil
				}
				return "", fmt.Errorf("error channel closed unexpectedly")
			}
			log.Printf("Error generating response: %+v", err)
			return "", err

		case <-ctx.Done():
			log.Printf("Request cancelled by client")
			if modelResponse.Len() > 0 {
				return modelResponse.String(), ctx.Err()
			}
			return "", ctx.Err()

		case <-timeout:
			log.Printf("Request timed out after 30 seconds")
			if modelResponse.Len() > 0 {
				log.Println("Returning partial response due to timeout")
				return modelResponse.String(), fmt.Errorf("request timed out after 30 seconds")
			}
			return "", fmt.Errorf("request timed out after 30 seconds")
		}
	}
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
