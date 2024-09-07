package testing

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"github.com/sdrshn-nmbr/tusk/internal/storage"
)

func TestGenerateResponse(t *testing.T) {
	cfg, _ := config.NewConfig()

	sysPrompt := `You are an AI assistant that helps users with their queries. Do NOT mention the documents anywhere in your response - make it sound as natural as possible.`

	model, _ := ai.NewModel(cfg, sysPrompt)
	defer model.Close()

	ctx := context.Background()
	query := "What is the capital of Ethiopia?"

	responseChan, errChan := model.GenerateResponse(ctx, query, nil)

	for {
		select {
		case response, ok := <-responseChan:
			if !ok {
				// Channel closed, we're done
				return
			}
			t.Log(response) // Print each chunk of the response
		case err, ok := <-errChan:
			if !ok {
				// Error channel closed
				return
			}
			if err != nil {
				t.Fatalf("Error generating response 2: %+v", err)
			}
		}
	}
}

func TestGeneratewithVectorSearch(t *testing.T) {

	cfg, err := config.NewConfig()
	if err != nil {
		t.Logf("Error: %+v", err)
	}

	ms, err := storage.NewMongoStorage(cfg)
	if err != nil {
		t.Logf("Error: %+v", err)
	}

	sysPrompt :=
		`You are an AI assistant that helps users with their queries. Do NOT mention the documents anywhere in your response - make it sound as natural as possible.`

	model, err := ai.NewModel(cfg, sysPrompt)
	if err != nil {
		log.Fatalf("Error: %+v", err)
	}
	defer model.Close()

	query := "In a what paper was mentioned a shocking finding where scientists unicorns? tell me more about this and what is mentioned in the paper."

	embedder := ai.NewEmbedder(cfg)

	queryEmbedding, err := embedder.GenerateEmbedding(query)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %+v", err)
	}

	chunks, err := ms.VectorSearch(queryEmbedding, 50, 2, "user_id", "collection_id")
	if err != nil {
		t.Logf("Error: %+v", err)
	}

	log.Printf("\n\n\n<<<CHUNKS>>>\n\n\n")

	var chunkStr string = ""

	for i, chunk := range chunks {
		// To see if chunks are accurate and formatted
		t.Logf("\n\n\nChunk for iter %d: \n\n%s\n\n", i, chunk.Content)

		chunkStr += fmt.Sprintf("Document %d: \n%s\n\n", i, chunk.Content)
	}

	// queryandchunks := fmt.Sprintf("%s\n Query: %s", chunkStr, query)

	// t.Logf("\n\n\n <<<Query and Chunks>>>\n%s\n\n\n", queryandchunks)

	ctx := context.Background()

	responseChan, errorChan := model.GenerateResponse(ctx, query, nil, chunkStr)

	for {
		select {
		case response, ok := <-responseChan:
			if !ok {
				// Channel closed, we're done
				return
			}
			t.Log(response) // Print each chunk of the response
		case err, ok := <-errorChan:
			if !ok {
				// Error channel closed
				return
			}
			if err != nil {
				t.Fatalf("Error generating response 3: %+v", err)
			}
		}
	}
}

func TestGenerateVision(t *testing.T) {
	cfg, _ := config.NewConfig()
	sysPrompt := `You are an AI assistant that helps users with their queries. Do NOT mention the documents anywhere in your response - make it sound as natural as possible.`

	model, _ := ai.NewModel(cfg, sysPrompt)
	defer model.Close()

	ctx := context.Background()
	query := "What is in this image?"

	responseChan, errChan := model.GenerateResponse(ctx, query, nil)

	for {
		select {
		case response, ok := <-responseChan:
			if !ok {
				// Channel closed, we're done
				return
			}
			t.Log(response) // Print each chunk of the response
		case err, ok := <-errChan:
			if !ok {
				// Error channel closed
				return
			}
			if err != nil {
				t.Fatalf("Error generating response 2: %+v", err)
			}
		}
	}
}
