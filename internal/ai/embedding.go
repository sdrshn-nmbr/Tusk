package ai

import (
	"context"
	"fmt"
	"log"

	"github.com/sashabaranov/go-openai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
)

type Embedder struct {
	client *openai.Client
}

func NewEmbedder(cfg *config.Config) *Embedder {
	return &Embedder{
		client: openai.NewClient(cfg.OpenAIAPIKey),
	}
}

// GenerateEmbedding generates an embedding for a single text.
func (e *Embedder) GenerateEmbedding(text string) ([]float32, error) {
	embeddings, err := e.GenerateEmbeddings([]string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) != 1 {
		return nil, fmt.Errorf("expected 1 embedding, got %d", len(embeddings))
	}
	return embeddings[0], nil
}

// GenerateEmbeddings generates embeddings for a batch of texts.
func (e *Embedder) GenerateEmbeddings(texts []string) ([][]float32, error) {
	queryRequest := openai.EmbeddingRequest{
		Input: texts,
		Model: openai.AdaEmbeddingV2, // Use the appropriate model
	}

	queryResponse, err := e.client.CreateEmbeddings(context.Background(), queryRequest)
	if err != nil {
		log.Printf("Error creating embeddings: %+v", err)
		return nil, err
	}

	// Ensure the number of embeddings matches the number of inputs
	if len(queryResponse.Data) != len(texts) {
		err := fmt.Errorf("mismatch in number of embeddings: expected %d, got %d", len(texts), len(queryResponse.Data))
		log.Printf("Error: %v", err)
		return nil, err
	}

	embeddings := make([][]float32, len(queryResponse.Data))
	for i, data := range queryResponse.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}
