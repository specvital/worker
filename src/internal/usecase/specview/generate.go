package specview

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/specvital/worker/internal/domain/specview"
)

const (
	DefaultPhase1Timeout        = 270 * time.Second // 4m30s, below Gemini's 5min server limit
	DefaultPhase2Timeout        = 7 * time.Minute
	DefaultPhase2Concurrency    = int64(5)
	DefaultFailureThreshold     = 0.5 // 50% feature failure threshold
	DefaultPhase2FeatureTimeout = 90 * time.Second  // 1m30s for single feature conversion

	// Progress logging thresholds for Phase 2
	progressLogBatchSize     = 10               // Log every N completions
	progressLogTimeInterval  = 30 * time.Second // Log at least every 30 seconds
	progressLogMinFeatures   = 10               // Only log progress when total >= this
)

// Config holds configuration for GenerateSpecViewUseCase.
type Config struct {
	FailureThreshold  float64       // Threshold for partial failure (default: 0.5)
	Phase1Timeout     time.Duration // Timeout for Phase 1 (default: 2 minutes)
	Phase2Concurrency int64         // Max concurrent Phase 2 calls (default: 5)
	Phase2Timeout     time.Duration // Timeout for Phase 2 (default: 7 minutes)
}

// Option is a functional option for configuring GenerateSpecViewUseCase.
type Option func(*Config)

// WithPhase1Timeout sets the timeout for Phase 1.
func WithPhase1Timeout(d time.Duration) Option {
	return func(cfg *Config) {
		if d > 0 {
			cfg.Phase1Timeout = d
		}
	}
}

// WithPhase2Timeout sets the timeout for Phase 2.
func WithPhase2Timeout(d time.Duration) Option {
	return func(cfg *Config) {
		if d > 0 {
			cfg.Phase2Timeout = d
		}
	}
}

// WithPhase2Concurrency sets the max concurrent Phase 2 calls.
func WithPhase2Concurrency(n int64) Option {
	return func(cfg *Config) {
		if n > 0 {
			cfg.Phase2Concurrency = n
		}
	}
}

// WithFailureThreshold sets the failure threshold for partial failures.
func WithFailureThreshold(t float64) Option {
	return func(cfg *Config) {
		if t > 0 && t <= 1 {
			cfg.FailureThreshold = t
		}
	}
}

// GenerateSpecViewUseCase orchestrates spec-view document generation.
type GenerateSpecViewUseCase struct {
	aiProvider     specview.AIProvider
	config         Config
	defaultModelID string
	repository     specview.Repository
	phase2Sem      *semaphore.Weighted
}

