package config

import (
	"fmt"
	"log"
	"os"
	// "path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKey     string
	OpenAIAPIKey     string
	MongoDBURI       string
	MongoDBDatabase  string
	VectorCollection string
}

func NewConfig() (*Config, error) {
	// Try to load .env file from current directory and up to 3 parent directories
	for i := 0; i < 4; i++ {
		err := godotenv.Load()
		if err == nil {
			break
		}
		if i < 3 {
			// Change to parent directory
			err = os.Chdir("..")
			if err != nil {
				log.Printf("Failed to change to parent directory: %v", err)
				break
			}
		}
	}

	// Even if .env file is not found, we'll try to use environment variables
	conf := &Config{
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		MongoDBURI:       os.Getenv("MONGODB_URI"),
		MongoDBDatabase:  os.Getenv("MONGODB_DATABASE"),
		VectorCollection: "file_vectors",
	}

	// Validate that OpenAIAPIKey is not empty
	if conf.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set in the environment")
	}

	return conf, nil
}
