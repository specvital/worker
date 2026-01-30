package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
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

// FairnessConfig defines per-tier concurrent job limits and snooze parameters.
type FairnessConfig struct {
	Enabled                   bool
	FreeConcurrentLimit       int
	ProConcurrentLimit        int
	EnterpriseConcurrentLimit int
	SnoozeDuration            time.Duration
	SnoozeJitter              time.Duration
}

type Config struct {
	DatabaseURL       string
	EncryptionKey     string
	Fairness          FairnessConfig
	GeminiAPIKey      string
	GeminiPhase1Model string
	GeminiPhase2Model string
	MockMode          bool
	Phase1V2Enabled   bool
	Phase1V3Enabled   bool
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
		Fairness:          loadFairnessConfig(),
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		GeminiPhase1Model: os.Getenv("GEMINI_PHASE1_MODEL"),
		GeminiPhase2Model: os.Getenv("GEMINI_PHASE2_MODEL"),
		MockMode:          os.Getenv("MOCK_MODE") == "true",
		Phase1V2Enabled:   getEnvBool("SPECVIEW_PHASE1_V2", false),
		Phase1V3Enabled:   getEnvBool("SPECVIEW_PHASE1_V3", false),
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

// loadFairnessConfig loads fairness settings from environment variables.
// Defaults: ENABLED=true, FREE=1, PRO=3, ENTERPRISE=5, SNOOZE=30s, JITTER=10s
func loadFairnessConfig() FairnessConfig {
	cfg := FairnessConfig{
		Enabled:                   getEnvBool("FAIRNESS_ENABLED", true),
		FreeConcurrentLimit:       getEnvInt("FAIRNESS_FREE_LIMIT", 1),
		ProConcurrentLimit:        getEnvInt("FAIRNESS_PRO_LIMIT", 3),
		EnterpriseConcurrentLimit: getEnvInt("FAIRNESS_ENTERPRISE_LIMIT", 5),
		SnoozeDuration:            getEnvDuration("FAIRNESS_SNOOZE_DURATION", 30*time.Second),
		SnoozeJitter:              getEnvDuration("FAIRNESS_SNOOZE_JITTER", 10*time.Second),
	}

	if cfg.Enabled {
		if cfg.FreeConcurrentLimit <= 0 {
			panic(fmt.Errorf("FAIRNESS_FREE_LIMIT must be positive, got %d", cfg.FreeConcurrentLimit))
		}
		if cfg.ProConcurrentLimit <= 0 {
			panic(fmt.Errorf("FAIRNESS_PRO_LIMIT must be positive, got %d", cfg.ProConcurrentLimit))
		}
		if cfg.EnterpriseConcurrentLimit <= 0 {
			panic(fmt.Errorf("FAIRNESS_ENTERPRISE_LIMIT must be positive, got %d", cfg.EnterpriseConcurrentLimit))
		}
		if cfg.SnoozeDuration <= 0 {
			panic(fmt.Errorf("FAIRNESS_SNOOZE_DURATION must be positive, got %v", cfg.SnoozeDuration))
		}
		if cfg.SnoozeJitter < 0 {
			panic(fmt.Errorf("FAIRNESS_SNOOZE_JITTER must be non-negative, got %v", cfg.SnoozeJitter))
		}
	}

	return cfg
}

func getEnvBool(key string, defaultValue bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		return defaultValue
	}
	return parsed
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