// NewGenerateSpecViewUseCase creates a new GenerateSpecViewUseCase.
func NewGenerateSpecViewUseCase(
	repo specview.Repository,
	aiProvider specview.AIProvider,
	defaultModelID string,
	opts ...Option,
) *GenerateSpecViewUseCase {
	cfg := Config{
		FailureThreshold:  DefaultFailureThreshold,
		Phase1Timeout:     DefaultPhase1Timeout,
		Phase2Concurrency: DefaultPhase2Concurrency,
		Phase2Timeout:     DefaultPhase2Timeout,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &GenerateSpecViewUseCase{
		aiProvider:     aiProvider,
		config:         cfg,
		defaultModelID: defaultModelID,
		repository:     repo,
		phase2Sem:      semaphore.NewWeighted(cfg.Phase2Concurrency),
	}
}

// Execute generates a spec-view document for the given request.
func (uc *GenerateSpecViewUseCase) Execute(
	ctx context.Context,
	req specview.SpecViewRequest,
) (*specview.SpecViewResult, error) {
	startTime := time.Now()

	if err := req.Validate(); err != nil {
		return nil, err
	}

	analysisCtx, err := uc.repository.GetAnalysisContext(ctx, req.AnalysisID)
	if err != nil {
		return nil, err
	}

	modelID := req.ModelID
	if modelID == "" {
		modelID = uc.defaultModelID
	}

	files, err := uc.loadTestData(ctx, req.AnalysisID)
	if err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "load_data", startTime, err)
		return nil, err
	}

	if len(files) == 0 {
		slog.WarnContext(ctx, "no test files found",
			"analysis_id", req.AnalysisID,
			"owner", analysisCtx.Owner,
			"repo", analysisCtx.Repo,
		)
		return nil, fmt.Errorf("%w: no test files found for analysis", ErrLoadInventoryFailed)
	}

	contentHash := specview.GenerateContentHash(files, req.Language)

	if !req.ForceRegenerate {
		existingDoc, err := uc.repository.FindDocumentByContentHash(ctx, req.UserID, contentHash, req.Language, modelID)
		if err != nil {
			uc.logExecutionError(ctx, req.AnalysisID, "cache_check", startTime, err)
			return nil, fmt.Errorf("check cache: %w", err)
		}

		if existingDoc != nil {
			slog.InfoContext(ctx, "cache hit",
				"analysis_id", req.AnalysisID,
				"user_id", req.UserID,
				"owner", analysisCtx.Owner,
				"repo", analysisCtx.Repo,
				"document_id", existingDoc.ID,
			)

			uc.recordUserHistory(ctx, req.UserID, existingDoc.ID)

			return &specview.SpecViewResult{
				AnalysisContext: analysisCtx,
				CacheHit:        true,
				ContentHash:     contentHash,
				DocumentID:      existingDoc.ID,
			}, nil
		}
	}

	phase1Output, phase1Usage, err := uc.executePhase1(ctx, files, req.Language, req.AnalysisID)
	if err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "phase1", startTime, err)
		return nil, fmt.Errorf("%w: phase 1: %w", ErrAIProcessingFailed, err)
	}

	testIndexMap := buildTestIndexMap(files)

	phase2Results, phase2Usage, err := uc.executePhase2(ctx, phase1Output, req.Language, testIndexMap)
	if err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "phase2", startTime, err)
		return nil, fmt.Errorf("%w: phase 2: %w", ErrAIProcessingFailed, err)
	}

	doc := uc.assembleDocument(req, modelID, contentHash, phase1Output, phase2Results, testIndexMap)

	if err := uc.repository.SaveDocument(ctx, doc); err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "save", startTime, err)
		return nil, fmt.Errorf("%w: %w", ErrSaveFailed, err)
	}

	testCasesCount := countTotalTestCases(files)
	uc.recordUsageEvent(ctx, req.UserID, doc.ID, testCasesCount)
	uc.recordUserHistory(ctx, req.UserID, doc.ID)

	// Log token usage summary
	uc.logTokenUsage(ctx, req.AnalysisID, phase1Usage, phase2Usage)

	slog.InfoContext(ctx, "document generated",
		"analysis_id", req.AnalysisID,
		"user_id", req.UserID,
		"owner", analysisCtx.Owner,
		"repo", analysisCtx.Repo,
		"document_id", doc.ID,
		"domain_count", len(doc.Domains),
	)

	return &specview.SpecViewResult{
		AnalysisContext: analysisCtx,
		CacheHit:        false,
		ContentHash:     contentHash,
		DocumentID:      doc.ID,
	}, nil
}

func (uc *GenerateSpecViewUseCase) recordUsageEvent(
	ctx context.Context,
	userID string,
	documentID string,
	quotaAmount int,
) {
	if err := uc.repository.RecordUsageEvent(ctx, userID, documentID, quotaAmount); err != nil {
		slog.WarnContext(ctx, "failed to record usage event (non-critical)",
			"user_id", userID,
			"document_id", documentID,
			"quota_amount", quotaAmount,
			"error", err,
		)
	}
}

func (uc *GenerateSpecViewUseCase) recordUserHistory(
	ctx context.Context,
	userID string,
	documentID string,
) {
	if err := uc.repository.RecordUserHistory(ctx, userID, documentID); err != nil {
		slog.WarnContext(ctx, "failed to record user history (non-critical)",
			"user_id", userID,
			"document_id", documentID,
			"error", err,
		)
	}
}

