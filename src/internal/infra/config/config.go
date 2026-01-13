package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL       string
	EncryptionKey     string
	GeminiAPIKey      string
	GeminiPhase1Model string
	GeminiPhase2Model string
}

func Load() (*Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		return nil, errors.New("ENCRYPTION_KEY is required")
	}

	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	if geminiAPIKey == "" {
		return nil, errors.New("GEMINI_API_KEY is required")
	}

	return &Config{
		DatabaseURL:       databaseURL,
		EncryptionKey:     encryptionKey,
		GeminiAPIKey:      geminiAPIKey,
		GeminiPhase1Model: os.Getenv("GEMINI_PHASE1_MODEL"),
		GeminiPhase2Model: os.Getenv("GEMINI_PHASE2_MODEL"),
	}, nil
}
