package gemini

import (
	"github.com/specvital/worker/internal/domain/specview"
)

const (
	// MaxTokensPerChunk is the maximum estimated tokens per API call.
	// Gemini has 1M tokens/minute limit, we use 500K for safety margin.
	MaxTokensPerChunk = 500_000

	// MaxTestsPerChunk is the maximum number of tests per chunk.
	// Based on ~40 tokens per test average.
	MaxTestsPerChunk = 10_000

	// tokensPerTest is the estimated tokens per test (name + metadata).
	tokensPerTest = 40

	// tokensPerFile is the estimated tokens per file (path + framework + hints).
	tokensPerFile = 50

	// basePromptTokens is the estimated tokens for system prompt and boilerplate.
	basePromptTokens = 2000
)

// ChunkConfig holds configuration for test chunking.
type ChunkConfig struct {
	MaxTestsPerChunk  int
	MaxTokensPerChunk int
}

// DefaultChunkConfig returns the default chunking configuration.
func DefaultChunkConfig() ChunkConfig {
	return ChunkConfig{
		MaxTestsPerChunk:  MaxTestsPerChunk,
		MaxTokensPerChunk: MaxTokensPerChunk,
	}
}

// ChunkedInput represents a chunk of files for processing.
type ChunkedInput struct {
	Files []specview.FileInfo
}

// EstimateTokens estimates the total token count for Phase 1 input.
func EstimateTokens(files []specview.FileInfo) int {
	tokens := basePromptTokens

	for _, file := range files {
		tokens += tokensPerFile
		tokens += len(file.Tests) * tokensPerTest
	}

	return tokens
}

// countTests counts total tests across all files.
func countTests(files []specview.FileInfo) int {
	count := 0
	for _, file := range files {
		count += len(file.Tests)
	}
	return count
}

// NeedsChunking determines if the input requires chunking.
func NeedsChunking(files []specview.FileInfo, config ChunkConfig) bool {
	testCount := countTests(files)
	if testCount > config.MaxTestsPerChunk {
		return true
	}

	tokens := EstimateTokens(files)
	return tokens > config.MaxTokensPerChunk
}

// SplitIntoChunks splits files into chunks respecting token and test limits.
// Preserves file boundaries - tests from the same file stay together.
func SplitIntoChunks(files []specview.FileInfo, config ChunkConfig) []ChunkedInput {
	if !NeedsChunking(files, config) {
		return []ChunkedInput{{Files: files}}
	}

	var chunks []ChunkedInput
	var currentChunk []specview.FileInfo
	currentTokens := basePromptTokens
	currentTests := 0

	for _, file := range files {
		fileTokens := tokensPerFile + len(file.Tests)*tokensPerTest
		fileTests := len(file.Tests)

		// Check if adding this file would exceed limits
		wouldExceedTokens := currentTokens+fileTokens > config.MaxTokensPerChunk
		wouldExceedTests := currentTests+fileTests > config.MaxTestsPerChunk

		if len(currentChunk) > 0 && (wouldExceedTokens || wouldExceedTests) {
			// Finalize current chunk
			chunks = append(chunks, ChunkedInput{Files: currentChunk})

			// Start new chunk
			currentChunk = nil
			currentTokens = basePromptTokens
			currentTests = 0
		}

		currentChunk = append(currentChunk, file)
		currentTokens += fileTokens
		currentTests += fileTests
	}

	// Add remaining files as final chunk
	if len(currentChunk) > 0 {
		chunks = append(chunks, ChunkedInput{Files: currentChunk})
	}

	return chunks
}

// ReindexTests creates a copy of files with reindexed tests starting from 0.
// Returns the reindexed files and a map from new index to original index.
func ReindexTests(files []specview.FileInfo) ([]specview.FileInfo, map[int]int) {
	reindexed := make([]specview.FileInfo, len(files))
	indexMap := make(map[int]int)

	newIndex := 0
	for i, file := range files {
		reindexed[i] = specview.FileInfo{
			DomainHints: file.DomainHints,
			Framework:   file.Framework,
			Path:        file.Path,
			Tests:       make([]specview.TestInfo, len(file.Tests)),
		}

		for j, test := range file.Tests {
			indexMap[newIndex] = test.Index
			reindexed[i].Tests[j] = specview.TestInfo{
				Index:      newIndex,
				Name:       test.Name,
				SuitePath:  test.SuitePath,
				TestCaseID: test.TestCaseID,
			}
			newIndex++
		}
	}

	return reindexed, indexMap
}

// RestoreIndices adjusts test indices in the output using the index map.
func RestoreIndices(output *specview.Phase1Output, indexMap map[int]int) {
	for i := range output.Domains {
		for j := range output.Domains[i].Features {
			for k, idx := range output.Domains[i].Features[j].TestIndices {
				if originalIdx, ok := indexMap[idx]; ok {
					output.Domains[i].Features[j].TestIndices[k] = originalIdx
				}
			}
		}
	}
}

// domainAccumulator tracks domain data during merging.
type domainAccumulator struct {
	domain          specview.DomainGroup
	confidenceSum   float64
	confidenceCount int
}

// MergePhase1Outputs merges multiple Phase1Output results into one.
// Domains with the same name are merged, their features are combined.
func MergePhase1Outputs(outputs []*specview.Phase1Output) *specview.Phase1Output {
	if len(outputs) == 0 {
		return &specview.Phase1Output{}
	}

	if len(outputs) == 1 {
		return outputs[0]
	}

	domainMap := make(map[string]*domainAccumulator)
	var domainOrder []string

	for _, output := range outputs {
		for _, domain := range output.Domains {
			if existing, ok := domainMap[domain.Name]; ok {
				// Merge features into existing domain
				existing.domain.Features = append(existing.domain.Features, domain.Features...)
				// Accumulate confidence for proper averaging
				existing.confidenceSum += domain.Confidence
				existing.confidenceCount++
			} else {
				// Add new domain
				domainMap[domain.Name] = &domainAccumulator{
					domain: specview.DomainGroup{
						Confidence:  domain.Confidence,
						Description: domain.Description,
						Features:    append([]specview.FeatureGroup{}, domain.Features...),
						Name:        domain.Name,
					},
					confidenceSum:   domain.Confidence,
					confidenceCount: 1,
				}
				domainOrder = append(domainOrder, domain.Name)
			}
		}
	}

	// Build result preserving order with correct confidence averages
	result := &specview.Phase1Output{
		Domains: make([]specview.DomainGroup, 0, len(domainOrder)),
	}

	for _, name := range domainOrder {
		acc := domainMap[name]
		acc.domain.Confidence = acc.confidenceSum / float64(acc.confidenceCount)
		result.Domains = append(result.Domains, acc.domain)
	}

	return result
}
