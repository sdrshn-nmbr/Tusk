package ai

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"google.golang.org/api/option"
	"google.golang.org/api/iterator"
)

type Model struct {
	client *genai.Client
	model  *genai.GenerativeModel
	chat   *genai.ChatSession
}

func NewModel(cfg *config.Config) (*Model, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %v", err)
	}
	
	model := client.GenerativeModel("gemini-1.5-flash-latest")
	chat := model.StartChat()

	chat.History = []*genai.Content{
		{
			Parts: []genai.Part{
				genai.Text("You are an AI assistant that helps users with their queries. Do not specifically mention the documents anywhere in your response - make it sound as natural as possible."),
			},
			Role: "user",
		},
	}

	return &Model{
		client: client,
		model:  model,
		chat:   chat,
	}, nil
}

func (m *Model) GenerateResponse(ctx context.Context, query string) (<-chan string, <-chan error) {
	responseChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		defer close(responseChan)
		defer close(errChan)

		iter := m.chat.SendMessageStream(ctx, genai.Text(query))

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				errChan <- fmt.Errorf("error generating content: %v", err)
				return
			}

			for _, candidate := range resp.Candidates {
				for _, part := range candidate.Content.Parts {
					if textPart, ok := part.(genai.Text); ok {
						responseChan <- string(textPart)
					}
				}
			}
		}
	}()

	return responseChan, errChan
}

func (m *Model) Close() error {
	return m.client.Close()
}
