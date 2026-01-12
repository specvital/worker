package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/specvital/worker/internal/domain/analysis"
)

const (
	DefaultMaxConcurrentClones = 2
	DefaultAnalysisTimeout     = 15 * time.Minute
	// DefaultOAuthProvider is the OAuth provider for VCS authentication.
	// Currently only GitHub is supported as the VCS provider (see repoURL construction in Execute).
	DefaultOAuthProvider = "github"
	DefaultHost          = "github.com"
)

// AnalyzeUseCase orchestrates repository analysis workflow.
type AnalyzeUseCase struct {
	cloneSem      *semaphore.Weighted
	codebaseRepo  analysis.CodebaseRepository
	parser        analysis.Parser
	parserVersion string
	repository    analysis.Repository
	timeout       time.Duration
	tokenLookup   analysis.TokenLookup
	vcs           analysis.VCS
	vcsAPIClient  analysis.VCSAPIClient
}

// Config holds configuration for AnalyzeUseCase.
type Config struct {
	AnalysisTimeout     time.Duration
	MaxConcurrentClones int64
	ParserVersion       string
}

// Option is a functional option for configuring AnalyzeUseCase.
type Option func(*Config)

// WithAnalysisTimeout sets the timeout for analysis operations.
// Zero or negative values are ignored and the default timeout is used.
func WithAnalysisTimeout(d time.Duration) Option {
	return func(cfg *Config) {
		if d > 0 {
			cfg.AnalysisTimeout = d
		}
	}
}

// WithMaxConcurrentClones sets the maximum number of concurrent clone operations.
// Zero or negative values are ignored and the default value is used.
func WithMaxConcurrentClones(n int64) Option {
	return func(cfg *Config) {
		if n > 0 {
			cfg.MaxConcurrentClones = n
		}
	}
}

// WithParserVersion sets the parser version to be recorded with each analysis.
// This should be set to the core module version extracted at startup.
func WithParserVersion(v string) Option {
	return func(cfg *Config) {
		cfg.ParserVersion = v
	}
}

// NewAnalyzeUseCase creates a new AnalyzeUseCase with given dependencies.
// tokenLookup is optional - if nil, all clones use public access (token=nil).
func NewAnalyzeUseCase(
	repository analysis.Repository,
	codebaseRepo analysis.CodebaseRepository,
	vcs analysis.VCS,
	vcsAPIClient analysis.VCSAPIClient,
	parser analysis.Parser,
	tokenLookup analysis.TokenLookup,
	opts ...Option,
) *AnalyzeUseCase {
	cfg := Config{
		AnalysisTimeout:     DefaultAnalysisTimeout,
		MaxConcurrentClones: DefaultMaxConcurrentClones,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &AnalyzeUseCase{
		cloneSem:      semaphore.NewWeighted(cfg.MaxConcurrentClones),
		codebaseRepo:  codebaseRepo,
		parser:        parser,
		parserVersion: cfg.ParserVersion,
		repository:    repository,
		timeout:       cfg.AnalysisTimeout,
		tokenLookup:   tokenLookup,
		vcs:           vcs,
		vcsAPIClient:  vcsAPIClient,
	}
}

func (uc *AnalyzeUseCase) Execute(ctx context.Context, req analysis.AnalyzeRequest) (err error) {
	if err = req.Validate(); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, uc.timeout)
	defer cancel()

	repoURL := fmt.Sprintf("https://github.com/%s/%s", req.Owner, req.Repo)

	token, err := uc.lookupToken(timeoutCtx, req.UserID)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTokenLookupFailed, err)
	}

	commitInfo, err := uc.vcs.GetHeadCommit(timeoutCtx, repoURL, token)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHeadCommitFailed, err)
	}

	src, err := uc.cloneWithSemaphore(timeoutCtx, repoURL, token)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCloneFailed, err)
	}
	defer uc.closeSource(src, req.Owner, req.Repo)

	codebase, err := uc.resolveCodebase(timeoutCtx, req, src, token, commitInfo.IsPrivate)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCodebaseResolutionFailed, err)
	}

	createParams := analysis.CreateAnalysisRecordParams{
		Branch:         src.Branch(),
		CodebaseID:     &codebase.ID,
		CommitSHA:      src.CommitSHA(),
		ExternalRepoID: codebase.ExternalRepoID,
		Owner:          codebase.Owner,
		ParserVersion:  uc.parserVersion,
		Repo:           codebase.Name,
	}
	if err = createParams.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrSaveFailed, err)
	}

	analysisID, err := uc.repository.CreateAnalysisRecord(timeoutCtx, createParams)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSaveFailed, err)
	}

	defer func() {
		if err != nil {
			if recordErr := uc.repository.RecordFailure(context.Background(), analysisID, err.Error()); recordErr != nil {
				slog.ErrorContext(context.Background(), "failed to record analysis failure",
					"error", recordErr,
					"analysis_id", analysisID,
					"original_error", err,
				)
			}
		}
	}()

	inventory, err := uc.parser.Scan(timeoutCtx, src)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrScanFailed, err)
		return err
	}

	if inventory == nil {
		slog.WarnContext(ctx, "scan result has no inventory",
			"owner", req.Owner,
			"repo", req.Repo,
			"commit", src.CommitSHA(),
		)
		inventory = &analysis.Inventory{Files: []analysis.TestFile{}}
	}

	saveParams := analysis.SaveAnalysisInventoryParams{
		AnalysisID:  analysisID,
		CommittedAt: src.CommittedAt(),
		Inventory:   inventory,
		UserID:      req.UserID,
	}
	if err = saveParams.Validate(); err != nil {
		err = fmt.Errorf("%w: %w", ErrSaveFailed, err)
		return err
	}

	if err = uc.repository.SaveAnalysisInventory(timeoutCtx, saveParams); err != nil {
		err = fmt.Errorf("%w: %w", ErrSaveFailed, err)
		return err
	}

	return nil
}

