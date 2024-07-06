package storage

import (
	"context"
	"testing"

	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"go.mongodb.org/mongo-driver/bson"
)

func TestVectorSearch(t *testing.T) {
	query := "What modern framework greatly reduced the problems in distributed computing"

	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create a new MongoDB instance
	mongodb, err := NewMongoStorage(cfg, "documents")
	if err != nil {
		t.Fatalf("Failed to create MongoDB instance: %v", err)
	}

	// Test MongoDB connection
	err = mongodb.client.Ping(context.TODO(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	t.Log("Successfully connected to MongoDB")

	embedder := ai.NewEmbedder(cfg)

	queryEmbedding, err := embedder.GenerateEmbedding(query)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %v", err)
	}

	t.Logf("Generated embedding length: %d", len(queryEmbedding))

	if len(queryEmbedding) == 0 {
		t.Error("Generated embedding is empty")
	}

	var result []Document
	results, err := mongodb.VectorSearch(queryEmbedding, 3, 1)
	if err != nil {
		t.Fatalf("VectorSearch failed: %v", err)
	}

	t.Logf("Number of results: %d", len(results))
	for i, res := range results {
		t.Logf("Result %d:", i+1)
		t.Logf("  Filename: %s", res.Filename)
		t.Logf("  Number of chunks: %d", len(res.Chunks))
		if len(res.Chunks) > 0 {
			t.Logf("  Matching chunk: %.100s...", res.Chunks[0])
		}
		if score, ok := res.Metadata["score"]; ok {
			t.Logf("  Score: %s", score)
		}
	}

	// If no results, let's check if there are any documents in the collection
	if len(result) == 0 {
		count, err := mongodb.client.Database(mongodb.database).Collection("documents").CountDocuments(context.TODO(), bson.M{})
		if err != nil {
			t.Fatalf("Failed to count documents: %v", err)
		}
		t.Logf("Total number of documents in collection: %d", count)
	}
}
