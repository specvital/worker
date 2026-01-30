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

	// maxOutputTokens is the maximum output tokens for Gemini API.
	// Gemini 2.5 models support up to 65,536 output tokens.
	// Required for Phase 1 with large test sets (thousands of test indices in JSON).
	maxOutputTokens = int32(65536)
)

// Config holds configuration for the Gemini provider.
type Config struct {
	APIKey          string
	Phase1Model     string // Model for domain classification (default: gemini-2.5-flash)
	Phase1V2Enabled bool   // Enable Phase 1 V2 two-stage architecture
	Phase1V3Enabled bool   // Enable Phase 1 V3 sequential batch architecture
	Phase2Model     string // Model for test conversion (default: gemini-2.5-flash-lite)
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
	client          *genai.Client
	phase1Model     string
	phase1V2Enabled bool
	phase1V3Enabled bool
	phase2Model     string
	taxonomyCache   *TaxonomyCache

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

	if config.Phase1V3Enabled {
		slog.InfoContext(ctx, "phase1 v3 sequential batch architecture enabled")
	} else if config.Phase1V2Enabled {
		slog.InfoContext(ctx, "phase1 v2 two-stage architecture enabled")
	}

	return &Provider{
		client:          client,
		phase1Model:     phase1Model,
		phase1V2Enabled: config.Phase1V2Enabled,
		phase1V3Enabled: config.Phase1V3Enabled,
		phase2Model:     phase2Model,
		taxonomyCache:   NewTaxonomyCache(),
		rateLimiter:     reliability.GetGlobalRateLimiter(),
		phase1CB:        reliability.NewCircuitBreaker(reliability.DefaultPhase1CircuitConfig()),
		phase2CB:        reliability.NewCircuitBreaker(reliability.DefaultPhase2CircuitConfig()),
		phase1Retry:     reliability.NewRetryer(reliability.DefaultPhase1RetryConfig()),
		phase2Retry:     reliability.NewRetryer(reliability.DefaultPhase2RetryConfig()),
	}, nil
}

// ClassifyDomains performs Phase 1: domain and feature classification.
func (p *Provider) ClassifyDomains(ctx context.Context, input specview.Phase1Input) (*specview.Phase1Output, *specview.TokenUsage, error) {
	return p.classifyDomains(ctx, input, input.Language)
}

// ConvertTestNames performs Phase 2: test name to behavior conversion.
func (p *Provider) ConvertTestNames(ctx context.Context, input specview.Phase2Input) (*specview.Phase2Output, *specview.TokenUsage, error) {
	return p.convertTestNames(ctx, input, input.Language)
}

// PlaceNewTests places new tests into an existing domain/feature structure.
// Used for incremental caching: when tests are added, only placement is needed.
func (p *Provider) PlaceNewTests(ctx context.Context, input specview.PlacementInput) (*specview.PlacementOutput, *specview.TokenUsage, error) {
	return p.placeNewTests(ctx, input)
}

// GenerateSummary performs Phase 3: executive summary generation.
func (p *Provider) GenerateSummary(ctx context.Context, input specview.Phase3Input) (*specview.Phase3Output, *specview.TokenUsage, error) {
	return p.generateSummary(ctx, input)
}

// Close releases resources held by the provider.
func (p *Provider) Close() error {
	// genai.Client doesn't require explicit close
	return nil
}

// generateContent calls the Gemini API with rate limiting and circuit breaker.
// Returns the response text and token usage metadata.
func (p *Provider) generateContent(ctx context.Context, model, systemPrompt, userPrompt string, cb *reliability.CircuitBreaker) (string, *specview.TokenUsage, error) {
	// Check circuit breaker
	if !cb.Allow() {
		return "", nil, fmt.Errorf("%w: circuit breaker open", specview.ErrAIUnavailable)
	}

	// Wait for rate limiter
	if err := p.rateLimiter.Wait(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("%w: %v", specview.ErrRateLimited, err)
	}

	config := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr(float32(0.0)), // Deterministic output
		Seed:             genai.Ptr(defaultSeed),
		MaxOutputTokens:  maxOutputTokens,
		ResponseMIMEType: "application/json",
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
		// Disable thinking to reduce processing time for large inputs.
		// gemini-2.5-flash has thinking enabled by default (dynamic budget).
		// This significantly reduces timeout risk for Phase 1 classification.
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr(int32(0)),
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
			return "", nil, &reliability.RetryableError{Err: err}
		}
		return "", nil, err
	}

	// Check FinishReason before extracting text.
	// MAX_TOKENS indicates output was truncated - not retryable, requires input reduction.
	if len(result.Candidates) > 0 {
		candidate := result.Candidates[0]
		switch candidate.FinishReason {
		case genai.FinishReasonMaxTokens:
			cb.RecordSuccess() // API worked correctly, just hit limit
			slog.WarnContext(ctx, "gemini output truncated due to token limit",
				"model", model,
				"finish_reason", candidate.FinishReason,
				"finish_message", candidate.FinishMessage,
			)
			return "", nil, fmt.Errorf("%w: reduce input size or split into chunks", specview.ErrOutputTruncated)
		case genai.FinishReasonSafety, genai.FinishReasonRecitation, genai.FinishReasonBlocklist, genai.FinishReasonProhibitedContent, genai.FinishReasonSPII:
			cb.RecordSuccess() // API worked, content was blocked
			slog.WarnContext(ctx, "gemini output blocked by safety filters",
				"model", model,
				"finish_reason", candidate.FinishReason,
				"finish_message", candidate.FinishMessage,
			)
			return "", nil, fmt.Errorf("%w: content blocked (%s)", specview.ErrInvalidInput, candidate.FinishReason)
		}
	}

	// Extract text from response
	text := result.Text()
	if text == "" {
		cb.RecordFailure()
		return "", nil, errors.New("empty response from Gemini")
	}

	// Extract token usage from response metadata
	var usage *specview.TokenUsage
	if result.UsageMetadata != nil {
		usage = &specview.TokenUsage{
			CandidatesTokens: result.UsageMetadata.CandidatesTokenCount,
			Model:            model,
			PromptTokens:     result.UsageMetadata.PromptTokenCount,
			TotalTokens:      result.UsageMetadata.TotalTokenCount,
		}
	}

	cb.RecordSuccess()
	return text, usage, nil
}
