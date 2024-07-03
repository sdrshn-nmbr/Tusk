package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/sdrshn-nmbr/tusk/internal/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoDBMoviesQuery(t *testing.T) {
	// Load configuration
	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}

	// Create a new MongoDB instance
	mongodb, err := NewMongoDB(cfg)
	if err != nil {
		t.Fatalf("Failed to create MongoDB instance: %v", err)
	}
	defer mongodb.Close()

	// Use the movies collection
	coll := mongodb.database.Collection("embedded_movies")

	// Query for movies
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set a limit for the number of movies to retrieve
	const limit = 10 // Change this value to your desired limit

	// Find all movies, sort by name, and limit the results
	findOptions := options.Find().SetSort(bson.D{{Key: "year", Value: -1}}).SetLimit(int64(limit))
	cursor, err := coll.Find(ctx, bson.D{}, findOptions)
	if err != nil {
		t.Fatalf("Failed to query movies: %v", err)
	}
	defer cursor.Close(ctx)

	// Iterate through the results
	var movies []bson.M
	if err = cursor.All(ctx, &movies); err != nil {
		t.Fatalf("Failed to decode movies: %v", err)
	}

	// Check if we got any results
	if len(movies) == 0 {
		t.Errorf("No movies found in the collection")
	}

	// Print out the movie names
	fmt.Println("Movies found:")
	for _, movie := range movies {
		fmt.Printf("- title: %s, year: %s\n", movie["title"], movie["year"])
	}

	// Optional: Test a specific query
	var junglebook bson.M
	title := "Jungle Book"
	err = coll.FindOne(ctx, bson.M{"title": title}).Decode(&junglebook)
	if err != nil {
		t.Fatalf("Failed to find %s: %v", title, err)
	}
	fmt.Printf("\n<<<%s>>>\n", junglebook["title"])

	if junglebook["title"] != "Jungle Book" {
		t.Errorf("Expected to find Jungle Book, but got %v", junglebook["name"])
	}

	// Force the test to fail if we want to see output
	// t.Fail()
}
