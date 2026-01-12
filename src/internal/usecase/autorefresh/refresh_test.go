package autorefresh

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/specvital/worker/internal/domain/analysis"
)

type mockAutoRefreshRepository struct {
	codebases []analysis.CodebaseRefreshInfo
	err       error
}

func (m *mockAutoRefreshRepository) GetCodebasesForAutoRefresh(ctx context.Context) ([]analysis.CodebaseRefreshInfo, error) {
	return m.codebases, m.err
}

type mockTaskQueue struct {
	enqueuedTasks []struct {
		owner     string
		repo      string
		commitSHA string
	}
	err error
}

func (m *mockTaskQueue) EnqueueAnalysis(ctx context.Context, owner, repo, commitSHA string) error {
	if m.err != nil {
		return m.err
	}
	m.enqueuedTasks = append(m.enqueuedTasks, struct {
		owner     string
		repo      string
		commitSHA string
	}{owner, repo, commitSHA})
	return nil
}

type mockVCS struct {
	commitSHA string
	err       error
}

func (m *mockVCS) Clone(ctx context.Context, url string, token *string) (analysis.Source, error) {
	return nil, nil
}

func (m *mockVCS) GetHeadCommit(ctx context.Context, url string, token *string) (analysis.CommitInfo, error) {
	if m.err != nil {
		return analysis.CommitInfo{}, m.err
	}
	return analysis.CommitInfo{SHA: m.commitSHA, IsPrivate: false}, nil
}

type mockParserVersionProvider struct {
	version string
	err     error
}

func (m *mockParserVersionProvider) GetCurrentParserVersion(ctx context.Context) (string, error) {
	return m.version, m.err
}

func TestAutoRefreshUseCase_Execute_NoCodebases(t *testing.T) {
	repo := &mockAutoRefreshRepository{codebases: nil}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 0 {
		t.Errorf("expected no tasks enqueued, got %d", len(queue.enqueuedTasks))
	}
}

func TestAutoRefreshUseCase_Execute_RepositoryError(t *testing.T) {
	expectedErr := errors.New("database error")
	repo := &mockAutoRefreshRepository{err: expectedErr}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err == nil || !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestAutoRefreshUseCase_Execute_EnqueuesEligibleCodebases(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				ConsecutiveFailures: 0,
			},
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner2",
				Name:                "repo2",
				LastViewedAt:        now.Add(-2 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				ConsecutiveFailures: 0,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 2 {
		t.Errorf("expected 2 tasks enqueued, got %d", len(queue.enqueuedTasks))
	}
}

func TestAutoRefreshUseCase_Execute_SkipsRecentlyCompleted(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-1 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				ConsecutiveFailures: 0,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 0 {
		t.Errorf("expected 0 tasks enqueued (recently completed), got %d", len(queue.enqueuedTasks))
	}
}

func TestAutoRefreshUseCase_Execute_SkipsExcessiveFailures(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				ConsecutiveFailures: 5,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 0 {
		t.Errorf("expected 0 tasks enqueued (excessive failures), got %d", len(queue.enqueuedTasks))
	}
}

func TestAutoRefreshUseCase_Execute_SkipsSameCommitAndParserVersion(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				LastCommitSHA:       "abc123",
				LastParserVersion:   "v1.0.0",
				ConsecutiveFailures: 0,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 0 {
		t.Errorf("expected 0 tasks enqueued (same commit and parser version), got %d", len(queue.enqueuedTasks))
	}
}

func TestAutoRefreshUseCase_Execute_EnqueuesWhenCommitDiffers(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				LastCommitSHA:       "old-commit",
				LastParserVersion:   "v1.0.0",
				ConsecutiveFailures: 0,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "new-commit"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 1 {
		t.Errorf("expected 1 task enqueued, got %d", len(queue.enqueuedTasks))
	}
	if queue.enqueuedTasks[0].commitSHA != "new-commit" {
		t.Errorf("expected commit SHA 'new-commit', got %s", queue.enqueuedTasks[0].commitSHA)
	}
}

type errorOnFirstTaskQueue struct {
	callCount     int
	enqueuedTasks []struct {
		owner     string
		repo      string
		commitSHA string
	}
}

func (m *errorOnFirstTaskQueue) EnqueueAnalysis(ctx context.Context, owner, repo, commitSHA string) error {
	m.callCount++
	if m.callCount == 1 {
		return errors.New("enqueue error")
	}
	m.enqueuedTasks = append(m.enqueuedTasks, struct {
		owner     string
		repo      string
		commitSHA string
	}{owner, repo, commitSHA})
	return nil
}

func TestAutoRefreshUseCase_Execute_ContinuesOnEnqueueError(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				ConsecutiveFailures: 0,
			},
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner2",
				Name:                "repo2",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				ConsecutiveFailures: 0,
			},
		},
	}

	queue := &errorOnFirstTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)
	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 1 {
		t.Errorf("expected 1 task enqueued (second one after error), got %d", len(queue.enqueuedTasks))
	}
}

