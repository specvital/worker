package fairness

import (
	"strings"
	"testing"
)

func TestJSONArgsExtractor_ExtractUserID(t *testing.T) {
	extractor := &JSONArgsExtractor{}

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
