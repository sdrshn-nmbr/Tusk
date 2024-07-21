package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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

type OllamaResponse struct {
	Response string `json:"response"`
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
				genai.Text("You are an AI assistant that helps users with their queries. Do NOT mention the documents anywhere in your response - make it sound as natural as possible."),
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

func (m *Model) GenerateResponseOllama(ctx context.Context, query string, chunks ...string) (<-chan string, <-chan error) {
	responseChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		defer close(responseChan)
		defer close(errChan)

		url := "http://localhost:11434/api/generate"

		var chunkStr strings.Builder
		for _, chunk := range chunks {
			chunkStr.WriteString("Chunks for context: \n\n")
			chunkStr.WriteString(chunk + "\n")
		}

		payload := map[string]any{
			"model":  "phi3",
			"prompt": query + chunkStr.String(),
			"stream": true,
			"messages": []map[string]string{
				{
					"role": "system",
					"content": "You are an AI assistant that helps users with their queries. Do not mention the text provided by the user in your response explicitly. Only use it for reference to help you generate your response. Make your response sound as natural as possible",
				},
			},
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			errChan <- fmt.Errorf("error marshaling JSON: %v", err)
			return
		}

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			errChan <- fmt.Errorf("error making request: %v", err)
			return
		}
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadBytes('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				errChan <- fmt.Errorf("error reading response: %v", err)
				return
			}

			var streamResp struct {
				Response string `json:"response"`
			}
			if err := json.Unmarshal(line, &streamResp); err != nil {
				errChan <- fmt.Errorf("error unmarshaling JSON: %v", err)
				return
			}

			responseChan <- streamResp.Response
		}
	}()

	return responseChan, errChan
}