func (uc *GenerateSpecViewUseCase) loadTestData(
	ctx context.Context,
	analysisID string,
) ([]specview.FileInfo, error) {
	files, err := uc.repository.GetTestDataByAnalysisID(ctx, analysisID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrLoadInventoryFailed, err)
	}
	return files, nil
}

func (uc *GenerateSpecViewUseCase) executePhase1(
	ctx context.Context,
	files []specview.FileInfo,
	lang specview.Language,
	analysisID string,
) (*specview.Phase1Output, *specview.TokenUsage, error) {
	startTime := time.Now()
	fileCount := len(files)
	testCount := countTotalTestCases(files)

	slog.InfoContext(ctx, "phase 1 started",
		"analysis_id", analysisID,
		"file_count", fileCount,
		"test_count", testCount,
	)

	phase1Ctx, cancel := context.WithTimeout(ctx, uc.config.Phase1Timeout)
	defer cancel()

	input := specview.Phase1Input{
		AnalysisID: analysisID,
		Files:      files,
		Language:   lang,
	}

	output, usage, err := uc.aiProvider.ClassifyDomains(phase1Ctx, input)
	if err != nil {
		return nil, nil, err
	}

	durationMs := time.Since(startTime).Milliseconds()
	slog.InfoContext(ctx, "phase 1 complete",
		"analysis_id", analysisID,
		"domain_count", len(output.Domains),
		"duration_ms", durationMs,
	)

	return output, usage, nil
}

type phase2Result struct {
	domainIdx   int
	featureIdx  int
	behaviors   []specview.BehaviorSpec
	failedCount int
	usage       *specview.TokenUsage
}

