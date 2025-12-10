package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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

	recordFailureCalled := false
	saveErr := errors.New("db error")
	mockRepo := &mocks.MockAnalysisRepository{
		SaveAnalysisInventoryFunc: func(ctx context.Context, params repository.SaveAnalysisInventoryParams) error {
			return saveErr
		},
		RecordFailureFunc: func(ctx context.Context, analysisID pgtype.UUID, errMessage string) error {
			recordFailureCalled = true
			if errMessage != saveErr.Error() {
				t.Errorf("expected error message %q, got %q", saveErr.Error(), errMessage)
			}
			return nil
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

	if !recordFailureCalled {
		t.Error("expected RecordFailure to be called on save failure")
	}
}

func TestNewAnalysisService_WithMaxConcurrentClones(t *testing.T) {
	mockRepo := &mocks.MockAnalysisRepository{}

	svc := NewAnalysisService(mockRepo, WithMaxConcurrentClones(5))
	if svc == nil {
		t.Error("expected service, got nil")
	}

	svcZero := NewAnalysisService(mockRepo, WithMaxConcurrentClones(0))
	if svcZero == nil {
		t.Error("expected service with default config when 0, got nil")
	}

	svcNegative := NewAnalysisService(mockRepo, WithMaxConcurrentClones(-1))
	if svcNegative == nil {
		t.Error("expected service with default config when negative, got nil")
	}
}

func TestNewAnalysisService_WithAnalysisTimeout(t *testing.T) {
	mockRepo := &mocks.MockAnalysisRepository{}

	svc := NewAnalysisService(mockRepo, WithAnalysisTimeout(5*time.Minute))
	if svc == nil {
		t.Error("expected service, got nil")
	}

	svcZero := NewAnalysisService(mockRepo, WithAnalysisTimeout(0))
	if svcZero == nil {
		t.Error("expected service with default config when 0, got nil")
	}

	svcNegative := NewAnalysisService(mockRepo, WithAnalysisTimeout(-1*time.Minute))
	if svcNegative == nil {
		t.Error("expected service with default config when negative, got nil")
	}
}

func TestAnalysisService_Analyze_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network-dependent test in short mode")
	}

	mockRepo := &mocks.MockAnalysisRepository{}

	// Use very short timeout to trigger deadline during clone
	svc := NewAnalysisService(mockRepo, WithAnalysisTimeout(1*time.Nanosecond))

	err := svc.Analyze(context.Background(), AnalyzeRequest{
		Owner: "octocat",
		Repo:  "Hello-World",
	})

	// Should fail with ErrCloneFailed wrapping context.DeadlineExceeded
	if !errors.Is(err, ErrCloneFailed) {
		t.Errorf("expected ErrCloneFailed, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded in error chain, got %v", err)
	}
}
