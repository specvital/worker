package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/adapter/ai/reliability"
	"github.com/specvital/worker/internal/domain/specview"
)

// phase3Response represents the expected JSON response from Phase 3.
type phase3Response struct {
	Summary string `json:"summary"`
}

// generateSummary performs Phase 3: executive summary generation.
// Reuses Phase 1 circuit breaker and retry since it's a similar classification task.
func (p *Provider) generateSummary(ctx context.Context, input specview.Phase3Input) (*specview.Phase3Output, *specview.TokenUsage, error) {
	if len(input.Domains) == 0 {
		return nil, nil, fmt.Errorf("%w: no domains to summarize", specview.ErrInvalidInput)
	}

	systemPrompt := prompt.Phase3SystemPrompt
	userPrompt := prompt.BuildPhase3UserPrompt(input)

	var output *specview.Phase3Output
	var usage *specview.TokenUsage

	err := p.phase1Retry.Do(ctx, func() error {
		result, innerUsage, innerErr := p.generateContent(ctx, p.phase1Model, systemPrompt, userPrompt, p.phase1CB)
		if innerErr != nil {
			return innerErr
		}
		usage = innerUsage

		parsed, parseErr := parsePhase3Response(result)
		if parseErr != nil {
			slog.WarnContext(ctx, "failed to parse phase 3 response, will retry",
				"error", parseErr,
				"response", truncateForLog(result, 500),
			)
			return &reliability.RetryableError{Err: parseErr}
		}

		output = parsed
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("phase 3 generate summary: %w", err)
	}

	return output, usage, nil
}

// parsePhase3Response parses the Phase 3 JSON response.
func parsePhase3Response(text string) (*specview.Phase3Output, error) {
	var resp phase3Response
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal phase 3 response: %w", err)
	}

	if resp.Summary == "" {
		return nil, fmt.Errorf("empty summary in phase 3 response")
	}

	return &specview.Phase3Output{
		Summary: resp.Summary,
	}, nil
}
