package batch

import (
	"errors"
	"testing"

	"google.golang.org/genai"
)

func TestParseClassificationResponse(t *testing.T) {
	t.Run("should parse valid single response", func(t *testing.T) {
		jsonResponse := `{
			"domains": [
				{
					"name": "Authentication",
					"description": "User authentication and authorization",
					"confidence": 0.95,
					"features": [
						{
							"name": "Login",
							"description": "User login functionality",
							"confidence": 0.9,
							"test_indices": [0, 1, 2]
						}
					]
				}
			]
		}`

		result := &BatchResult{
			JobName: "test-job",
			State:   JobStateSucceeded,
			Responses: []*genai.InlinedResponse{
				{
					Response: &genai.GenerateContentResponse{
						Candidates: []*genai.Candidate{
							{
								Content: &genai.Content{
									Parts: []*genai.Part{
										{Text: jsonResponse},
									},
								},
							},
						},
					},
				},
			},
		}

		output, err := ParseClassificationResponse(result)
		if err != nil {
			t.Fatalf("ParseClassificationResponse() error = %v", err)
		}

		if output.Output == nil {
			t.Fatal("Output should not be nil")
		}

		if len(output.Output.Domains) != 1 {
			t.Errorf("len(Domains) = %d, expected 1", len(output.Output.Domains))
		}

		domain := output.Output.Domains[0]
		if domain.Name != "Authentication" {
			t.Errorf("Domain.Name = %q, expected %q", domain.Name, "Authentication")
		}
		if domain.Confidence != 0.95 {
			t.Errorf("Domain.Confidence = %v, expected 0.95", domain.Confidence)
		}
		if len(domain.Features) != 1 {
			t.Errorf("len(Features) = %d, expected 1", len(domain.Features))
		}

		feature := domain.Features[0]
		if feature.Name != "Login" {
			t.Errorf("Feature.Name = %q, expected %q", feature.Name, "Login")
		}
		if len(feature.TestIndices) != 3 {
			t.Errorf("len(TestIndices) = %d, expected 3", len(feature.TestIndices))
		}
	})

	t.Run("should return error for nil result", func(t *testing.T) {
		_, err := ParseClassificationResponse(nil)
		if err == nil {
			t.Error("expected error for nil result")
		}
	})

	t.Run("should return error for empty responses", func(t *testing.T) {
		result := &BatchResult{
			JobName:   "test-job",
			State:     JobStateSucceeded,
			Responses: []*genai.InlinedResponse{},
		}

		_, err := ParseClassificationResponse(result)
		if !errors.Is(err, ErrNoResponses) {
			t.Errorf("error = %v, want ErrNoResponses", err)
		}
	})

	t.Run("should return error for multiple responses", func(t *testing.T) {
		result := &BatchResult{
			JobName: "test-job",
			State:   JobStateSucceeded,
			Responses: []*genai.InlinedResponse{
				{Response: &genai.GenerateContentResponse{}},
				{Response: &genai.GenerateContentResponse{}},
			},
		}

		_, err := ParseClassificationResponse(result)
		if err == nil {
			t.Error("expected error for multiple responses")
		}
	})
}