// resolveCodebase determines which codebase to use for the analysis request.
//
// Resolution strategy uses external_repo_id as source of truth:
//   - Case A: New analysis - no codebase exists, create new
//   - Case B: Reanalysis - codebase exists, git fetch verifies same repo (API-free)
//   - Case D: Rename/Transfer - different owner/name but same external_repo_id
//   - Case E: Delete and recreate - same owner/name but different external_repo_id
//   - Case F: Force push - git fetch fails but same external_repo_id
func (uc *AnalyzeUseCase) resolveCodebase(
	ctx context.Context,
	req analysis.AnalyzeRequest,
	src analysis.Source,
	token *string,
	isPrivate bool,
) (*analysis.Codebase, error) {
	host := DefaultHost

	codebase, err := uc.codebaseRepo.FindWithLastCommit(ctx, host, req.Owner, req.Repo)
	if err != nil && !errors.Is(err, analysis.ErrCodebaseNotFound) {
		return nil, fmt.Errorf("find codebase for %s/%s: %w", req.Owner, req.Repo, err)
	}

	if codebase != nil {
		if codebase.LastCommitSHA != "" {
			verified, verifyErr := src.VerifyCommitExists(ctx, codebase.LastCommitSHA)
			if verifyErr != nil {
				slog.WarnContext(ctx, "commit verification failed, falling back to API",
					"error", verifyErr,
					"owner", req.Owner,
					"repo", req.Repo,
					"last_commit_sha", codebase.LastCommitSHA,
				)
			} else if verified {
				if err := uc.codebaseRepo.UpdateVisibility(ctx, codebase.ID, isPrivate); err != nil {
					slog.WarnContext(ctx, "failed to update visibility",
						"error", err,
						"codebase_id", codebase.ID,
						"is_private", isPrivate,
					)
				}
				slog.InfoContext(ctx, "codebase resolved",
					"case", "reanalysis",
					"owner", req.Owner,
					"repo", req.Repo,
					"codebase_id", codebase.ID,
				)
				return codebase, nil
			}
		}
	}

	return uc.resolveCodebaseWithAPI(ctx, host, req, codebase, token, isPrivate)
}

