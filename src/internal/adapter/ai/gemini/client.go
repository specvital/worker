package gemini

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/genai"

	"github.com/specvital/worker/internal/adapter/ai/reliability"
	"github.com/specvital/worker/internal/domain/specview"
)

const (
	defaultPhase1Model = "gemini-2.5-flash"
	defaultPhase2Model = "gemini-2.5-flash-lite"
	defaultSeed        = int32(42) // Fixed seed for deterministic output
)

// Config holds configuration for the Gemini provider.
type Config struct {
	APIKey      string
	Phase1Model string // Model for domain classification (default: gemini-2.5-flash)
	Phase2Model string // Model for test conversion (default: gemini-2.5-flash-lite)
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.APIKey == "" {
		return errors.New("gemini API key is required")
	}
	return nil
}

// Provider implements specview.AIProvider using Google Gemini.
type Provider struct {
	client      *genai.Client
	phase1Model string
	phase2Model string

	rateLimiter *reliability.RateLimiter
	phase1CB    *reliability.CircuitBreaker
	phase2CB    *reliability.CircuitBreaker
	phase1Retry *reliability.Retryer
	phase2Retry *reliability.Retryer
}

// NewProvider creates a new Gemini provider.
func NewProvider(ctx context.Context, config Config) (*Provider, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	phase1Model := config.Phase1Model
	if phase1Model == "" {
		phase1Model = defaultPhase1Model
	}

	phase2Model := config.Phase2Model
	if phase2Model == "" {
		phase2Model = defaultPhase2Model
	}

	return &Provider{
		client:      client,
		phase1Model: phase1Model,
		phase2Model: phase2Model,
		rateLimiter: reliability.GetGlobalRateLimiter(),
		phase1CB:    reliability.NewCircuitBreaker(reliability.DefaultPhase1CircuitConfig()),
		phase2CB:    reliability.NewCircuitBreaker(reliability.DefaultPhase2CircuitConfig()),
		phase1Retry: reliability.NewRetryer(reliability.DefaultPhase1RetryConfig()),
		phase2Retry: reliability.NewRetryer(reliability.DefaultPhase2RetryConfig()),
	}, nil
}

// ClassifyDomains performs Phase 1: domain and feature classification.
func (p *Provider) ClassifyDomains(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, error) {
	return p.classifyDomains(ctx, input, input.Language)
}

// ConvertTestNames performs Phase 2: test name to behavior conversion.
func (p *Provider) ConvertTestNames(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, error) {
	return p.convertTestNames(ctx, input, input.Language)
}

// Close releases resources held by the provider.
func (p *Provider) Close() error {
	// genai.Client doesn't require explicit close
	return nil
}

// generateContent calls the Gemini API with rate limiting and circuit breaker.
func (p *Provider) generateContent(ctx context.Context, model, systemPrompt, userPrompt string, cb *reliability.CircuitBreaker) (string, error) {
	// Check circuit breaker
	if !cb.Allow() {
		return "", fmt.Errorf("%w: circuit breaker open", specview.ErrAIUnavailable)
	}

	// Wait for rate limiter
	if err := p.rateLimiter.Wait(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", err
		}
		return "", fmt.Errorf("%w: %v", specview.ErrRateLimited, err)
	}

	config := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr(float32(0.0)), // Deterministic output
		Seed:             genai.Ptr(defaultSeed),
		ResponseMIMEType: "application/json",
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
	}

	result, err := p.client.Models.GenerateContent(ctx, model, genai.Text(userPrompt), config)
	if err != nil {
		cb.RecordFailure()
		slog.WarnContext(ctx, "gemini API call failed",
			"model", model,
			"error", err,
		)
		// Only wrap as retryable if it's a server-side or transient error
		if reliability.IsRetryable(err) {
			return "", &reliability.RetryableError{Err: err}
		}
		return "", err
	}

	// Extract text from response
	text := result.Text()
	if text == "" {
		cb.RecordFailure()
		return "", errors.New("empty response from Gemini")
	}

	cb.RecordSuccess()
	return text, nil
}
