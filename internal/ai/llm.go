package ai

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/google/generative-ai-go/genai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Model struct {
	client *genai.Client
	model  *genai.GenerativeModel
	chat   *genai.ChatSession
}

func NewModel(cfg *config.Config) *Model {
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	model := client.GenerativeModel("gemini-1.5-flash-latest")
	chat := model.StartChat()

	// Set the initial "system prompt"
	chat.History = []*genai.Content{
		{
			Parts: []genai.Part{
				// genai.Text("You are an AI assistant that helps users with their queries about documents. Your responses should be clear, concise, and directly related to the context and query provided."),
				genai.Text("You are an AI assistant that helps users with their queries. Do not specifically mention the documents anywhere in your response - make it sound as natural as possible."),
			},
			Role: "user",
		},
	}

	return &Model{
		client: client,
		model:  model,
		chat:   chat,
	}
}


// * Usage:
// * 1) First we take in the query
// * 2) Then we embed it and use the VectorSearch function to query all the 
// * 3) We retrieve the chunks from the VectorSearch function and combine the content from them into a single string
// * 4) We then pass that in as the context with the original query from the user into the generate response
func (m *Model) GenerateResponse(ctx context.Context, query string, output io.Writer) error {
	iter := m.chat.SendMessageStream(ctx, genai.Text(query))

	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("error generating content: %v", err)
		}

		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
				if textPart, ok := part.(genai.Text); ok {
					_, err := fmt.Fprint(output, string(textPart))
					if err != nil {
						return fmt.Errorf("error writing output: %v", err)
					}
				}
			}
		}
	}

	return nil
}

func (m *Model) Close() error {
	return m.client.Close()
}