func TestParseBatchResults(t *testing.T) {
	t.Run("should parse multiple responses with partial success", func(t *testing.T) {
		validJSON := `{"domains": [{"name": "Test", "description": "Test domain", "confidence": 0.9, "features": []}]}`
		invalidJSON := `{invalid json`

		result := &BatchResult{
			JobName: "test-job",
			State:   JobStateSucceeded,
			Responses: []*genai.InlinedResponse{
				{
					Response: &genai.GenerateContentResponse{
						Candidates: []*genai.Candidate{
							{
								Content: &genai.Content{
									Parts: []*genai.Part{{Text: validJSON}},
								},
							},
						},
					},
				},
				{
					Response: &genai.GenerateContentResponse{
						Candidates: []*genai.Candidate{
							{
								Content: &genai.Content{
									Parts: []*genai.Part{{Text: invalidJSON}},
								},
							},
						},
					},
				},
			},
		}

		results, err := ParseBatchResults(result)
		if err != nil {
			t.Fatalf("ParseBatchResults() error = %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("len(results) = %d, expected 2", len(results))
		}

		// First should succeed
		if results[0].Error != nil {
			t.Errorf("results[0].Error = %v, expected nil", results[0].Error)
		}
		if results[0].Output == nil {
			t.Error("results[0].Output should not be nil")
		}

		// Second should fail
		if results[1].Error == nil {
			t.Error("results[1].Error should not be nil")
		}
		if results[1].Output != nil {
			t.Error("results[1].Output should be nil")
		}
	})

	t.Run("should return error for nil result", func(t *testing.T) {
		_, err := ParseBatchResults(nil)
		if err == nil {
			t.Error("expected error for nil result")
		}
	})

	t.Run("should return error for no responses", func(t *testing.T) {
		result := &BatchResult{
			Responses: []*genai.InlinedResponse{},
		}

		_, err := ParseBatchResults(result)
		if !errors.Is(err, ErrNoResponses) {
			t.Errorf("error = %v, want ErrNoResponses", err)
		}
	})
}

func TestParseInlinedResponse(t *testing.T) {
	t.Run("should return error for nil response", func(t *testing.T) {
		_, err := parseInlinedResponse(nil)
		if err == nil {
			t.Error("expected error for nil response")
		}
	})

	t.Run("should return error when response has error", func(t *testing.T) {
		resp := &genai.InlinedResponse{
			Error: &genai.JobError{
				Message: "rate limit exceeded",
			},
		}

		_, err := parseInlinedResponse(resp)
		if err == nil {
			t.Error("expected error for response with error")
		}
	})

	t.Run("should return error for empty response", func(t *testing.T) {
		resp := &genai.InlinedResponse{
			Response: nil,
		}

		_, err := parseInlinedResponse(resp)
		if !errors.Is(err, ErrEmptyResponse) {
			t.Errorf("error = %v, want ErrEmptyResponse", err)
		}
	})

	t.Run("should return error for response with no candidates", func(t *testing.T) {
		resp := &genai.InlinedResponse{
			Response: &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{},
			},
		}

		_, err := parseInlinedResponse(resp)
		if !errors.Is(err, ErrEmptyResponse) {
			t.Errorf("error = %v, want ErrEmptyResponse", err)
		}
	})
}

func TestParsePhase1JSON(t *testing.T) {
	t.Run("should parse valid JSON with multiple domains", func(t *testing.T) {
		jsonStr := `{
			"domains": [
				{
					"name": "Auth",
					"description": "Authentication",
					"confidence": 0.9,
					"features": [
						{
							"name": "Login",
							"description": "Login feature",
							"confidence": 0.85,
							"test_indices": [0, 1]
						},
						{
							"name": "Logout",
							"description": "Logout feature",
							"confidence": 0.88,
							"test_indices": [2]
						}
					]
				},
				{
					"name": "User",
					"description": "User management",
					"confidence": 0.92,
					"features": [
						{
							"name": "Profile",
							"description": "User profile",
							"confidence": 0.9,
							"test_indices": [3, 4, 5]
						}
					]
				}
			]
		}`

		output, err := parsePhase1JSON(jsonStr)
		if err != nil {
			t.Fatalf("parsePhase1JSON() error = %v", err)
		}

		if len(output.Domains) != 2 {
			t.Errorf("len(Domains) = %d, expected 2", len(output.Domains))
		}

		// Verify first domain
		if output.Domains[0].Name != "Auth" {
			t.Errorf("Domains[0].Name = %q, expected %q", output.Domains[0].Name, "Auth")
		}
		if len(output.Domains[0].Features) != 2 {
			t.Errorf("len(Domains[0].Features) = %d, expected 2", len(output.Domains[0].Features))
		}

		// Verify second domain
		if output.Domains[1].Name != "User" {
			t.Errorf("Domains[1].Name = %q, expected %q", output.Domains[1].Name, "User")
		}
	})

	t.Run("should return error for invalid JSON", func(t *testing.T) {
		_, err := parsePhase1JSON(`{invalid`)
		if !errors.Is(err, ErrInvalidJSON) {
			t.Errorf("error = %v, want ErrInvalidJSON", err)
		}
	})

	t.Run("should handle empty domains array", func(t *testing.T) {
		output, err := parsePhase1JSON(`{"domains": []}`)
		if err != nil {
			t.Fatalf("parsePhase1JSON() error = %v", err)
		}

		if len(output.Domains) != 0 {
			t.Errorf("len(Domains) = %d, expected 0", len(output.Domains))
		}
	})
}

