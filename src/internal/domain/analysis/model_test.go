package analysis

import (
	"errors"
	"testing"
)

func TestAnalyzeRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     AnalyzeRequest
		wantErr error
	}{
		{
			name:    "valid request",
			req:     AnalyzeRequest{Owner: "owner", Repo: "repo"},
			wantErr: nil,
		},
		{
			name:    "valid with dot",
			req:     AnalyzeRequest{Owner: "owner.name", Repo: "repo.js"},
			wantErr: nil,
		},
		{
			name:    "valid with dash and underscore",
			req:     AnalyzeRequest{Owner: "my-org", Repo: "my_repo"},
			wantErr: nil,
		},
		{
			name:    "valid with mixed case",
			req:     AnalyzeRequest{Owner: "MyOrg", Repo: "MyRepo"},
			wantErr: nil,
		},
		{
			name:    "valid with numbers",
			req:     AnalyzeRequest{Owner: "org123", Repo: "repo456"},
			wantErr: nil,
		},
		{
			name:    "empty owner",
			req:     AnalyzeRequest{Owner: "", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "empty repo",
			req:     AnalyzeRequest{Owner: "owner", Repo: ""},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner exceeds length limit",
			req:     AnalyzeRequest{Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "repo exceeds length limit",
			req:     AnalyzeRequest{Owner: "owner", Repo: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with slash (SSRF)",
			req:     AnalyzeRequest{Owner: "evil/path", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "repo with at sign (SSRF)",
			req:     AnalyzeRequest{Owner: "owner", Repo: "repo@evil"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with colon (SSRF)",
			req:     AnalyzeRequest{Owner: "evil:8080", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with hash (SSRF)",
			req:     AnalyzeRequest{Owner: "evil#fragment", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "repo with query (SSRF)",
			req:     AnalyzeRequest{Owner: "owner", Repo: "repo?query=1"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with space",
			req:     AnalyzeRequest{Owner: "evil owner", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "repo with semicolon",
			req:     AnalyzeRequest{Owner: "owner", Repo: "repo;cmd"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with ampersand",
			req:     AnalyzeRequest{Owner: "owner&cmd", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with path traversal (..)",
			req:     AnalyzeRequest{Owner: "..", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "repo with path traversal (..)",
			req:     AnalyzeRequest{Owner: "owner", Repo: ".."},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with embedded path traversal",
			req:     AnalyzeRequest{Owner: "foo..bar", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
		{
			name:    "owner with single dot",
			req:     AnalyzeRequest{Owner: ".", Repo: "repo"},
			wantErr: ErrInvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if tt.wantErr == nil && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestIsValidGitHubName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid alphanumeric",
			input: "owner123",
			want:  true,
		},
		{
			name:  "valid with dash",
			input: "my-org",
			want:  true,
		},
		{
			name:  "valid with underscore",
			input: "my_repo",
			want:  true,
		},
		{
			name:  "valid with dot",
			input: "repo.js",
			want:  true,
		},
		{
			name:  "valid mixed",
			input: "My-Org_123.repo",
			want:  true,
		},
		{
			name:  "invalid with slash",
			input: "evil/path",
			want:  false,
		},
		{
			name:  "invalid with at",
			input: "evil@sign",
			want:  false,
		},
		{
			name:  "invalid with colon",
			input: "evil:port",
			want:  false,
		},
		{
			name:  "invalid with hash",
			input: "evil#fragment",
			want:  false,
		},
		{
			name:  "invalid with question",
			input: "evil?query",
			want:  false,
		},
		{
			name:  "invalid with space",
			input: "evil space",
			want:  false,
		},
		{
			name:  "invalid with semicolon",
			input: "evil;cmd",
			want:  false,
		},
		{
			name:  "invalid with ampersand",
			input: "evil&cmd",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "path traversal double dot",
			input: "..",
			want:  false,
		},
		{
			name:  "path traversal single dot",
			input: ".",
			want:  false,
		},
		{
			name:  "embedded path traversal",
			input: "foo..bar",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidGitHubName(tt.input)
			if got != tt.want {
				t.Errorf("isValidGitHubName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