type alwaysFailingTaskQueue struct {
	callCount int
}

func (m *alwaysFailingTaskQueue) EnqueueAnalysis(ctx context.Context, owner, repo, commitSHA string) error {
	m.callCount++
	return errors.New("enqueue error")
}

func TestAutoRefreshUseCase_Execute_CircuitBreakerTriggered(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{Owner: "owner1", Name: "repo1", LastViewedAt: now.Add(-1 * 24 * time.Hour), LastCompletedAt: &completedAt},
			{Owner: "owner2", Name: "repo2", LastViewedAt: now.Add(-1 * 24 * time.Hour), LastCompletedAt: &completedAt},
			{Owner: "owner3", Name: "repo3", LastViewedAt: now.Add(-1 * 24 * time.Hour), LastCompletedAt: &completedAt},
			{Owner: "owner4", Name: "repo4", LastViewedAt: now.Add(-1 * 24 * time.Hour), LastCompletedAt: &completedAt},
			{Owner: "owner5", Name: "repo5", LastViewedAt: now.Add(-1 * 24 * time.Hour), LastCompletedAt: &completedAt},
		},
	}

	queue := &alwaysFailingTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v1.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)
	err := uc.Execute(context.Background())

	if err == nil {
		t.Error("expected circuit breaker error, got nil")
	}
	if !errors.Is(err, ErrCircuitBreakerOpen) {
		t.Errorf("expected ErrCircuitBreakerOpen, got %v", err)
	}
	if queue.callCount != 3 {
		t.Errorf("expected 3 attempts before circuit breaker, got %d", queue.callCount)
	}
}

func TestAutoRefreshUseCase_Execute_EnqueuesWhenParserVersionDiffers(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				LastCommitSHA:       "abc123",
				LastParserVersion:   "v1.0.0",
				ConsecutiveFailures: 0,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v2.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 1 {
		t.Errorf("expected 1 task enqueued (parser version changed), got %d", len(queue.enqueuedTasks))
	}
}

func TestAutoRefreshUseCase_Execute_SkipsWhenParserVersionProviderFails(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				LastCommitSHA:       "abc123",
				LastParserVersion:   "v1.0.0",
				ConsecutiveFailures: 0,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{err: errors.New("config not found")}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 0 {
		t.Errorf("expected 0 tasks enqueued (parser version check skipped), got %d", len(queue.enqueuedTasks))
	}
}

func TestAutoRefreshUseCase_Execute_EnqueuesLegacyAnalysisWhenVersionChanged(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-7 * time.Hour)

	repo := &mockAutoRefreshRepository{
		codebases: []analysis.CodebaseRefreshInfo{
			{
				ID:                  analysis.UUID{},
				Host:                "github.com",
				Owner:               "owner1",
				Name:                "repo1",
				LastViewedAt:        now.Add(-1 * 24 * time.Hour),
				LastCompletedAt:     &completedAt,
				LastCommitSHA:       "abc123",
				LastParserVersion:   "legacy",
				ConsecutiveFailures: 0,
			},
		},
	}
	queue := &mockTaskQueue{}
	vcs := &mockVCS{commitSHA: "abc123"}
	pvp := &mockParserVersionProvider{version: "v2.0.0"}
	uc := NewAutoRefreshUseCase(repo, queue, vcs, pvp)

	err := uc.Execute(context.Background())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(queue.enqueuedTasks) != 1 {
		t.Errorf("expected 1 task enqueued (legacy analysis re-analyzed), got %d", len(queue.enqueuedTasks))
	}
}
