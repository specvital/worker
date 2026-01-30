package batch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/specvital/worker/internal/domain/specview"
)

// ErrNoResponses indicates the batch result contains no responses.
var ErrNoResponses = errors.New("batch: no responses in result")

// ErrEmptyResponse indicates a response has no content.
var ErrEmptyResponse = errors.New("batch: empty response content")

// ErrInvalidJSON indicates the response content is not valid JSON.
var ErrInvalidJSON = errors.New("batch: invalid JSON in response")

// phase1Response represents the expected JSON response from Phase 1.
type phase1Response struct {
	Domains []phase1Domain `json:"domains"`
}

type phase1Domain struct {
	Confidence  float64         `json:"confidence"`
	Description string          `json:"description"`
	Features    []phase1Feature `json:"features"`
	Name        string          `json:"name"`
}

type phase1Feature struct {
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
	Name        string  `json:"name"`
	TestIndices []int   `json:"test_indices"`
}

// ParseResult represents the parsed result of a single batch response.
type ParseResult struct {
	Error  error // parsing error for this response (nil if success)
	Index  int   // original request index
	Output *specview.Phase1Output
}

// ParseBatchResults parses all responses in a batch result into Phase1Outputs.
// Supports partial success - returns results for successful parses and errors for failures.
func ParseBatchResults(result *BatchResult) ([]ParseResult, error) {
	if result == nil {
		return nil, errors.New("batch: nil result")
	}

	if len(result.Responses) == 0 {
		return nil, ErrNoResponses
	}

	results := make([]ParseResult, 0, len(result.Responses))

	for i, resp := range result.Responses {
		parseResult := ParseResult{Index: i}

		output, err := parseInlinedResponse(resp)
		if err != nil {
			parseResult.Error = fmt.Errorf("response %d: %w", i, err)
		} else {
			parseResult.Output = output
		}

		results = append(results, parseResult)
	}

	return results, nil
}

// ParseClassificationResponse parses a batch result into a single ClassificationJobResult.
// Supports both single and chunked (multi-response) batch jobs.
// For chunked jobs, responses are parsed and merged into a single Phase1Output.
func ParseClassificationResponse(result *BatchResult) (*ClassificationJobResult, error) {
	if result == nil {
		return nil, errors.New("batch: nil result")
	}

	if len(result.Responses) == 0 {
		return nil, ErrNoResponses
	}

	// Single response case
	if len(result.Responses) == 1 {
		output, err := parseInlinedResponse(result.Responses[0])
		if err != nil {
			return nil, err
		}
		return &ClassificationJobResult{
			Output:     output,
			TokenUsage: result.TokenUsage,
		}, nil
	}

	// Multiple responses (chunked) - parse each and merge
	outputs := make([]*specview.Phase1Output, 0, len(result.Responses))
	for i, resp := range result.Responses {
		output, err := parseInlinedResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("chunk %d: %w", i, err)
		}
		outputs = append(outputs, output)
	}

	merged := MergeChunkOutputs(outputs)
	return &ClassificationJobResult{
		Output:     merged,
		TokenUsage: result.TokenUsage,
	}, nil
}

// parseInlinedResponse parses a single InlinedResponse into Phase1Output.
func parseInlinedResponse(resp *genai.InlinedResponse) (*specview.Phase1Output, error) {
	if resp == nil {
		return nil, errors.New("nil response")
	}

	// Check for response-level error
	if resp.Error != nil {
		return nil, fmt.Errorf("response error: %s", resp.Error.Message)
	}

	if resp.Response == nil {
		return nil, ErrEmptyResponse
	}

	// Extract text from candidates
	text, err := extractResponseText(resp.Response)
	if err != nil {
		return nil, err
	}

	// Parse JSON into Phase1Output
	return parsePhase1JSON(text)
}

// extractResponseText extracts text content from a GenerateContentResponse.
// Concatenates all text parts as Batch API may split a single JSON across multiple parts.
// Returns ErrResponseTruncated if the response was cut off due to MAX_TOKENS limit.
func extractResponseText(resp *genai.GenerateContentResponse) (string, error) {
	if resp == nil || len(resp.Candidates) == 0 {
		return "", ErrEmptyResponse
	}

	candidate := resp.Candidates[0]

	// Check for MAX_TOKENS truncation
	if candidate.FinishReason == genai.FinishReasonMaxTokens {
		return "", ErrResponseTruncated
	}

	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", ErrEmptyResponse
	}

	// Concatenate all text parts - Batch API splits large JSON across multiple parts
	var sb strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			sb.WriteString(part.Text)
		}
	}

	text := sb.String()
	if text == "" {
		return "", ErrEmptyResponse
	}

	return text, nil
}

// extractJSONFromMarkdown removes markdown code block wrapper if present.
// Handles: ```json\n{...}\n``` or ```\n{...}\n```
func extractJSONFromMarkdown(s string) string {
	s = strings.TrimSpace(s)

	// Remove ```json or ``` prefix
	if after, found := strings.CutPrefix(s, "```json"); found {
		s = after
	} else if after, found := strings.CutPrefix(s, "```"); found {
		s = after
	}

	// Remove ``` suffix (unconditional - TrimSuffix is no-op if not present)
	s = strings.TrimSuffix(s, "```")

	return strings.TrimSpace(s)
}

// extractFirstJSON extracts the first complete JSON object from a string.
// Handles cases where extra text appears after the JSON (e.g., "[Note: ...]").
func extractFirstJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return s
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return s[start:] // Return as-is if incomplete (will fail parsing with clear error)
}

