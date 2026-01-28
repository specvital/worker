package fairness

import (
	"strings"
	"testing"
)

func TestJSONArgsExtractor_ExtractUserID(t *testing.T) {
	extractor := NewJSONArgsExtractor()

	tests := []struct {
		name     string
		args     []byte
		expected string
	}{
		{
			name:     "specview args with user_id",
			args:     []byte(`{"analysis_id":"123","user_id":"user-abc","language":"English"}`),
			expected: "user-abc",
		},
		{
			name:     "analysis args with user_id",
			args:     []byte(`{"commit_sha":"abc123","owner":"octocat","repo":"hello","user_id":"user-xyz"}`),
			expected: "user-xyz",
		},
		{
			name:     "analysis args with null user_id",
			args:     []byte(`{"commit_sha":"abc123","owner":"octocat","repo":"hello","user_id":null}`),
			expected: "",
		},
		{
			name:     "missing user_id field",
			args:     []byte(`{"commit_sha":"abc123","owner":"octocat","repo":"hello"}`),
			expected: "",
		},
		{
			name:     "empty user_id string",
			args:     []byte(`{"user_id":""}`),
			expected: "",
		},
		{
			name:     "invalid json",
			args:     []byte(`{invalid json}`),
			expected: "",
		},
		{
			name:     "empty byte array",
			args:     []byte{},
			expected: "",
		},
		{
			name:     "oversized json exceeds limit",
			args:     []byte(`{"user_id":"user-abc","data":"` + strings.Repeat("x", 70*1024) + `"}`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.ExtractUserID(tt.args)
			if result != tt.expected {
				t.Errorf("ExtractUserID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestJSONArgsExtractor_ExtractTier(t *testing.T) {
	extractor := NewJSONArgsExtractor()

	tests := []struct {
		name     string
		args     []byte
		expected PlanTier
	}{
		{
			name:     "free tier",
			args:     []byte(`{"user_id":"user-abc","tier":"free"}`),
			expected: TierFree,
		},
		{
			name:     "pro tier",
			args:     []byte(`{"user_id":"user-abc","tier":"pro"}`),
			expected: TierPro,
		},
		{
			name:     "pro_plus tier",
			args:     []byte(`{"user_id":"user-abc","tier":"pro_plus"}`),
			expected: TierProPlus,
		},
		{
			name:     "enterprise tier",
			args:     []byte(`{"user_id":"user-abc","tier":"enterprise"}`),
			expected: TierEnterprise,
		},
		{
			name:     "missing tier defaults to free",
			args:     []byte(`{"user_id":"user-abc"}`),
			expected: TierFree,
		},
		{
			name:     "empty tier defaults to free",
			args:     []byte(`{"user_id":"user-abc","tier":""}`),
			expected: TierFree,
		},
		{
			name:     "invalid json defaults to free",
			args:     []byte(`{invalid json}`),
			expected: TierFree,
		},
		{
			name:     "empty byte array defaults to free",
			args:     []byte{},
			expected: TierFree,
		},
		{
			name:     "unknown tier defaults to free",
			args:     []byte(`{"user_id":"user-abc","tier":"premium"}`),
			expected: TierFree,
		},
		{
			name:     "invalid tier defaults to free",
			args:     []byte(`{"user_id":"user-abc","tier":"unknown"}`),
			expected: TierFree,
		},
		{
			name:     "oversized json exceeds limit",
			args:     []byte(`{"tier":"free","data":"` + strings.Repeat("x", 70*1024) + `"}`),
			expected: TierFree,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.ExtractTier(tt.args)
			if result != tt.expected {
				t.Errorf("ExtractTier() = %q, want %q", result, tt.expected)
			}
		})
	}
}
