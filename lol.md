# MongoDB Atlas Integration Plan

## 1. Project Structure Updates

Add the following directories and files to your project:

```
.
├── ...
├── internal
│   ├── ...
│   ├── database
│   │   ├── mongodb.go
│   │   └── models.go
│   ├── ai
│   │   ├── embedding.go
│   │   └── assistant.go
│   └── config
│       └── config.go
├── ...
```

## 2. New Packages to Install

Add the following Go packages to your `go.mod` file:

```go
go get go.mongodb.org/mongo-driver/mongo
go get github.com/sashabaranov/go-openai
```

## 3. Configuration (config/config.go)

Create a configuration file to store MongoDB and OpenAI API credentials:

```go
package config

import (
	"os"
)

type Config struct {
	MongoDBURI        string
	MongoDBDatabase   string
	OpenAIAPIKey      string
	VectorCollection  string
}

func LoadConfig() *Config {
	return &Config{
		MongoDBURI:        os.Getenv("MONGODB_URI"),
		MongoDBDatabase:   os.Getenv("MONGODB_DATABASE"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		VectorCollection:  "file_vectors",
	}
}
```

## 4. MongoDB Integration (database/mongodb.go)

Create a MongoDB client and functions to interact with the database:

```go
package database

import (
	"context"
	"log"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/sdrshn-nmbr/tusk/internal/config"
)

type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewMongoDB(cfg *config.Config) (*MongoDB, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoDBURI)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, err
	}

	database := client.Database(cfg.MongoDBDatabase)

	return &MongoDB{
		client:   client,
		database: database,
	}, nil
}

func (m *MongoDB) Close() {
	if err := m.client.Disconnect(context.TODO()); err != nil {
		log.Printf("Error disconnecting from MongoDB: %+v", err)
	}
}

// Add methods for CRUD operations on file metadata and vector embeddings
```

## 5. AI Integration (ai/embedding.go)

Create functions to generate and store vector embeddings:

```go
package ai

import (
	"context"

	"github.com/sashabaranov/go-openai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
)

type Embedder struct {
	client *openai.Client
}

func NewEmbedder(cfg *config.Config) *Embedder {
	client := openai.NewClient(cfg.OpenAIAPIKey)
	return &Embedder{client: client}
}

func (e *Embedder) GenerateEmbedding(text string) ([]float32, error) {
	resp, err := e.client.CreateEmbeddings(
		context.Background(),
		openai.EmbeddingRequest{
			Model: openai.AdaEmbeddingV2,
			Input: []string{text},
		},
	)
	if err != nil {
		return nil, err
	}
	return resp.Data[0].Embedding, nil
}
```

## 6. Update Storage Interface (storage/storage.go)

Modify your storage interface to include methods for storing and retrieving vector embeddings:

```go
type Storage interface {
	SaveFile(filename string, content io.Reader) error
	GetFile(filename string) ([]byte, error)
	DeleteFile(filename string) error
	ListFiles() ([]string, error)
	GetFileSize(filename string) (int64, error)
	SaveEmbedding(filename string, embedding []float32) error
	GetEmbedding(filename string) ([]float32, error)
}
```

## 7. Update Handlers (handlers/handlers.go)

Modify your handlers to generate and store embeddings when uploading files:

```go
func (h *Handler) UploadFile(c *gin.Context) {
	// ... (existing file upload logic)

	// Generate embedding
	fileContent, err := h.Storage.GetFile(header.Filename)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	embedding, err := h.Embedder.GenerateEmbedding(string(fileContent))
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	// Save embedding
	err = h.Storage.SaveEmbedding(header.Filename, embedding)
	if err != nil {
		h.handleError(c, http.StatusInternalServerError, err)
		return
	}

	// ... (rest of the handler)
}
```

## 8. Main Application (cmd/server/main.go)

Update your main application to initialize MongoDB and the embedder:

```go
func main() {
	cfg := config.LoadConfig()

	// Initialize MongoDB
	mongodb, err := database.NewMongoDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %+v", err)
	}
	defer mongodb.Close()

	// Initialize embedder
	embedder := ai.NewEmbedder(cfg)

	// ... (rest of your initialization code)

	// Update handler initialization to include MongoDB and embedder
	h := handlers.NewHandler(fs, tmpl, mongodb, embedder)

	// ... (rest of your main function)
}
```

## Next Steps

1. Implement the `SaveEmbedding` and `GetEmbedding` methods in your storage package.
2. Create models for file metadata and vector embeddings in `database/models.go`.
3. Implement CRUD operations for file metadata and vector embeddings in `database/mongodb.go`.
4. Update your handlers to use the new MongoDB storage for file metadata and embeddings.
5. Implement the AI assistant functionality in `ai/assistant.go`.
6. Update your frontend to include the AI assistant interface.
7. Modify your Dockerfile and deployment scripts to include the new environment variables for MongoDB and OpenAI.

Remember to handle errors appropriately and implement proper authentication and authorization for your application.
