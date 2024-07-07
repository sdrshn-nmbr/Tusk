package ai

import (
	"fmt"
	"testing"

	"github.com/sdrshn-nmbr/tusk/internal/config"
	// "github.com/sdrshn-nmbr/tusk/internal/ai"
)

func TestGenerateEmbedding(t *testing.T) {
	chunk := "What is the meaning of life?"

	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	embedder := NewEmbedder(cfg)

	response, err := embedder.GenerateEmbedding(chunk)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}

	if len(response) == 0 {
		t.Error("Generated embedding is empty")
	}

	fmt.Print(response)
}