// removeTrailingCommas removes trailing commas before ] or } in JSON.
// This handles a common LLM output issue where models generate invalid JSON with trailing commas.
// Example: [1, 2, 3,] -> [1, 2, 3]
// Example: {"a": 1,} -> {"a": 1}
func removeTrailingCommas(s string) string {
	result := make([]byte, 0, len(s))
	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escaped {
			escaped = false
			result = append(result, c)
			continue
		}

		if c == '\\' && inString {
			escaped = true
			result = append(result, c)
			continue
		}

		if c == '"' {
			inString = !inString
			result = append(result, c)
			continue
		}

		if inString {
			result = append(result, c)
			continue
		}

		// Skip comma if followed by ] or } (with optional whitespace)
		if c == ',' {
			// Look ahead to see if next non-whitespace is ] or }
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
				j++
			}
			if j < len(s) && (s[j] == ']' || s[j] == '}') {
				// Skip this trailing comma
				continue
			}
		}

		result = append(result, c)
	}

	return string(result)
}

// parsePhase1JSON parses JSON string into Phase1Output.
func parsePhase1JSON(jsonStr string) (*specview.Phase1Output, error) {
	// Try to extract JSON from markdown code block if present
	cleaned := extractJSONFromMarkdown(jsonStr)

	// Extract only the first JSON object (ignore trailing text)
	cleaned = extractFirstJSON(cleaned)

	// Remove trailing commas (common LLM output issue)
	cleaned = removeTrailingCommas(cleaned)

	var resp phase1Response
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		preview := cleaned
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		return nil, fmt.Errorf("%w: %v (preview: %s)", ErrInvalidJSON, err, preview)
	}

	output := &specview.Phase1Output{
		Domains: make([]specview.DomainGroup, 0, len(resp.Domains)),
	}

	for _, d := range resp.Domains {
		domain := specview.DomainGroup{
			Confidence:  d.Confidence,
			Description: d.Description,
			Features:    make([]specview.FeatureGroup, 0, len(d.Features)),
			Name:        d.Name,
		}

		for _, f := range d.Features {
			feature := specview.FeatureGroup{
				Confidence:  f.Confidence,
				Description: f.Description,
				Name:        f.Name,
				TestIndices: f.TestIndices,
			}
			domain.Features = append(domain.Features, feature)
		}

		output.Domains = append(output.Domains, domain)
	}

	return output, nil
}

// ErrResponseTruncated indicates the response was truncated due to MAX_TOKENS limit.
var ErrResponseTruncated = errors.New("batch: response truncated (MAX_TOKENS limit reached)")

// MergeChunkOutputs merges multiple Phase1Outputs from chunked requests into a single output.
// Domains with the same name are merged, combining their features.
func MergeChunkOutputs(outputs []*specview.Phase1Output) *specview.Phase1Output {
	if len(outputs) == 0 {
		return nil
	}
	if len(outputs) == 1 {
		return outputs[0]
	}

	// Map domain name -> merged domain
	domainMap := make(map[string]*specview.DomainGroup)
	var domainOrder []string // preserve order

	for _, output := range outputs {
		if output == nil {
			continue
		}
		for _, domain := range output.Domains {
			existing, exists := domainMap[domain.Name]
			if !exists {
				// New domain - copy it
				copied := specview.DomainGroup{
					Confidence:  domain.Confidence,
					Description: domain.Description,
					Features:    make([]specview.FeatureGroup, len(domain.Features)),
					Name:        domain.Name,
				}
				copy(copied.Features, domain.Features)
				domainMap[domain.Name] = &copied
				domainOrder = append(domainOrder, domain.Name)
			} else {
				// Existing domain - merge features
				existing.Features = append(existing.Features, domain.Features...)
				// Update confidence if higher
				if domain.Confidence > existing.Confidence {
					existing.Confidence = domain.Confidence
				}
			}
		}
	}

	// Build result preserving order
	result := &specview.Phase1Output{
		Domains: make([]specview.DomainGroup, 0, len(domainOrder)),
	}
	for _, name := range domainOrder {
		result.Domains = append(result.Domains, *domainMap[name])
	}

	return result
}

// ValidatePhase1Output validates the parsed output against expected test indices.
func ValidatePhase1Output(output *specview.Phase1Output, expectedIndices map[int]bool) error {
	if output == nil || len(output.Domains) == 0 {
		return errors.New("no domains in output")
	}

	for _, domain := range output.Domains {
		if domain.Name == "" {
			return errors.New("domain name is empty")
		}
		for _, feature := range domain.Features {
			if feature.Name == "" {
				return fmt.Errorf("feature name is empty in domain %q", domain.Name)
			}
			for _, idx := range feature.TestIndices {
				if idx < 0 {
					return fmt.Errorf("negative test index %d in feature %q", idx, feature.Name)
				}
				if !expectedIndices[idx] {
					return fmt.Errorf("unexpected test index %d in feature %q", idx, feature.Name)
				}
			}
		}
	}

	return nil
}

// CountCoveredIndices counts how many expected indices are covered in the output.
func CountCoveredIndices(output *specview.Phase1Output) int {
	if output == nil {
		return 0
	}

	covered := make(map[int]bool)

	for _, domain := range output.Domains {
		for _, feature := range domain.Features {
			for _, idx := range feature.TestIndices {
				covered[idx] = true
			}
		}
	}

	return len(covered)
}
