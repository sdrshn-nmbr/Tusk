package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoURI           string
	MongoDBDatabase    string
	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string
	OpenAIAPIKey       string
	GeminiAPIKey       string
	UnidocAPIKey       string
}

func NewConfig() (*Config, error) {
	// Load .env file if it exists
	godotenv.Load()

	return &Config{
		MongoURI:           os.Getenv("MONGO_URI"),
		MongoDBDatabase:    os.Getenv("MONGODB_DATABASE"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GithubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GithubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		UnidocAPIKey:       os.Getenv("UNIDOC_API_KEY"),
	}, nil
}
