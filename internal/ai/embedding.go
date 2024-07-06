package ai

import (
	"context"
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

func (e *Embedder) GenerateEmbedding(text string) ([]float32, error) {
	queryRequest := openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.AdaEmbeddingV2,
	}

	queryResponse, err := e.client.CreateEmbeddings(context.Background(), queryRequest)
	if err != nil {
		log.Printf("Error creating query embedding: %v", err)
		return nil, err
	}

	return queryResponse.Data[0].Embedding, nil
}
