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

type ImageData struct {
	Data     []byte
	MimeType string
}

func NewModel(cfg *config.Config, sysPrompt string) (*Model, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %+v", err)
	}

	model := client.GenerativeModel("gemini-1.5-flash-latest")
	chat := model.StartChat()

	chat.History = []*genai.Content{
		{
			Parts: []genai.Part{
				genai.Text(sysPrompt),
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

func (m *Model) GenerateResponse(ctx context.Context, query string, imgData []byte, chunks ...string) (<-chan string, <-chan error) {
	responseChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		defer close(responseChan)
		defer close(errChan)

		// Join chunks and query
		allText := strings.Join(append(chunks, "Query: "+query), "\n")

		var iter *genai.GenerateContentResponseIterator
		if imgData != nil {
			iter = m.chat.SendMessageStream(ctx, genai.Text(allText), genai.ImageData("", imgData))
		} else {
			iter = m.chat.SendMessageStream(ctx, genai.Text(allText))
		}

		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				return
			}
			if err != nil {
				errChan <- fmt.Errorf("error generating content: %+v", err)
				return
			}

			for _, candidate := range resp.Candidates {
				for _, part := range candidate.Content.Parts {
					if textPart, ok := part.(genai.Text); ok {
						select {
						case responseChan <- string(textPart):
						case <-ctx.Done():
							errChan <- ctx.Err()
							return
						}
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

func (m *Model) GenerateResponseOllama(ctx context.Context, query string) (<-chan string, <-chan error) {
	responseChan := make(chan string)
	errChan := make(chan error, 1)

	go func() {
		defer close(responseChan)
		defer close(errChan)

		url := "http://localhost:11434/api/generate"

		payload := map[string]any{
			"model":  "phi3",
			"prompt": query,
			"stream": true,
			"messages": []map[string]string{
				{
					"role":    "system",
					"content": "You are an AI assistant that answers the user's queries with the provided context which is given to you in the form of chunks. You are never to use a word that refers to the chunks in any way in your response, so as to make your response as natural sounding as possible.",
				},
			},
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			errChan <- fmt.Errorf("error with marshalling json payload: %+v", err)
			return
		}

		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			errChan <- fmt.Errorf("error with api json response: %+v", err)
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
				errChan <- fmt.Errorf("error reading bytes from response: %+v", err)
				break
			}

			var streamResp struct {
				Response string `json:"response"`
			}
			if err := json.Unmarshal(line, &streamResp); err != nil {
				errChan <- fmt.Errorf("error unmarshalling json response: %+v", err)
				break
			}

			responseChan <- streamResp.Response
		}

	}()

	return responseChan, errChan
}