func TestValidatePhase1Output(t *testing.T) {
	t.Run("should validate correct output", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{
			"domains": [
				{
					"name": "Test",
					"description": "Test domain",
					"confidence": 0.9,
					"features": [
						{
							"name": "Feature1",
							"description": "Test feature",
							"confidence": 0.85,
							"test_indices": [0, 1, 2]
						}
					]
				}
			]
		}`)

		expected := map[int]bool{0: true, 1: true, 2: true}
		err := ValidatePhase1Output(output, expected)
		if err != nil {
			t.Errorf("ValidatePhase1Output() error = %v", err)
		}
	})

	t.Run("should return error for nil output", func(t *testing.T) {
		err := ValidatePhase1Output(nil, map[int]bool{0: true})
		if err == nil {
			t.Error("expected error for nil output")
		}
	})

	t.Run("should return error for empty domains", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{"domains": []}`)
		err := ValidatePhase1Output(output, map[int]bool{0: true})
		if err == nil {
			t.Error("expected error for empty domains")
		}
	})

	t.Run("should return error for empty domain name", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{
			"domains": [
				{
					"name": "",
					"description": "Test",
					"confidence": 0.9,
					"features": []
				}
			]
		}`)

		err := ValidatePhase1Output(output, map[int]bool{})
		if err == nil {
			t.Error("expected error for empty domain name")
		}
	})

	t.Run("should return error for empty feature name", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{
			"domains": [
				{
					"name": "Test",
					"description": "Test",
					"confidence": 0.9,
					"features": [
						{
							"name": "",
							"description": "Test feature",
							"confidence": 0.85,
							"test_indices": []
						}
					]
				}
			]
		}`)

		err := ValidatePhase1Output(output, map[int]bool{})
		if err == nil {
			t.Error("expected error for empty feature name")
		}
	})

	t.Run("should return error for unexpected test index", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{
			"domains": [
				{
					"name": "Test",
					"description": "Test",
					"confidence": 0.9,
					"features": [
						{
							"name": "Feature1",
							"description": "Test feature",
							"confidence": 0.85,
							"test_indices": [0, 99]
						}
					]
				}
			]
		}`)

		expected := map[int]bool{0: true, 1: true}
		err := ValidatePhase1Output(output, expected)
		if err == nil {
			t.Error("expected error for unexpected test index")
		}
	})

	t.Run("should return error for negative test index", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{
			"domains": [
				{
					"name": "Test",
					"description": "Test",
					"confidence": 0.9,
					"features": [
						{
							"name": "Feature1",
							"description": "Test feature",
							"confidence": 0.85,
							"test_indices": [0, -1]
						}
					]
				}
			]
		}`)

		expected := map[int]bool{0: true, 1: true}
		err := ValidatePhase1Output(output, expected)
		if err == nil {
			t.Error("expected error for negative test index")
		}
	})
}

func TestCountCoveredIndices(t *testing.T) {
	t.Run("should count unique indices", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{
			"domains": [
				{
					"name": "Domain1",
					"description": "Test",
					"confidence": 0.9,
					"features": [
						{
							"name": "Feature1",
							"description": "Test",
							"confidence": 0.85,
							"test_indices": [0, 1, 2]
						},
						{
							"name": "Feature2",
							"description": "Test",
							"confidence": 0.85,
							"test_indices": [2, 3]
						}
					]
				}
			]
		}`)

		count := CountCoveredIndices(output)
		if count != 4 {
			t.Errorf("CountCoveredIndices() = %d, expected 4", count)
		}
	})

	t.Run("should handle empty output", func(t *testing.T) {
		output, _ := parsePhase1JSON(`{"domains": []}`)
		count := CountCoveredIndices(output)
		if count != 0 {
			t.Errorf("CountCoveredIndices() = %d, expected 0", count)
		}
	})

	t.Run("should handle nil output", func(t *testing.T) {
		count := CountCoveredIndices(nil)
		if count != 0 {
			t.Errorf("CountCoveredIndices(nil) = %d, expected 0", count)
		}
	})
}

func TestExtractResponseText(t *testing.T) {
	t.Run("should use first text part only to avoid JSON corruption", func(t *testing.T) {
		// Batch API may return multiple parts that produce invalid JSON when concatenated
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Parts: []*genai.Part{
							{Text: `{"domains":[]}`},
							{Text: `[extra data]`}, // This would corrupt JSON if concatenated
						},
					},
				},
			},
		}

		text, err := extractResponseText(resp)
		if err != nil {
			t.Fatalf("extractResponseText() error = %v", err)
		}
		if text != `{"domains":[]}` {
			t.Errorf("text = %q, expected first part only", text)
		}
	})

	t.Run("should return error for nil response", func(t *testing.T) {
		_, err := extractResponseText(nil)
		if !errors.Is(err, ErrEmptyResponse) {
			t.Errorf("error = %v, want ErrEmptyResponse", err)
		}
	})

	t.Run("should return error for empty text", func(t *testing.T) {
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Parts: []*genai.Part{
							{Text: ""},
						},
					},
				},
			},
		}

		_, err := extractResponseText(resp)
		if !errors.Is(err, ErrEmptyResponse) {
			t.Errorf("error = %v, want ErrEmptyResponse", err)
		}
	})
}
