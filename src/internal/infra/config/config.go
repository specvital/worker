package config

import (
	"errors"
	"os"
	"strconv"
)

// Default worker counts for queue allocation
const (
	defaultAnalyzerPriorityWorkers  = 5
	defaultAnalyzerDefaultWorkers   = 3
	defaultAnalyzerScheduledWorkers = 2

	defaultSpecgenPriorityWorkers  = 3
	defaultSpecgenDefaultWorkers   = 2
	defaultSpecgenScheduledWorkers = 1
)

// QueueWorkers defines worker counts for each queue tier.
type QueueWorkers struct {
	Priority  int
	Default   int
	Scheduled int
}

// QueueConfig contains queue allocation settings for all services.
type QueueConfig struct {
	Analyzer QueueWorkers
	Specgen  QueueWorkers
}

type Config struct {
	DatabaseURL       string
	EncryptionKey     string
	GeminiAPIKey      string
	GeminiPhase1Model string
	GeminiPhase2Model string
	MockMode          bool
	Queue             QueueConfig
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

	return &Config{
		DatabaseURL:       databaseURL,
		EncryptionKey:     encryptionKey,
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		GeminiPhase1Model: os.Getenv("GEMINI_PHASE1_MODEL"),
		GeminiPhase2Model: os.Getenv("GEMINI_PHASE2_MODEL"),
		MockMode:          os.Getenv("MOCK_MODE") == "true",
		Queue:             loadQueueConfig(),
	}, nil
}

func loadQueueConfig() QueueConfig {
	return QueueConfig{
		Analyzer: QueueWorkers{
			Priority:  getEnvInt("ANALYZER_QUEUE_PRIORITY_WORKERS", defaultAnalyzerPriorityWorkers),
			Default:   getEnvInt("ANALYZER_QUEUE_DEFAULT_WORKERS", defaultAnalyzerDefaultWorkers),
			Scheduled: getEnvInt("ANALYZER_QUEUE_SCHEDULED_WORKERS", defaultAnalyzerScheduledWorkers),
		},
		Specgen: QueueWorkers{
			Priority:  getEnvInt("SPECGEN_QUEUE_PRIORITY_WORKERS", defaultSpecgenPriorityWorkers),
			Default:   getEnvInt("SPECGEN_QUEUE_DEFAULT_WORKERS", defaultSpecgenDefaultWorkers),
			Scheduled: getEnvInt("SPECGEN_QUEUE_SCHEDULED_WORKERS", defaultSpecgenScheduledWorkers),
		},
	}
}

func getEnvInt(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	if parsed <= 0 {
		return defaultValue
	}
	return parsed
}
