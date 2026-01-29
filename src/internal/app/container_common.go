package app

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/specvital/worker/internal/adapter/queue/fairness"
	"github.com/specvital/worker/internal/infra/config"
)

const schedulerLockKey = "scheduler:auto-refresh:lock"

// ContainerConfig holds common configuration for dependency injection containers.
type ContainerConfig struct {
	EncryptionKey     string
	Fairness          config.FairnessConfig
	GeminiAPIKey      string
	GeminiPhase1Model string // optional: default gemini-2.5-flash
	GeminiPhase2Model string // optional: default gemini-2.5-flash-lite
	MockMode          bool   // enable mock AI provider for development/testing
	ParserVersion     string
	Pool              *pgxpool.Pool
	SpecView          config.SpecViewConfig
}

// Validate checks that required common configuration fields are set.
func (c ContainerConfig) Validate() error {
	if c.Pool == nil {
		return fmt.Errorf("pool is required")
	}
	return nil
}

// ValidateAnalyzer checks that all analyzer-specific configuration is valid.
func (c ContainerConfig) ValidateAnalyzer() error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.EncryptionKey == "" {
		return fmt.Errorf("encryption key is required")
	}
	if c.ParserVersion == "" {
		return fmt.Errorf("parser version is required")
	}
	return nil
}

// ValidateSpecGenerator checks that all spec-generator-specific configuration is valid.
func (c ContainerConfig) ValidateSpecGenerator() error {
	if err := c.Validate(); err != nil {
		return err
	}
	// Skip GeminiAPIKey validation when MockMode is enabled
	if !c.MockMode && c.GeminiAPIKey == "" {
		return fmt.Errorf("gemini API key is required (set MOCK_MODE=true to skip)")
	}
	// Validate Batch API settings
	if c.SpecView.UseBatchAPI {
		if c.SpecView.BatchPollInterval <= 0 {
			return fmt.Errorf("batch poll interval must be positive when Batch API is enabled")
		}
		if c.SpecView.BatchThreshold <= 0 {
			return fmt.Errorf("batch threshold must be positive when Batch API is enabled")
		}
	}
	return nil
}

// NewFairnessMiddleware creates a new fairness middleware from configuration.
//
// Why factory pattern: Each container (analyzer/spec-generator) needs independent
// middleware instances to avoid shared state between services.
//
// Returns nil if fairness is disabled (FAIRNESS_ENABLED=false).
// Returns error if configuration is invalid.
func NewFairnessMiddleware(cfg config.FairnessConfig) (*fairness.FairnessMiddleware, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	fairnessConfig := &fairness.Config{
		FreeConcurrentLimit:       cfg.FreeConcurrentLimit,
		ProConcurrentLimit:        cfg.ProConcurrentLimit,
		EnterpriseConcurrentLimit: cfg.EnterpriseConcurrentLimit,
		SnoozeDuration:            cfg.SnoozeDuration,
		SnoozeJitter:              cfg.SnoozeJitter,
	}

	limiter, err := fairness.NewPerUserLimiter(fairnessConfig)
	if err != nil {
		return nil, fmt.Errorf("create fairness limiter: %w", err)
	}

	extractor := &fairness.JSONArgsExtractor{}

	return fairness.NewFairnessMiddleware(limiter, extractor, fairnessConfig), nil
}
