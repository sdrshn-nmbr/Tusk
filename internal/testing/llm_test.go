package ai

import (
	"context"
	"fmt"
	"os"

	// "runtime/msan"
	"testing"

	// "github.com/pelletier/go-toml/query"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/storage"
)

func TestGenerateResponse(t *testing.T) {
	cfg, err := config.NewConfig()
	if err != nil {
		t.Logf("Error: %+v", err)
	}
	model := ai.NewModel(cfg)
	defer model.Close()

	ctx := context.Background()
	err = model.GenerateResponse(ctx, "Tell me about the benefits of regular exercise", os.Stdout)
	if err != nil {
		t.Fatalf("Error generating response: %v", err)
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

	model := ai.NewModel(cfg)
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

	t.Logf("\n\n\n <<<Query and Chunks>>>\n%s\n\n\n", queryandchunks)

	ctx := context.Background()
	err = model.GenerateResponse(ctx, queryandchunks, os.Stdout)
	if err != nil {
		t.Fatalf("Error generating response: %v", err)
	}
}
