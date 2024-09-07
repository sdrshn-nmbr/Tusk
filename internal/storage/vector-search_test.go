package storage

import (
	"context"
	"testing"

	"github.com/sdrshn-nmbr/tusk/internal/ai"
	"github.com/sdrshn-nmbr/tusk/internal/config"
	"go.mongodb.org/mongo-driver/bson"
)

func TestVectorSearch(t *testing.T) {
	query := "In a what paper was mentioned a shocking finding where scientists unicorns? tell me more about this and what is mentioned in the paper."

	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %+v", err)
	}

	// Create a new MongoDB instance
	mongodb, err := NewMongoStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create MongoDB instance: %+v", err)
	}

	// Test MongoDB connection
	err = mongodb.client.Ping(context.TODO(), nil)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %+v", err)
	}
	t.Log("Successfully connected to MongoDB")

	embedder := ai.NewEmbedder(cfg)

	queryEmbedding, err := embedder.GenerateEmbedding(query)
	if err != nil {
		t.Fatalf("Failed to generate embedding: %+v", err)
	}

	t.Logf("Generated embedding length: %d", len(queryEmbedding))

	if len(queryEmbedding) == 0 {
		t.Error("Generated embedding is empty")
	}

	results, err := mongodb.VectorSearch(queryEmbedding, 20, 1, "user_id", "collection_id")
	if err != nil {
		t.Fatalf("VectorSearch failed: %+v", err)
	}

	t.Logf("Number of results: %d", len(results))
	for i, res := range results {
		t.Logf("Result %d:", i+1)
		t.Logf("  Filename: %s", res.parent)
		t.Logf("  Matching chunk: %s...", res.Content)
	}

	// If no results, let's check if there are any chunks in the collection
	if len(results) == 0 {
		count, err := mongodb.client.Database(mongodb.database).Collection(mongodb.chunksCollection).CountDocuments(context.TODO(), bson.M{})
		if err != nil {
			t.Fatalf("Failed to count chunks: %+v", err)
		}
		t.Logf("Total number of chunks in collection: %d", count)
	}
}
