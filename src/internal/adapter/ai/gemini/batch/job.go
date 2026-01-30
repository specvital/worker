package batch

import (
	"fmt"

	"google.golang.org/genai"

	"github.com/specvital/worker/internal/adapter/ai/prompt"
	"github.com/specvital/worker/internal/domain/specview"
)

const (
	defaultSeedForRequest = int32(42)   // Fixed seed for deterministic output
	maxOutputTokensValue  = int32(65536)
	testsPerChunk         = 5000        // Max tests per request to avoid MAX_TOKENS truncation
)

// CreateClassificationJob creates a batch job request for Phase 1 classification.
// Automatically splits large inputs into multiple chunks to avoid MAX_TOKENS truncation.
func (p *Provider) CreateClassificationJob(input specview.Phase1Input) (BatchRequest, error) {
	if len(input.Files) == 0 {
		return BatchRequest{}, fmt.Errorf("no files to classify")
	}

	// Count total tests
	totalTests := 0
	for _, file := range input.Files {
		totalTests += len(file.Tests)
	}

	// If within limit, create single request
	if totalTests <= testsPerChunk {
		request := p.createSingleRequest(input, nil)
		return BatchRequest{
			AnalysisID: input.AnalysisID,
			Model:      p.config.Phase1Model,
			Requests:   []*genai.InlinedRequest{request},
		}, nil
	}

	// Split into chunks
	chunks := splitInputIntoChunks(input, testsPerChunk)
	requests := make([]*genai.InlinedRequest, 0, len(chunks))

	for i, chunk := range chunks {
		var anchors []specview.DomainGroup
		if i > 0 {
			// Use placeholder anchors for subsequent chunks
			// Real anchors will be populated after first chunk is parsed
			anchors = []specview.DomainGroup{}
		}
		request := p.createSingleRequest(chunk, anchors)
		requests = append(requests, request)
	}

	return BatchRequest{
		AnalysisID: input.AnalysisID,
		ChunkCount: len(chunks),
		Model:      p.config.Phase1Model,
		Requests:   requests,
	}, nil
}

// createSingleRequest creates a single InlinedRequest for a Phase1Input.
func (p *Provider) createSingleRequest(input specview.Phase1Input, anchors []specview.DomainGroup) *genai.InlinedRequest {
	systemPrompt := prompt.Phase1SystemPrompt

	var userPrompt string
	if len(anchors) > 0 {
		userPrompt = prompt.BuildPhase1UserPromptWithAnchors(input, input.Language, anchors)
	} else {
		userPrompt = prompt.BuildPhase1UserPrompt(input, input.Language)
	}

	return &genai.InlinedRequest{
		Contents: []*genai.Content{
			{
				Parts: []*genai.Part{
					{Text: userPrompt},
				},
				Role: "user",
			},
		},
		Config: &genai.GenerateContentConfig{
			Temperature:      genai.Ptr(float32(0.0)), // Deterministic output
			Seed:             genai.Ptr(defaultSeedForRequest),
			MaxOutputTokens:  maxOutputTokensValue,
			ResponseMIMEType: "application/json",
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: systemPrompt}},
			},
			// Disable thinking to reduce processing time
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: genai.Ptr(int32(0)),
			},
		},
	}
}

// splitInputIntoChunks splits Phase1Input into multiple chunks based on test count.
// Preserves file boundaries - never splits tests within a single file.
func splitInputIntoChunks(input specview.Phase1Input, maxTestsPerChunk int) []specview.Phase1Input {
	var chunks []specview.Phase1Input
	var currentChunk specview.Phase1Input
	currentChunk.AnalysisID = input.AnalysisID
	currentChunk.Language = input.Language
	currentTestCount := 0

	for _, file := range input.Files {
		fileTestCount := len(file.Tests)

		// If adding this file exceeds limit and we have files, start new chunk
		if currentTestCount+fileTestCount > maxTestsPerChunk && len(currentChunk.Files) > 0 {
			chunks = append(chunks, currentChunk)
			currentChunk = specview.Phase1Input{
				AnalysisID: input.AnalysisID,
				Language:   input.Language,
			}
			currentTestCount = 0
		}

		currentChunk.Files = append(currentChunk.Files, file)
		currentTestCount += fileTestCount
	}

	// Add remaining files
	if len(currentChunk.Files) > 0 {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}