func (uc *AnalyzeUseCase) resolveCodebaseWithAPI(
	ctx context.Context,
	host string,
	req analysis.AnalyzeRequest,
	codebaseByName *analysis.Codebase,
	token *string,
	isPrivate bool,
) (*analysis.Codebase, error) {
	repoInfo, err := uc.vcsAPIClient.GetRepoInfo(ctx, host, req.Owner, req.Repo, token)
	if err != nil {
		if errors.Is(err, analysis.ErrRepoNotFound) {
			return nil, fmt.Errorf("repository not found %s/%s: %w", req.Owner, req.Repo, err)
		}
		return nil, fmt.Errorf("get repo info for %s/%s: %w", req.Owner, req.Repo, err)
	}

	if !strings.EqualFold(repoInfo.Owner, req.Owner) || !strings.EqualFold(repoInfo.Name, req.Repo) {
		slog.WarnContext(ctx, "race condition detected: repository renamed during clone",
			"requested_owner", req.Owner,
			"requested_repo", req.Repo,
			"actual_owner", repoInfo.Owner,
			"actual_repo", repoInfo.Name,
		)
		return nil, fmt.Errorf(
			"%w: repository state changed during analysis",
			ErrRaceConditionDetected,
		)
	}

	externalRepoID := repoInfo.ExternalRepoID
	codebaseByID, err := uc.codebaseRepo.FindByExternalID(ctx, host, externalRepoID)
	if err != nil && !errors.Is(err, analysis.ErrCodebaseNotFound) {
		return nil, fmt.Errorf("find by external ID %s for %s/%s: %w", externalRepoID, req.Owner, req.Repo, err)
	}

	if codebaseByID != nil {
		if codebaseByID.IsStale {
			updated, updateErr := uc.codebaseRepo.UnmarkStale(ctx, codebaseByID.ID, req.Owner, req.Repo)
			if updateErr != nil {
				return nil, fmt.Errorf("unmark stale for %s/%s: %w", req.Owner, req.Repo, updateErr)
			}
			slog.InfoContext(ctx, "codebase resolved",
				"case", "repo_restored",
				"owner", req.Owner,
				"repo", req.Repo,
				"codebase_id", updated.ID,
			)
			return updated, nil
		}

		if codebaseByID.Owner != req.Owner || codebaseByID.Name != req.Repo {
			updated, updateErr := uc.codebaseRepo.UpdateOwnerName(ctx, codebaseByID.ID, req.Owner, req.Repo)
			if updateErr != nil {
				return nil, fmt.Errorf("update owner/name for %s/%s: %w", req.Owner, req.Repo, updateErr)
			}
			slog.InfoContext(ctx, "codebase resolved",
				"case", "rename_transfer",
				"owner", req.Owner,
				"repo", req.Repo,
				"codebase_id", updated.ID,
				"old_owner", codebaseByID.Owner,
				"old_name", codebaseByID.Name,
			)
			return updated, nil
		}

		slog.InfoContext(ctx, "codebase resolved",
			"case", "force_push",
			"owner", req.Owner,
			"repo", req.Repo,
			"codebase_id", codebaseByID.ID,
		)
		return codebaseByID, nil
	}

	upsertParams := analysis.UpsertCodebaseParams{
		Host:           host,
		Owner:          req.Owner,
		Name:           req.Repo,
		ExternalRepoID: externalRepoID,
		IsPrivate:      isPrivate,
	}

	if codebaseByName != nil && codebaseByName.ExternalRepoID != externalRepoID {
		newCodebase, err := uc.codebaseRepo.MarkStaleAndUpsert(ctx, codebaseByName.ID, upsertParams)
		if err != nil {
			return nil, fmt.Errorf("mark stale and upsert for %s/%s: %w", req.Owner, req.Repo, err)
		}
		slog.InfoContext(ctx, "codebase resolved",
			"case", "delete_recreate",
			"owner", req.Owner,
			"repo", req.Repo,
			"codebase_id", newCodebase.ID,
			"old_codebase_id", codebaseByName.ID,
		)
		return newCodebase, nil
	}

	newCodebase, err := uc.codebaseRepo.Upsert(ctx, upsertParams)
	if err != nil {
		return nil, fmt.Errorf("upsert codebase for %s/%s: %w", req.Owner, req.Repo, err)
	}

	slog.InfoContext(ctx, "codebase resolved",
		"case", "new",
		"owner", req.Owner,
		"repo", req.Repo,
		"codebase_id", newCodebase.ID,
	)
	return newCodebase, nil
}

func (uc *AnalyzeUseCase) cloneWithSemaphore(ctx context.Context, url string, token *string) (analysis.Source, error) {
	if err := uc.cloneSem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer uc.cloneSem.Release(1)

	return uc.vcs.Clone(ctx, url, token)
}

// lookupToken retrieves OAuth token for the given user.
//
// Returns:
//   - (nil, nil): no userID provided, tokenLookup not configured, or token not found (graceful degradation)
//   - (*token, nil): token found successfully
//   - (nil, error): infrastructure error (should fail the operation)
//
// Token not found (analysis.ErrTokenNotFound) triggers graceful degradation and is logged at INFO level.
// Infrastructure errors are returned to fail the operation.
func (uc *AnalyzeUseCase) lookupToken(ctx context.Context, userID *string) (*string, error) {
	if userID == nil || uc.tokenLookup == nil {
		return nil, nil
	}

	token, err := uc.tokenLookup.GetOAuthToken(ctx, *userID, DefaultOAuthProvider)
	if err != nil {
		if errors.Is(err, analysis.ErrTokenNotFound) {
			slog.InfoContext(ctx, "no OAuth token found, using public access",
				"user_id", *userID,
			)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lookup OAuth token for user %s: %w", *userID, err)
	}

	if token == "" {
		slog.WarnContext(ctx, "empty token returned, using public access",
			"user_id", *userID,
		)
		return nil, nil
	}

	return &token, nil
}

func (uc *AnalyzeUseCase) closeSource(src analysis.Source, owner, repo string) {
	// Use background context for cleanup operations
	ctx := context.Background()
	if closeErr := src.Close(ctx); closeErr != nil {
		slog.Error("failed to close source",
			"error", closeErr,
			"owner", owner,
			"repo", repo,
		)
	}
}