func (uc *GenerateSpecViewUseCase) executePhase2(
	ctx context.Context,
	phase1Output *specview.Phase1Output,
	lang specview.Language,
	testIndexMap map[int]specview.TestInfo,
) ([]phase2Result, *specview.TokenUsage, error) {
	startTime := time.Now()

	phase2Ctx, cancel := context.WithTimeout(ctx, uc.config.Phase2Timeout)
	defer cancel()

	var featureTasks []featureTask
	for di, domain := range phase1Output.Domains {
		for fi, feature := range domain.Features {
			featureTasks = append(featureTasks, featureTask{
				domainIdx:     di,
				domainContext: domain.Name + ": " + domain.Description,
				featureIdx:    fi,
				feature:       feature,
			})
		}
	}

	if len(featureTasks) == 0 {
		return nil, nil, nil
	}

	slog.InfoContext(ctx, "phase 2 started",
		"feature_count", len(featureTasks),
	)

	var (
		results   = make([]phase2Result, len(featureTasks))
		resultsMu sync.Mutex
		tracker   = newProgressTracker(len(featureTasks))
	)

	g, gCtx := errgroup.WithContext(phase2Ctx)

	for i, task := range featureTasks {
		g.Go(func() error {
			if err := uc.phase2Sem.Acquire(gCtx, 1); err != nil {
				return err
			}
			defer uc.phase2Sem.Release(1)

			behaviors, usage, failed := uc.convertFeature(gCtx, task, lang, testIndexMap)

			resultsMu.Lock()
			results[i] = phase2Result{
				domainIdx:   task.domainIdx,
				featureIdx:  task.featureIdx,
				behaviors:   behaviors,
				failedCount: failed,
				usage:       usage,
			}
			resultsMu.Unlock()

			tracker.recordCompletion(ctx, failed > 0)

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	failedCount := int(tracker.failed.Load())
	failureRate := float64(failedCount) / float64(len(featureTasks))
	if failureRate > uc.config.FailureThreshold {
		return nil, nil, fmt.Errorf("%w: %.0f%% features failed (threshold: %.0f%%)",
			ErrPartialFeatureFailure,
			failureRate*100,
			uc.config.FailureThreshold*100,
		)
	}

	// Aggregate Phase 2 token usage
	var aggregateUsage specview.TokenUsage
	for _, r := range results {
		if r.usage != nil {
			aggregateUsage = aggregateUsage.Add(*r.usage)
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	slog.InfoContext(ctx, "phase 2 complete",
		"feature_count", len(featureTasks),
		"failed_count", failedCount,
		"duration_ms", durationMs,
	)

	return results, &aggregateUsage, nil
}

type featureTask struct {
	domainContext string
	domainIdx     int
	feature       specview.FeatureGroup
	featureIdx    int
}

// progressTracker tracks Phase 2 progress and handles batch logging.
type progressTracker struct {
	completed   atomic.Int32
	failed      atomic.Int32
	lastLogTime atomic.Int64 // unix nano
	total       int32
}

func newProgressTracker(total int) *progressTracker {
	pt := &progressTracker{total: int32(total)}
	pt.lastLogTime.Store(time.Now().UnixNano())
	return pt
}

func (pt *progressTracker) recordCompletion(ctx context.Context, failed bool) {
	completed := pt.completed.Add(1)
	if failed {
		pt.failed.Add(1)
	}

	if pt.total < progressLogMinFeatures {
		return
	}

	pt.maybeLogProgress(ctx, completed)
}

func (pt *progressTracker) maybeLogProgress(ctx context.Context, completed int32) {
	lastLog := pt.lastLogTime.Load()
	now := time.Now().UnixNano()
	timeSinceLastLog := time.Duration(now - lastLog)

	shouldLogByBatch := completed%progressLogBatchSize == 0
	shouldLogByTime := timeSinceLastLog >= progressLogTimeInterval
	isComplete := completed >= pt.total

	if !shouldLogByBatch && !shouldLogByTime {
		return
	}
	if isComplete {
		return
	}

	if pt.lastLogTime.CompareAndSwap(lastLog, now) {
		slog.InfoContext(ctx, "phase 2 progress",
			"completed", completed,
			"total", pt.total,
			"failed", pt.failed.Load(),
		)
	}
}

func (uc *GenerateSpecViewUseCase) convertFeature(
	ctx context.Context,
	task featureTask,
	lang specview.Language,
	testIndexMap map[int]specview.TestInfo,
) ([]specview.BehaviorSpec, *specview.TokenUsage, int) {
	featureCtx, cancel := context.WithTimeout(ctx, DefaultPhase2FeatureTimeout)
	defer cancel()

	tests := make([]specview.TestForConversion, 0, len(task.feature.TestIndices))
	for _, idx := range task.feature.TestIndices {
		if testInfo, ok := testIndexMap[idx]; ok {
			tests = append(tests, specview.TestForConversion{
				Index: idx,
				Name:  testInfo.Name,
			})
		}
	}

	if len(tests) == 0 {
		return nil, nil, 0
	}

	input := specview.Phase2Input{
		DomainContext: task.domainContext,
		FeatureName:   task.feature.Name,
		Language:      lang,
		Tests:         tests,
	}

	output, usage, err := uc.aiProvider.ConvertTestNames(featureCtx, input)
	if err != nil {
		slog.WarnContext(ctx, "feature conversion failed, using fallback",
			"feature", task.feature.Name,
			"error", err,
		)
		return uc.generateFallbackBehaviors(tests), nil, 1
	}

	return output.Behaviors, usage, 0
}

func (uc *GenerateSpecViewUseCase) generateFallbackBehaviors(
	tests []specview.TestForConversion,
) []specview.BehaviorSpec {
	behaviors := make([]specview.BehaviorSpec, len(tests))
	for i, test := range tests {
		behaviors[i] = specview.BehaviorSpec{
			Confidence:  0.0,
			Description: test.Name,
			TestIndex:   test.Index,
		}
	}
	return behaviors
}

func (uc *GenerateSpecViewUseCase) assembleDocument(
	req specview.SpecViewRequest,
	modelID string,
	contentHash []byte,
	phase1Output *specview.Phase1Output,
	phase2Results []phase2Result,
	testIndexMap map[int]specview.TestInfo,
) *specview.SpecDocument {
	behaviorMap := make(map[int]map[int][]specview.BehaviorSpec)
	for _, r := range phase2Results {
		if behaviorMap[r.domainIdx] == nil {
			behaviorMap[r.domainIdx] = make(map[int][]specview.BehaviorSpec)
		}
		behaviorMap[r.domainIdx][r.featureIdx] = r.behaviors
	}

	domains := make([]specview.Domain, len(phase1Output.Domains))
	for di, domainGroup := range phase1Output.Domains {
		features := make([]specview.Feature, len(domainGroup.Features))
		for fi, featureGroup := range domainGroup.Features {
			var behaviors []specview.Behavior

			if featureBehaviors, ok := behaviorMap[di][fi]; ok {
				behaviors = make([]specview.Behavior, len(featureBehaviors))
				for bi, bs := range featureBehaviors {
					testCaseID := ""
					originalName := ""
					if testInfo, ok := testIndexMap[bs.TestIndex]; ok {
						testCaseID = testInfo.TestCaseID
						originalName = testInfo.Name
					}
					behaviors[bi] = specview.Behavior{
						Confidence:   bs.Confidence,
						Description:  bs.Description,
						OriginalName: originalName,
						TestCaseID:   testCaseID,
					}
				}
			}

			features[fi] = specview.Feature{
				Behaviors:   behaviors,
				Confidence:  featureGroup.Confidence,
				Description: featureGroup.Description,
				Name:        featureGroup.Name,
			}
		}

		domains[di] = specview.Domain{
			Confidence:  domainGroup.Confidence,
			Description: domainGroup.Description,
			Features:    features,
			Name:        domainGroup.Name,
		}
	}

	return &specview.SpecDocument{
		AnalysisID:  req.AnalysisID,
		ContentHash: contentHash,
		CreatedAt:   time.Now().UTC(),
		Domains:     domains,
		Language:    req.Language,
		ModelID:     modelID,
		UserID:      req.UserID,
	}
}

func buildTestIndexMap(files []specview.FileInfo) map[int]specview.TestInfo {
	m := make(map[int]specview.TestInfo)
	for _, f := range files {
		for _, t := range f.Tests {
			m[t.Index] = t
		}
	}
	return m
}

func countTotalTestCases(files []specview.FileInfo) int {
	count := 0
	for _, f := range files {
		count += len(f.Tests)
	}
	return count
}

func (uc *GenerateSpecViewUseCase) logExecutionError(
	ctx context.Context,
	analysisID string,
	phase string,
	startTime time.Time,
	err error,
) {
	durationMs := time.Since(startTime).Milliseconds()
	slog.ErrorContext(ctx, "specview execution failed",
		"analysis_id", analysisID,
		"phase", phase,
		"duration_ms", durationMs,
		"error", err,
	)
}

func (uc *GenerateSpecViewUseCase) logTokenUsage(
	ctx context.Context,
	analysisID string,
	phase1Usage *specview.TokenUsage,
	phase2Usage *specview.TokenUsage,
) {
	var phase1Prompt, phase1Candidates, phase1Total int32
	var phase1Model string
	if phase1Usage != nil {
		phase1Prompt = phase1Usage.PromptTokens
		phase1Candidates = phase1Usage.CandidatesTokens
		phase1Total = phase1Usage.TotalTokens
		phase1Model = phase1Usage.Model
	}

	var phase2Prompt, phase2Candidates, phase2Total int32
	if phase2Usage != nil {
		phase2Prompt = phase2Usage.PromptTokens
		phase2Candidates = phase2Usage.CandidatesTokens
		phase2Total = phase2Usage.TotalTokens
	}

	grandTotal := phase1Total + phase2Total

	slog.InfoContext(ctx, "specview_token_usage",
		"analysis_id", analysisID,
		"phase1_model", phase1Model,
		"phase1_prompt_tokens", phase1Prompt,
		"phase1_candidates_tokens", phase1Candidates,
		"phase1_total_tokens", phase1Total,
		"phase2_prompt_tokens", phase2Prompt,
		"phase2_candidates_tokens", phase2Candidates,
		"phase2_total_tokens", phase2Total,
		"grand_total_tokens", grandTotal,
	)
}
