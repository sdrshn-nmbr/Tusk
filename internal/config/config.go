package config

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
)

type Config struct {
	GeminiAPIKey       string
	OpenAIAPIKey       string
	MongoDBURI         string
	MongoDBDatabase    string
	UnidocAPIKey       string
	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string
	SessionSecret      string
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
				log.Printf("Failed to change to parent directory: %+v", err)
				break
			}
		}
	}

	// Even if .env file is not found, we'll try to use environment variables
	conf := &Config{
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		OpenAIAPIKey:       os.Getenv("OPENAI_API_KEY"),
		MongoDBURI:         os.Getenv("MONGODB_URI"),
		MongoDBDatabase:    os.Getenv("MONGODB_DATABASE"),
		UnidocAPIKey:       os.Getenv("UNIDOC_API_KEY"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GithubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GithubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
	}

	// Validate that required fields are not empty
	var missingVars []string
	if conf.OpenAIAPIKey == "" {
		missingVars = append(missingVars, "OPENAI_API_KEY")
	}
	if conf.MongoDBURI == "" {
		missingVars = append(missingVars, "MONGODB_URI")
	}
	if conf.MongoDBDatabase == "" {
		missingVars = append(missingVars, "MONGODB_DATABASE")
	}
	if conf.SessionSecret == "" {
		missingVars = append(missingVars, "SESSION_SECRET")
	}

	// Log warnings for missing optional variables
	optionalVars := []struct {
		name  string
		value string
	}{
		{"GEMINI_API_KEY", conf.GeminiAPIKey},
		{"UNIDOC_API_KEY", conf.UnidocAPIKey},
		{"GOOGLE_CLIENT_ID", conf.GoogleClientID},
		{"GOOGLE_CLIENT_SECRET", conf.GoogleClientSecret},
		{"GITHUB_CLIENT_ID", conf.GithubClientID},
		{"GITHUB_CLIENT_SECRET", conf.GithubClientSecret},
	}

	for _, v := range optionalVars {
		if v.value == "" {
			log.Printf("Warning: %s is not set in the environment", v.name)
		}
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("required environment variables are missing: %v", missingVars)
	}

	return conf, nil
}
