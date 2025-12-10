package service

import (
	"context"
	"errors"
	"testing"

	"github.com/specvital/collector/internal/repository"
	"github.com/specvital/collector/internal/repository/mocks"
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

func TestNewAnalysisService(t *testing.T) {
	mockRepo := &mocks.MockAnalysisRepository{}
	svc := NewAnalysisService(mockRepo)
	if svc == nil {
		t.Error("expected service, got nil")
	}
}

func TestAnalysisService_Analyze_ValidationError(t *testing.T) {
	mockRepo := &mocks.MockAnalysisRepository{}
	svc := NewAnalysisService(mockRepo)

	tests := []struct {
		name string
		req  AnalyzeRequest
	}{
		{
			name: "empty owner",
			req:  AnalyzeRequest{Owner: "", Repo: "repo"},
		},
		{
			name: "empty repo",
			req:  AnalyzeRequest{Owner: "owner", Repo: ""},
		},
		{
			name: "SSRF attempt with slash",
			req:  AnalyzeRequest{Owner: "evil.com/foo", Repo: "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.Analyze(context.Background(), tt.req)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestAnalysisService_Analyze_CloneFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent test in short mode")
	}

	mockRepo := &mocks.MockAnalysisRepository{}
	svc := NewAnalysisService(mockRepo)

	err := svc.Analyze(context.Background(), AnalyzeRequest{
		Owner: "nonexistent-owner-12345",
		Repo:  "nonexistent-repo-67890",
	})

	if !errors.Is(err, ErrCloneFailed) {
		t.Errorf("expected ErrCloneFailed, got %v", err)
	}
}

func TestAnalysisService_Analyze_SaveFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent test in short mode")
	}

	saveErr := errors.New("db error")
	mockRepo := &mocks.MockAnalysisRepository{
		SaveAnalysisResultFunc: func(ctx context.Context, params repository.SaveAnalysisResultParams) error {
			return saveErr
		},
	}
	svc := NewAnalysisService(mockRepo)

	err := svc.Analyze(context.Background(), AnalyzeRequest{
		Owner: "octocat",
		Repo:  "Hello-World",
	})

	if !errors.Is(err, ErrSaveFailed) {
		t.Errorf("expected ErrSaveFailed, got %v", err)
	}
}
