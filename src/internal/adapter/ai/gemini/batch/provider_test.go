package batch

import (
	"context"
	"testing"

	"github.com/specvital/worker/internal/domain/specview"
)

func TestBatchJobState_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		state    BatchJobState
		expected bool
	}{
		{
			name:     "pending is not terminal",
			state:    JobStatePending,
			expected: false,
		},
		{
			name:     "running is not terminal",
			state:    JobStateRunning,
			expected: false,
		},
		{
			name:     "succeeded is terminal",
			state:    JobStateSucceeded,
			expected: true,
		},
		{
			name:     "failed is terminal",
			state:    JobStateFailed,
			expected: true,
		},
		{
			name:     "cancelled is terminal",
			state:    JobStateCancelled,
			expected: true,
		},
		{
			name:     "expired is terminal",
			state:    JobStateExpired,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.IsTerminal()
			if got != tt.expected {
				t.Errorf("IsTerminal() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestCreateClassificationJob(t *testing.T) {
	t.Run("should convert Phase1Input to BatchRequest", func(t *testing.T) {
		config := BatchConfig{
			APIKey:      "test-key",
			Phase1Model: "gemini-2.5-flash",
			UseBatchAPI: true,
		}

		// Note: NewProvider requires a real API client, so we test CreateClassificationJob
		// logic in isolation by creating a provider with minimal setup
		provider := &Provider{
			config: config,
		}

		input := specview.Phase1Input{
			AnalysisID: "test-analysis",
			Files: []specview.FileInfo{
				{
					Path:      "test/example.test.js",
					Framework: "jest",
					Tests: []specview.TestInfo{
						{
							Index: 0,
							Name:  "should work correctly",
						},
					},
				},
			},
			Language: specview.Language("en"),
		}

		req, err := provider.CreateClassificationJob(input)
		if err != nil {
			t.Fatalf("CreateClassificationJob() error = %v", err)
		}

		if req.AnalysisID != "test-analysis" {
			t.Errorf("AnalysisID = %v, expected test-analysis", req.AnalysisID)
		}

		if req.Model != "gemini-2.5-flash" {
			t.Errorf("Model = %v, expected gemini-2.5-flash", req.Model)
		}

		if len(req.Requests) != 1 {
			t.Fatalf("len(Requests) = %d, expected 1", len(req.Requests))
		}

		request := req.Requests[0]
		if len(request.Contents) == 0 {
			t.Error("Contents should not be empty")
		}

		if request.Config == nil {
			t.Fatal("Config should not be nil")
		}

		if request.Config.ResponseMIMEType != "application/json" {
			t.Errorf("ResponseMIMEType = %v, expected application/json", request.Config.ResponseMIMEType)
		}
	})

	t.Run("should return error when no files", func(t *testing.T) {
		provider := &Provider{
			config: BatchConfig{
				Phase1Model: "gemini-2.5-flash",
			},
		}

		input := specview.Phase1Input{
			AnalysisID: "test-analysis",
			Files:      []specview.FileInfo{},
			Language:   specview.Language("en"),
		}

		_, err := provider.CreateClassificationJob(input)
		if err == nil {
			t.Error("CreateClassificationJob() should return error when no files")
		}
	})
}

func TestBatchConfig_Validate(t *testing.T) {
	t.Run("should return error when API key is empty", func(t *testing.T) {
		config := BatchConfig{
			Phase1Model: "gemini-2.5-flash",
		}

		err := config.Validate()
		if err == nil {
			t.Error("Validate() should return error when API key is empty")
		}
	})

	t.Run("should return error when Phase1Model is empty", func(t *testing.T) {
		config := BatchConfig{
			APIKey: "test-key",
		}

		err := config.Validate()
		if err == nil {
			t.Error("Validate() should return error when Phase1Model is empty")
		}
	})

	t.Run("should return nil when config is valid", func(t *testing.T) {
		config := BatchConfig{
			APIKey:      "test-key",
			Phase1Model: "gemini-2.5-flash",
		}

		err := config.Validate()
		if err != nil {
			t.Errorf("Validate() should return nil for valid config, got %v", err)
		}
	})
}

func TestNewProvider(t *testing.T) {
	t.Run("should return error when API key is empty", func(t *testing.T) {
		_, err := NewProvider(context.Background(), BatchConfig{
			Phase1Model: "gemini-2.5-flash",
		})
		if err == nil {
			t.Error("NewProvider() should return error when API key is empty")
		}
	})

	t.Run("should return error when Phase1Model is empty", func(t *testing.T) {
		_, err := NewProvider(context.Background(), BatchConfig{
			APIKey: "test-key",
		})
		if err == nil {
			t.Error("NewProvider() should return error when Phase1Model is empty")
		}
	})
}

func TestSplitInputIntoChunks(t *testing.T) {
	t.Run("should not split when under threshold", func(t *testing.T) {
		input := specview.Phase1Input{
			AnalysisID: "test",
			Language:   "Korean",
			Files: []specview.FileInfo{
				{Path: "a.test.js", Tests: makeTests(100)},
				{Path: "b.test.js", Tests: makeTests(200)},
			},
		}

		chunks := splitInputIntoChunks(input, 1000)

		if len(chunks) != 1 {
			t.Errorf("len(chunks) = %d, expected 1", len(chunks))
		}
		if len(chunks[0].Files) != 2 {
			t.Errorf("len(Files) = %d, expected 2", len(chunks[0].Files))
		}
	})

	t.Run("should split when over threshold", func(t *testing.T) {
		input := specview.Phase1Input{
			AnalysisID: "test",
			Language:   "Korean",
			Files: []specview.FileInfo{
				{Path: "a.test.js", Tests: makeTests(600)},
				{Path: "b.test.js", Tests: makeTests(300)}, // fits with a
				{Path: "c.test.js", Tests: makeTests(600)}, // new chunk
			},
		}

		chunks := splitInputIntoChunks(input, 1000)

		// a (600) + b (300) = 900 < 1000 -> chunk 1
		// c (600) -> chunk 2
		if len(chunks) != 2 {
			t.Errorf("len(chunks) = %d, expected 2", len(chunks))
		}
		if len(chunks[0].Files) != 2 {
			t.Errorf("chunks[0].Files = %d, expected 2 (a+b)", len(chunks[0].Files))
		}
		if len(chunks[1].Files) != 1 {
			t.Errorf("chunks[1].Files = %d, expected 1 (c)", len(chunks[1].Files))
		}
	})

	t.Run("should preserve metadata in all chunks", func(t *testing.T) {
		input := specview.Phase1Input{
			AnalysisID: "my-analysis",
			Language:   "English",
			Files: []specview.FileInfo{
				{Path: "a.test.js", Tests: makeTests(600)},
				{Path: "b.test.js", Tests: makeTests(600)},
			},
		}

		chunks := splitInputIntoChunks(input, 500)

		for i, chunk := range chunks {
			if chunk.AnalysisID != "my-analysis" {
				t.Errorf("chunk[%d].AnalysisID = %q, expected %q", i, chunk.AnalysisID, "my-analysis")
			}
			if chunk.Language != "English" {
				t.Errorf("chunk[%d].Language = %q, expected %q", i, chunk.Language, "English")
			}
		}
	})
}

func TestCreateClassificationJobChunking(t *testing.T) {
	t.Run("should create multiple requests for large input", func(t *testing.T) {
		provider := &Provider{
			config: BatchConfig{
				Phase1Model: "gemini-2.5-flash",
			},
		}

		// Create input with > 5000 tests
		input := specview.Phase1Input{
			AnalysisID: "test-analysis",
			Language:   "Korean",
			Files: []specview.FileInfo{
				{Path: "a.test.js", Tests: makeTests(3000)},
				{Path: "b.test.js", Tests: makeTests(3000)},
			},
		}

		req, err := provider.CreateClassificationJob(input)
		if err != nil {
			t.Fatalf("CreateClassificationJob() error = %v", err)
		}

		if len(req.Requests) != 2 {
			t.Errorf("len(Requests) = %d, expected 2", len(req.Requests))
		}
		if req.ChunkCount != 2 {
			t.Errorf("ChunkCount = %d, expected 2", req.ChunkCount)
		}
	})
}

// makeTests creates n test infos with sequential indices.
func makeTests(n int) []specview.TestInfo {
	tests := make([]specview.TestInfo, n)
	for i := 0; i < n; i++ {
		tests[i] = specview.TestInfo{Index: i, Name: "test"}
	}
	return tests
}
