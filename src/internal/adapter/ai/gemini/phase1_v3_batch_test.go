package gemini

import (
	"fmt"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestParseV3BatchResponse_Success(t *testing.T) {
	jsonStr := `[{"d": "Authentication", "f": "Login"}, {"d": "Payment", "f": "Checkout"}]`

	results, err := parseV3BatchResponse(jsonStr)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Domain != "Authentication" {
		t.Errorf("expected domain 'Authentication', got %q", results[0].Domain)
	}
	if results[0].Feature != "Login" {
		t.Errorf("expected feature 'Login', got %q", results[0].Feature)
	}
	if results[1].Domain != "Payment" {
		t.Errorf("expected domain 'Payment', got %q", results[1].Domain)
	}
	if results[1].Feature != "Checkout" {
		t.Errorf("expected feature 'Checkout', got %q", results[1].Feature)
	}
}

func TestParseV3BatchResponse_EmptyArray(t *testing.T) {
	jsonStr := `[]`

	results, err := parseV3BatchResponse(jsonStr)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseV3BatchResponse_EmptyString(t *testing.T) {
	_, err := parseV3BatchResponse("")

	if err == nil {
		t.Fatal("expected error for empty string")
	}
}

func TestParseV3BatchResponse_NullArray(t *testing.T) {
	_, err := parseV3BatchResponse("null")

	if err == nil {
		t.Fatal("expected error for null response")
	}
}

func TestParseV3BatchResponse_ObjectInsteadOfArray(t *testing.T) {
	_, err := parseV3BatchResponse(`{"invalid": "not an array"}`)

	if err == nil {
		t.Fatal("expected error for JSON object instead of array")
	}
}

func TestParseV3BatchResponse_MalformedJSON(t *testing.T) {
	_, err := parseV3BatchResponse(`[{"d": "Auth", "f": "Login"`)

	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseV3BatchResponse_SingleItem(t *testing.T) {
	jsonStr := `[{"d": "Uncategorized", "f": "General"}]`

	results, err := parseV3BatchResponse(jsonStr)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Domain != "Uncategorized" {
		t.Errorf("expected domain 'Uncategorized', got %q", results[0].Domain)
	}
}

func TestValidateV3BatchCount_Match(t *testing.T) {
	results := []v3BatchResult{
		{Domain: "Auth", Feature: "Login"},
		{Domain: "Payment", Feature: "Checkout"},
	}

	err := validateV3BatchCount(results, 2)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateV3BatchCount_TooFew(t *testing.T) {
	results := []v3BatchResult{
		{Domain: "Auth", Feature: "Login"},
	}

	err := validateV3BatchCount(results, 3)

	if err == nil {
		t.Fatal("expected error for count mismatch")
	}
}

func TestValidateV3BatchCount_TooMany(t *testing.T) {
	results := []v3BatchResult{
		{Domain: "Auth", Feature: "Login"},
		{Domain: "Payment", Feature: "Checkout"},
		{Domain: "User", Feature: "Profile"},
	}

	err := validateV3BatchCount(results, 2)

	if err == nil {
		t.Fatal("expected error for count mismatch")
	}
}

func TestValidateV3BatchCount_EmptyExpected(t *testing.T) {
	results := []v3BatchResult{}

	err := validateV3BatchCount(results, 0)

	if err != nil {
		t.Errorf("unexpected error for empty expected: %v", err)
	}
}

func TestValidateV3BatchCount_EmptyGot(t *testing.T) {
	results := []v3BatchResult{}

	err := validateV3BatchCount(results, 2)

	if err == nil {
		t.Fatal("expected error for count mismatch")
	}
}

func TestParseV3BatchResponse_EmptyDomain(t *testing.T) {
	jsonStr := `[{"d": "", "f": "Login"}]`

	_, err := parseV3BatchResponse(jsonStr)

	if err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestParseV3BatchResponse_EmptyFeature(t *testing.T) {
	jsonStr := `[{"d": "Auth", "f": ""}]`

	_, err := parseV3BatchResponse(jsonStr)

	if err == nil {
		t.Fatal("expected error for empty feature")
	}
}

func TestParseV3BatchResponse_EmptyFieldAtSecondIndex(t *testing.T) {
	jsonStr := `[{"d": "Auth", "f": "Login"}, {"d": "", "f": "Checkout"}]`

	_, err := parseV3BatchResponse(jsonStr)

	if err == nil {
		t.Fatal("expected error for empty domain at index 1")
	}
}

func TestSplitBatch_EvenCount(t *testing.T) {
	tests := makeTestsForAssignment(4)

	left, right := splitBatch(tests)

	if len(left) != 2 {
		t.Errorf("expected left half to have 2 items, got %d", len(left))
	}
	if len(right) != 2 {
		t.Errorf("expected right half to have 2 items, got %d", len(right))
	}
}

func TestSplitBatch_OddCount(t *testing.T) {
	tests := makeTestsForAssignment(5)

	left, right := splitBatch(tests)

	if len(left) != 2 {
		t.Errorf("expected left half to have 2 items, got %d", len(left))
	}
	if len(right) != 3 {
		t.Errorf("expected right half to have 3 items, got %d", len(right))
	}
}

func TestSplitBatch_MinimumSize(t *testing.T) {
	tests := makeTestsForAssignment(2)

	left, right := splitBatch(tests)

	if len(left) != 1 {
		t.Errorf("expected left half to have 1 item, got %d", len(left))
	}
	if len(right) != 1 {
		t.Errorf("expected right half to have 1 item, got %d", len(right))
	}
}

func TestAccumulateTokenUsage_NilSource(t *testing.T) {
	target := &specview.TokenUsage{
		PromptTokens:     100,
		CandidatesTokens: 50,
		TotalTokens:      150,
	}

	accumulateTokenUsage(target, nil)

	if target.PromptTokens != 100 {
		t.Errorf("expected prompt tokens to remain 100, got %d", target.PromptTokens)
	}
}

func TestAccumulateTokenUsage_NilTarget(t *testing.T) {
	source := &specview.TokenUsage{
		PromptTokens:     100,
		CandidatesTokens: 50,
		TotalTokens:      150,
	}

	// Should not panic
	accumulateTokenUsage(nil, source)
}

func TestAccumulateTokenUsage_Accumulates(t *testing.T) {
	target := &specview.TokenUsage{
		PromptTokens:     100,
		CandidatesTokens: 50,
		TotalTokens:      150,
	}
	source := &specview.TokenUsage{
		PromptTokens:     200,
		CandidatesTokens: 100,
		TotalTokens:      300,
	}

	accumulateTokenUsage(target, source)

	if target.PromptTokens != 300 {
		t.Errorf("expected prompt tokens 300, got %d", target.PromptTokens)
	}
	if target.CandidatesTokens != 150 {
		t.Errorf("expected candidates tokens 150, got %d", target.CandidatesTokens)
	}
	if target.TotalTokens != 450 {
		t.Errorf("expected total tokens 450, got %d", target.TotalTokens)
	}
}

func TestV3Constants_Values(t *testing.T) {
	if v3BatchSize != 20 {
		t.Errorf("expected v3BatchSize=20, got %d", v3BatchSize)
	}
	if v3MaxRetries != 3 {
		t.Errorf("expected v3MaxRetries=3, got %d", v3MaxRetries)
	}
	if v3MinBatchSizeForSplit != 4 {
		t.Errorf("expected v3MinBatchSizeForSplit=4, got %d", v3MinBatchSizeForSplit)
	}
}

func makeTestsForAssignment(count int) []specview.TestForAssignment {
	tests := make([]specview.TestForAssignment, count)
	for i := 0; i < count; i++ {
		tests[i] = specview.TestForAssignment{
			FilePath: "test.spec.ts",
			Name:     fmt.Sprintf("test-%d", i),
		}
	}
	return tests
}

// v3RetryStrategy tests the retry/split/individual fallback flow.
// Uses a mock batch processor to simulate various failure scenarios.
type v3RetryStrategy struct {
	model          string
	batchProcessor func(tests []specview.TestForAssignment) ([]v3BatchResult, *specview.TokenUsage, error)
}

func (s *v3RetryStrategy) processWithRetry(
	tests []specview.TestForAssignment,
) ([]v3BatchResult, *specview.TokenUsage, int, int) {
	totalUsage := &specview.TokenUsage{Model: s.model}
	retryCount := 0
	fallbackCount := 0

	if len(tests) == 0 {
		return []v3BatchResult{}, totalUsage, 0, 0
	}

	// Try batch processing with retries
	for attempt := 1; attempt <= v3MaxRetries; attempt++ {
		results, usage, err := s.batchProcessor(tests)
		if usage != nil {
			accumulateTokenUsage(totalUsage, usage)
		}

		if err == nil {
			return results, totalUsage, retryCount, fallbackCount
		}
		retryCount++
	}

	// Batch processing failed - try splitting
	if len(tests) >= v3MinBatchSizeForSplit {
		left, right := splitBatch(tests)

		leftResults, leftUsage, leftRetries, leftFallbacks := s.processWithRetry(left)
		accumulateTokenUsage(totalUsage, leftUsage)
		retryCount += leftRetries
		fallbackCount += leftFallbacks

		rightResults, rightUsage, rightRetries, rightFallbacks := s.processWithRetry(right)
		accumulateTokenUsage(totalUsage, rightUsage)
		retryCount += rightRetries
		fallbackCount += rightFallbacks

		results := make([]v3BatchResult, 0, len(leftResults)+len(rightResults))
		results = append(results, leftResults...)
		results = append(results, rightResults...)

		return results, totalUsage, retryCount, fallbackCount
	}

	// Fall back to individual processing
	results := make([]v3BatchResult, 0, len(tests))
	for _, test := range tests {
		singleTest := []specview.TestForAssignment{test}
		result, usage, err := s.batchProcessor(singleTest)
		if usage != nil {
			accumulateTokenUsage(totalUsage, usage)
		}

		if err != nil || len(result) != 1 {
			results = append(results, v3BatchResult{
				Domain:  uncategorizedDomainName,
				Feature: uncategorizedFeatureName,
			})
			fallbackCount++
			continue
		}
		results = append(results, result[0])
	}

	return results, totalUsage, retryCount, fallbackCount
}

func TestV3RetryStrategy_FirstAttemptSuccess(t *testing.T) {
	callCount := 0
	strategy := &v3RetryStrategy{
		model: "test-model",
		batchProcessor: func(tests []specview.TestForAssignment) ([]v3BatchResult, *specview.TokenUsage, error) {
			callCount++
			results := make([]v3BatchResult, len(tests))
			for i := range tests {
				results[i] = v3BatchResult{Domain: "Auth", Feature: "Login"}
			}
			return results, &specview.TokenUsage{PromptTokens: 100, CandidatesTokens: 50}, nil
		},
	}

	tests := makeTestsForAssignment(5)
	results, usage, retries, fallbacks := strategy.processWithRetry(tests)

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
	if retries != 0 {
		t.Errorf("expected 0 retries, got %d", retries)
	}
	if fallbacks != 0 {
		t.Errorf("expected 0 fallbacks, got %d", fallbacks)
	}
	if usage.PromptTokens != 100 {
		t.Errorf("expected 100 prompt tokens, got %d", usage.PromptTokens)
	}
}

func TestV3RetryStrategy_RetryThenSuccess(t *testing.T) {
	callCount := 0
	strategy := &v3RetryStrategy{
		model: "test-model",
		batchProcessor: func(tests []specview.TestForAssignment) ([]v3BatchResult, *specview.TokenUsage, error) {
			callCount++
			// Fail first 2 attempts, succeed on 3rd
			if callCount < 3 {
				return nil, &specview.TokenUsage{PromptTokens: 50}, fmt.Errorf("transient error")
			}
			results := make([]v3BatchResult, len(tests))
			for i := range tests {
				results[i] = v3BatchResult{Domain: "Payment", Feature: "Checkout"}
			}
			return results, &specview.TokenUsage{PromptTokens: 100}, nil
		},
	}

	tests := makeTestsForAssignment(3)
	results, usage, retries, fallbacks := strategy.processWithRetry(tests)

	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if retries != 2 {
		t.Errorf("expected 2 retries, got %d", retries)
	}
	if fallbacks != 0 {
		t.Errorf("expected 0 fallbacks, got %d", fallbacks)
	}
	// 2 failed attempts (50 each) + 1 success (100) = 200
	if usage.PromptTokens != 200 {
		t.Errorf("expected 200 prompt tokens, got %d", usage.PromptTokens)
	}
}

func TestV3RetryStrategy_SplitAfterRetryExhausted(t *testing.T) {
	callCount := 0
	strategy := &v3RetryStrategy{
		model: "test-model",
		batchProcessor: func(tests []specview.TestForAssignment) ([]v3BatchResult, *specview.TokenUsage, error) {
			callCount++
			// Large batches fail, small batches (<=2) succeed
			if len(tests) > 2 {
				return nil, &specview.TokenUsage{PromptTokens: 10}, fmt.Errorf("batch too large")
			}
			results := make([]v3BatchResult, len(tests))
			for i := range tests {
				results[i] = v3BatchResult{Domain: "User", Feature: "Profile"}
			}
			return results, &specview.TokenUsage{PromptTokens: 20}, nil
		},
	}

	tests := makeTestsForAssignment(4) // Will split to 2+2
	results, _, retries, fallbacks := strategy.processWithRetry(tests)

	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}
	// 3 retries for batch of 4, then split succeeds immediately (2+2)
	if retries != 3 {
		t.Errorf("expected 3 retries (for initial batch), got %d", retries)
	}
	if fallbacks != 0 {
		t.Errorf("expected 0 fallbacks, got %d", fallbacks)
	}
}

func TestV3RetryStrategy_IndividualFallback(t *testing.T) {
	callCount := 0
	strategy := &v3RetryStrategy{
		model: "test-model",
		batchProcessor: func(tests []specview.TestForAssignment) ([]v3BatchResult, *specview.TokenUsage, error) {
			callCount++
			// All batch calls fail, only single-item calls succeed
			if len(tests) > 1 {
				return nil, &specview.TokenUsage{PromptTokens: 10}, fmt.Errorf("batch fails")
			}
			return []v3BatchResult{{Domain: "Individual", Feature: "Success"}}, &specview.TokenUsage{PromptTokens: 5}, nil
		},
	}

	tests := makeTestsForAssignment(3) // Too small to split (< 4)
	results, _, retries, fallbacks := strategy.processWithRetry(tests)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if retries != 3 {
		t.Errorf("expected 3 retries, got %d", retries)
	}
	if fallbacks != 0 {
		t.Errorf("expected 0 fallbacks (individual succeeded), got %d", fallbacks)
	}
	// All results should be from individual processing
	for i, r := range results {
		if r.Domain != "Individual" {
			t.Errorf("result %d: expected domain 'Individual', got %q", i, r.Domain)
		}
	}
}

func TestV3RetryStrategy_UncategorizedFallback(t *testing.T) {
	strategy := &v3RetryStrategy{
		model: "test-model",
		batchProcessor: func(tests []specview.TestForAssignment) ([]v3BatchResult, *specview.TokenUsage, error) {
			// All calls fail
			return nil, &specview.TokenUsage{PromptTokens: 10}, fmt.Errorf("always fails")
		},
	}

	tests := makeTestsForAssignment(2) // Too small to split
	results, _, retries, fallbacks := strategy.processWithRetry(tests)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	if retries != 3 {
		t.Errorf("expected 3 retries, got %d", retries)
	}
	if fallbacks != 2 {
		t.Errorf("expected 2 fallbacks, got %d", fallbacks)
	}
	// All results should be Uncategorized
	for i, r := range results {
		if r.Domain != uncategorizedDomainName {
			t.Errorf("result %d: expected domain %q, got %q", i, uncategorizedDomainName, r.Domain)
		}
		if r.Feature != uncategorizedFeatureName {
			t.Errorf("result %d: expected feature %q, got %q", i, uncategorizedFeatureName, r.Feature)
		}
	}
}

func TestV3RetryStrategy_EmptyInput(t *testing.T) {
	callCount := 0
	strategy := &v3RetryStrategy{
		model: "test-model",
		batchProcessor: func(tests []specview.TestForAssignment) ([]v3BatchResult, *specview.TokenUsage, error) {
			callCount++
			return nil, nil, nil
		},
	}

	results, usage, retries, fallbacks := strategy.processWithRetry([]specview.TestForAssignment{})

	if callCount != 0 {
		t.Errorf("expected 0 calls for empty input, got %d", callCount)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if retries != 0 || fallbacks != 0 {
		t.Errorf("expected 0 retries/fallbacks for empty input")
	}
	if usage == nil {
		t.Error("expected non-nil usage")
	}
}
