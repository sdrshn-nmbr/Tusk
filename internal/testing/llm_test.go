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

	model, _ := ai.NewModel(cfg)
	defer model.Close()

	ctx := context.Background()
	query := "What is the capital of Ethiopia?"

	responseChan, errChan := model.GenerateResponse(ctx, query)

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
				t.Fatalf("Error generating response: %v", err)
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

	model, err := ai.NewModel(cfg)
	if err != nil {
		log.Fatalf("Error: %+v", err)
	}
	defer model.Close()

	query := "What modern framework greatly reduced the problems in distributed computing? Tell me a little bit about it."

	embedder := ai.NewEmbedder(cfg)

	queryEmbedding, err := embedder.GenerateEmbedding(query)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %+v", err)
	}

	chunks, err := ms.VectorSearch(queryEmbedding, 50, 2)
	if err != nil {
		t.Logf("Error: %+v", err)
	}

	var chunkStr string = ""

	for i, chunk := range chunks {
		chunkStr += fmt.Sprintf("Document %d: \n%s\n\n", i, chunk.Content)
	}

	queryandchunks := fmt.Sprintf("%s\n Query: %s", chunkStr, query)

	// t.Logf("\n\n\n <<<Query and Chunks>>>\n%s\n\n\n", queryandchunks)

	ctx := context.Background()

	responseChan, errorChan := model.GenerateResponse(ctx, queryandchunks)

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
				t.Fatalf("Error generating response: %v", err)
			}
		}
	}
}
