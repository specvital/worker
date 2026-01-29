package specview

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/specvital/worker/internal/domain/specview"
)

// internalCacheStats tracks cache hit/miss statistics for Phase 2 behavior cache (internal use).
type internalCacheStats struct {
	cacheHits   int
	cacheMisses int
	totalTests  int
}

func (s *internalCacheStats) hitRate() float64 {
	if s.totalTests == 0 {
		return 0.0
	}
	return float64(s.cacheHits) / float64(s.totalTests)
}

func (s *internalCacheStats) toPublic() *specview.BehaviorCacheStats {
	if s == nil || s.totalTests == 0 {
		return nil
	}
	return &specview.BehaviorCacheStats{
		CachedBehaviors:    s.cacheHits,
		GeneratedBehaviors: s.cacheMisses,
		HitRate:            s.hitRate(),
		TotalBehaviors:     s.totalTests,
	}
}

const (
	DefaultPhase1Timeout        = 5 * time.Minute
	DefaultPhase2Timeout        = 25 * time.Minute
	DefaultPhase2Concurrency    = int64(5)
	DefaultFailureThreshold     = 0.5 // 50% feature failure threshold
	DefaultPhase2FeatureTimeout = 90 * time.Second  // 1m30s for single feature conversion

	// Progress logging thresholds for Phase 2
	progressLogBatchSize     = 10               // Log every N completions
	progressLogTimeInterval  = 30 * time.Second // Log at least every 30 seconds
	progressLogMinFeatures   = 10               // Only log progress when total >= this
	salvageCacheTimeout      = 10 * time.Second
)

// BatchConfig holds configuration for Batch API routing.
type BatchConfig struct {
	BatchThreshold int  // Minimum test count to trigger batch mode
	UseBatchAPI    bool // Enable Batch API for large jobs
}

// Config holds configuration for GenerateSpecViewUseCase.
type Config struct {
	BatchConfig       BatchConfig   // Batch API routing configuration
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

// WithBatchConfig sets the Batch API routing configuration.
func WithBatchConfig(batchCfg BatchConfig) Option {
	return func(cfg *Config) {
		cfg.BatchConfig = batchCfg
	}
}

// GenerateSpecViewUseCase orchestrates spec-view document generation.
type GenerateSpecViewUseCase struct {
	aiProvider     specview.AIProvider
	config         Config
	defaultModelID string
	repository     specview.Repository
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
	}
}

// ShouldUseBatchAPI determines whether the Batch API should be used based on test count.
func (uc *GenerateSpecViewUseCase) ShouldUseBatchAPI(testCount int) bool {
	return uc.config.BatchConfig.UseBatchAPI &&
		testCount >= uc.config.BatchConfig.BatchThreshold
}

// ExecutePhase2And3FromBatch processes Phase 2 and 3 using pre-classified domains from Batch API.
// This method reuses the standard Execute logic but skips Phase 1 since classification
// was already performed by the Batch API.
func (uc *GenerateSpecViewUseCase) ExecutePhase2And3FromBatch(
	ctx context.Context,
	req specview.SpecViewRequest,
	phase1Output *specview.Phase1Output,
) (*specview.SpecViewResult, error) {
	startTime := time.Now()

	if err := req.Validate(); err != nil {
		return nil, err
	}

	if phase1Output == nil {
		return nil, fmt.Errorf("%w: phase 1 output is required", specview.ErrInvalidInput)
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

	testIndexMap := buildTestIndexMap(files)

	phase2Results, internalStats, phase2Usage, err := uc.executePhase2(
		ctx,
		req.AnalysisID,
		phase1Output,
		req.Language,
		modelID,
		testIndexMap,
		files,
		req.ForceRegenerate,
	)
	if err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "phase2", startTime, err)
		return nil, fmt.Errorf("%w: phase 2: %w", ErrAIProcessingFailed, err)
	}

	// Log behavior cache stats
	if internalStats != nil && internalStats.totalTests > 0 {
		slog.InfoContext(ctx, "behavior cache stats (batch mode)",
			"analysis_id", req.AnalysisID,
			"total_tests", internalStats.totalTests,
			"cache_hits", internalStats.cacheHits,
			"cache_misses", internalStats.cacheMisses,
			"hit_rate", fmt.Sprintf("%.1f%%", internalStats.hitRate()*100),
		)
	}

	doc := uc.assembleDocument(req, modelID, contentHash, phase1Output, phase2Results, testIndexMap)

	// Phase 3: Executive summary generation (non-fatal)
	phase3Usage := uc.executePhase3(ctx, req.AnalysisID, doc)

	if err := uc.repository.SaveDocument(ctx, doc); err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "save", startTime, err)
		return nil, fmt.Errorf("%w: %w", ErrSaveFailed, err)
	}

	// Quota based on AI-generated behaviors only (cache hits are free)
	quotaAmount := internalStats.cacheMisses
	uc.recordUsageEvent(ctx, req.UserID, doc.ID, quotaAmount)
	uc.recordUserHistory(ctx, req.UserID, doc.ID)

	// Log token usage summary (Phase 1 is nil since it came from batch)
	uc.logTokenUsage(ctx, req.AnalysisID, nil, phase2Usage, phase3Usage)

	slog.InfoContext(ctx, "document generated (batch mode)",
		"analysis_id", req.AnalysisID,
		"user_id", req.UserID,
		"owner", analysisCtx.Owner,
		"repo", analysisCtx.Repo,
		"document_id", doc.ID,
		"domain_count", len(doc.Domains),
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	return &specview.SpecViewResult{
		AnalysisContext:    analysisCtx,
		BehaviorCacheStats: internalStats.toPublic(),
		CacheHit:           false,
		ContentHash:        contentHash,
		DocumentID:         doc.ID,
	}, nil
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

	phase1Output, phase1Usage, err := uc.executePhase1WithCache(
		ctx,
		files,
		req.Language,
		modelID,
		req.AnalysisID,
		req.ForceRegenerate,
	)
	if err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "phase1", startTime, err)
		return nil, fmt.Errorf("%w: phase 1: %w", ErrAIProcessingFailed, err)
	}

	testIndexMap := buildTestIndexMap(files)

	phase2Results, internalStats, phase2Usage, err := uc.executePhase2(
		ctx,
		req.AnalysisID,
		phase1Output,
		req.Language,
		modelID,
		testIndexMap,
		files,
		req.ForceRegenerate,
	)
	if err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "phase2", startTime, err)
		return nil, fmt.Errorf("%w: phase 2: %w", ErrAIProcessingFailed, err)
	}

	// Log behavior cache stats
	if internalStats != nil && internalStats.totalTests > 0 {
		slog.InfoContext(ctx, "behavior cache stats",
			"analysis_id", req.AnalysisID,
			"total_tests", internalStats.totalTests,
			"cache_hits", internalStats.cacheHits,
			"cache_misses", internalStats.cacheMisses,
			"hit_rate", fmt.Sprintf("%.1f%%", internalStats.hitRate()*100),
		)
	}

	doc := uc.assembleDocument(req, modelID, contentHash, phase1Output, phase2Results, testIndexMap)

	// Phase 3: Executive summary generation (non-fatal)
	phase3Usage := uc.executePhase3(ctx, req.AnalysisID, doc)

	if err := uc.repository.SaveDocument(ctx, doc); err != nil {
		uc.logExecutionError(ctx, req.AnalysisID, "save", startTime, err)
		return nil, fmt.Errorf("%w: %w", ErrSaveFailed, err)
	}

	// Quota based on AI-generated behaviors only (cache hits are free)
	quotaAmount := internalStats.cacheMisses
	uc.recordUsageEvent(ctx, req.UserID, doc.ID, quotaAmount)
	uc.recordUserHistory(ctx, req.UserID, doc.ID)

	// Log token usage summary
	uc.logTokenUsage(ctx, req.AnalysisID, phase1Usage, phase2Usage, phase3Usage)

	slog.InfoContext(ctx, "document generated",
		"analysis_id", req.AnalysisID,
		"user_id", req.UserID,
		"owner", analysisCtx.Owner,
		"repo", analysisCtx.Repo,
		"document_id", doc.ID,
		"domain_count", len(doc.Domains),
	)

	return &specview.SpecViewResult{
		AnalysisContext:    analysisCtx,
		BehaviorCacheStats: internalStats.toPublic(),
		CacheHit:           false,
		ContentHash:        contentHash,
		DocumentID:         doc.ID,
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

// executePhase1WithCache performs Phase 1 classification with incremental caching.
// Cache flow:
// - forceRegenerate=true: full classification, save cache
// - cache miss: full classification, save cache
// - cache hit + no changes: return cached output (zero token usage)
// - cache hit + deletions only: remove indices, update cache
// - cache hit + additions: call placement AI, fallback to Uncategorized on failure
func (uc *GenerateSpecViewUseCase) executePhase1WithCache(
	ctx context.Context,
	files []specview.FileInfo,
	lang specview.Language,
	modelID string,
	analysisID string,
	forceRegenerate bool,
) (*specview.Phase1Output, *specview.TokenUsage, error) {
	fileSignature := specview.GenerateFileSignature(files)

	// Skip cache lookup if forceRegenerate
	if forceRegenerate {
		slog.InfoContext(ctx, "classification cache bypassed (force regenerate)",
			"analysis_id", analysisID,
		)
		return uc.executePhase1AndSaveCache(ctx, files, lang, modelID, analysisID, fileSignature)
	}

	// Lookup classification cache
	cache, err := uc.repository.FindClassificationCache(ctx, fileSignature, lang, modelID)
	if err != nil {
		slog.WarnContext(ctx, "classification cache lookup failed, proceeding without cache",
			"analysis_id", analysisID,
			"error", err,
		)
		return uc.executePhase1AndSaveCache(ctx, files, lang, modelID, analysisID, fileSignature)
	}

	// Cache miss
	if cache == nil {
		slog.InfoContext(ctx, "classification cache miss",
			"analysis_id", analysisID,
		)
		return uc.executePhase1AndSaveCache(ctx, files, lang, modelID, analysisID, fileSignature)
	}

	// Cache hit - calculate diff
	diff := CalculateTestDiff(cache.TestIndexMap, files)

	// No changes
	if len(diff.NewTests) == 0 && len(diff.DeletedTests) == 0 {
		slog.InfoContext(ctx, "classification cache hit (no changes)",
			"analysis_id", analysisID,
			"domain_count", len(cache.ClassificationResult.Domains),
		)
		return cache.ClassificationResult, &specview.TokenUsage{}, nil
	}

	// Deletions only
	if len(diff.NewTests) == 0 && len(diff.DeletedTests) > 0 {
		slog.InfoContext(ctx, "classification cache hit (deletions only)",
			"analysis_id", analysisID,
			"deleted_count", len(diff.DeletedTests),
		)
		updatedOutput := RemoveDeletedTestIndices(cache.ClassificationResult, diff.DeletedTests)
		uc.updateClassificationCache(ctx, cache, updatedOutput, files)
		return updatedOutput, &specview.TokenUsage{}, nil
	}

	// Additions (with or without deletions) - call placement AI
	slog.InfoContext(ctx, "classification cache hit (additions detected)",
		"analysis_id", analysisID,
		"new_count", len(diff.NewTests),
		"deleted_count", len(diff.DeletedTests),
	)

	// First apply deletions if any
	baseOutput := cache.ClassificationResult
	if len(diff.DeletedTests) > 0 {
		baseOutput = RemoveDeletedTestIndices(cache.ClassificationResult, diff.DeletedTests)
	}

	// Call placement AI for new tests
	placementInput := specview.PlacementInput{
		ExistingStructure: baseOutput,
		Language:          lang,
		NewTests:          diff.NewTests,
	}

	placementOutput, usage, err := uc.aiProvider.PlaceNewTests(ctx, placementInput)
	if err != nil {
		slog.WarnContext(ctx, "placement AI failed, falling back to Uncategorized",
			"analysis_id", analysisID,
			"new_test_count", len(diff.NewTests),
			"error", err,
		)
		// Fallback: place all new tests in Uncategorized
		updatedOutput := placeAllToUncategorized(baseOutput, diff.NewTests)
		uc.updateClassificationCache(ctx, cache, updatedOutput, files)
		return updatedOutput, &specview.TokenUsage{}, nil
	}

	// Apply placements
	updatedOutput := applyPlacements(baseOutput, placementOutput.Placements)
	uc.updateClassificationCache(ctx, cache, updatedOutput, files)

	slog.InfoContext(ctx, "placement AI completed",
		"analysis_id", analysisID,
		"placed_count", len(placementOutput.Placements),
	)

	return updatedOutput, usage, nil
}

// executePhase1AndSaveCache runs full Phase 1 classification and saves the result to cache.
func (uc *GenerateSpecViewUseCase) executePhase1AndSaveCache(
	ctx context.Context,
	files []specview.FileInfo,
	lang specview.Language,
	modelID string,
	analysisID string,
	fileSignature []byte,
) (*specview.Phase1Output, *specview.TokenUsage, error) {
	output, usage, err := uc.executePhase1(ctx, files, lang, analysisID)
	if err != nil {
		return nil, nil, err
	}

	// Build and save cache (non-blocking on error)
	cache := &specview.ClassificationCache{
		ClassificationResult: output,
		FileSignature:        fileSignature,
		Language:             lang,
		ModelID:              modelID,
		TestIndexMap:         specview.BuildTestIndexMap(output, files),
	}

	if err := uc.repository.SaveClassificationCache(ctx, cache); err != nil {
		slog.WarnContext(ctx, "failed to save classification cache (non-critical)",
			"analysis_id", analysisID,
			"error", err,
		)
	}

	return output, usage, nil
}

// updateClassificationCache updates an existing cache with new classification result.
func (uc *GenerateSpecViewUseCase) updateClassificationCache(
	ctx context.Context,
	cache *specview.ClassificationCache,
	updatedOutput *specview.Phase1Output,
	files []specview.FileInfo,
) {
	cache.ClassificationResult = updatedOutput
	cache.TestIndexMap = specview.BuildTestIndexMap(updatedOutput, files)

	if err := uc.repository.SaveClassificationCache(ctx, cache); err != nil {
		slog.WarnContext(ctx, "failed to update classification cache (non-critical)",
			"cache_id", cache.ID,
			"error", err,
		)
	}
}

type phase2Result struct {
	domainIdx       int
	featureIdx      int
	behaviors       []specview.BehaviorSpec
	failedCount     int
	usage           *specview.TokenUsage
	newCacheEntries []specview.BehaviorCacheEntry
}

func (uc *GenerateSpecViewUseCase) executePhase2(
	ctx context.Context,
	analysisID string,
	phase1Output *specview.Phase1Output,
	lang specview.Language,
	modelID string,
	testIndexMap map[int]specview.TestInfo,
	files []specview.FileInfo,
	forceRegenerate bool,
) ([]phase2Result, *internalCacheStats, *specview.TokenUsage, error) {
	startTime := time.Now()

	phase2Ctx, cancel := context.WithTimeout(ctx, uc.config.Phase2Timeout)
	defer cancel()

	var featureTasks []featureTask
	totalTests := 0
	for di, domain := range phase1Output.Domains {
		for fi, feature := range domain.Features {
			featureTasks = append(featureTasks, featureTask{
				domainIdx:     di,
				domainContext: domain.Name + ": " + domain.Description,
				featureIdx:    fi,
				feature:       feature,
			})
			totalTests += len(feature.TestIndices)
		}
	}

	if len(featureTasks) == 0 {
		return nil, &internalCacheStats{}, nil, nil
	}

	// Build test -> filePath mapping for cache key generation
	testFilePathMap := buildTestFilePathMap(files)

	// Lookup behavior cache (skip if forceRegenerate)
	var cachedBehaviors map[string]string
	var testHashMap map[int]string
	cacheStats := &internalCacheStats{totalTests: totalTests}

	if !forceRegenerate {
		var err error
		cachedBehaviors, testHashMap, err = uc.lookupBehaviorCache(
			ctx,
			phase1Output,
			testIndexMap,
			testFilePathMap,
			lang,
			modelID,
		)
		if err != nil {
			slog.WarnContext(ctx, "behavior cache lookup failed, proceeding without cache",
				"error", err,
			)
			cachedBehaviors = make(map[string]string)
			testHashMap = make(map[int]string)
		}
		cacheStats.cacheHits = len(cachedBehaviors)
		cacheStats.cacheMisses = totalTests - cacheStats.cacheHits
	} else {
		cachedBehaviors = make(map[string]string)
		testHashMap = uc.buildTestHashMap(phase1Output, testIndexMap, testFilePathMap, lang, modelID)
		cacheStats.cacheMisses = totalTests
	}

	slog.InfoContext(ctx, "phase 2 started",
		"analysis_id", analysisID,
		"feature_count", len(featureTasks),
		"total_tests", totalTests,
		"cache_hits", cacheStats.cacheHits,
		"cache_misses", cacheStats.cacheMisses,
		"force_regenerate", forceRegenerate,
	)

	var (
		results   = make([]phase2Result, len(featureTasks))
		resultsMu sync.Mutex
		tracker   = newProgressTracker(len(featureTasks), analysisID)
	)

	// Per-job semaphore: prevents concurrent jobs from competing for shared slots
	phase2Sem := semaphore.NewWeighted(uc.config.Phase2Concurrency)

	g, gCtx := errgroup.WithContext(phase2Ctx)

	for i, task := range featureTasks {
		g.Go(func() error {
			if err := phase2Sem.Acquire(gCtx, 1); err != nil {
				return err
			}
			defer phase2Sem.Release(1)

			behaviors, usage, failed, newEntries := uc.convertFeatureWithCache(
				gCtx,
				task,
				lang,
				testIndexMap,
				testHashMap,
				cachedBehaviors,
			)

			resultsMu.Lock()
			results[i] = phase2Result{
				domainIdx:       task.domainIdx,
				featureIdx:      task.featureIdx,
				behaviors:       behaviors,
				failedCount:     failed,
				usage:           usage,
				newCacheEntries: newEntries,
			}
			resultsMu.Unlock()

			tracker.recordCompletion(ctx, failed > 0)

			return nil
		})
	}

	waitErr := g.Wait()
	if waitErr != nil {
		uc.salvageBehaviorCache(ctx, results)
		return nil, nil, nil, waitErr
	}

	failedCount := int(tracker.failed.Load())
	failureRate := float64(failedCount) / float64(len(featureTasks))
	if failureRate > uc.config.FailureThreshold {
		uc.salvageBehaviorCache(ctx, results)
		return nil, nil, nil, fmt.Errorf("%w: %.0f%% features failed (threshold: %.0f%%)",
			ErrPartialFeatureFailure,
			failureRate*100,
			uc.config.FailureThreshold*100,
		)
	}

	// Aggregate Phase 2 token usage and collect cache entries to save
	var aggregateUsage specview.TokenUsage
	var allNewCacheEntries []specview.BehaviorCacheEntry
	for _, r := range results {
		if r.usage != nil {
			aggregateUsage = aggregateUsage.Add(*r.usage)
		}
		allNewCacheEntries = append(allNewCacheEntries, r.newCacheEntries...)
	}

	// Save new cache entries (non-blocking on error)
	if len(allNewCacheEntries) > 0 {
		if err := uc.repository.SaveBehaviorCache(ctx, allNewCacheEntries); err != nil {
			slog.WarnContext(ctx, "failed to save behavior cache (non-critical)",
				"entry_count", len(allNewCacheEntries),
				"error", err,
			)
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	slog.InfoContext(ctx, "phase 2 complete",
		"analysis_id", analysisID,
		"feature_count", len(featureTasks),
		"failed_count", failedCount,
		"duration_ms", durationMs,
	)

	return results, cacheStats, &aggregateUsage, nil
}

// salvageBehaviorCache persists cache entries from successfully processed features even when
// Phase 2 fails, enabling cache hits on subsequent retries.
func (uc *GenerateSpecViewUseCase) salvageBehaviorCache(ctx context.Context, results []phase2Result) {
	var entries []specview.BehaviorCacheEntry
	for _, r := range results {
		entries = append(entries, r.newCacheEntries...)
	}
	if len(entries) == 0 {
		return
	}
	salvageCtx, cancel := context.WithTimeout(ctx, salvageCacheTimeout)
	defer cancel()
	if err := uc.repository.SaveBehaviorCache(salvageCtx, entries); err != nil {
		slog.WarnContext(ctx, "failed to salvage behavior cache",
			"entry_count", len(entries),
			"error", err,
		)
		return
	}
	slog.InfoContext(ctx, "salvaged behavior cache from partial results",
		"entry_count", len(entries),
	)
}

// buildTestFilePathMap creates a mapping from test index to file path.
func buildTestFilePathMap(files []specview.FileInfo) map[int]string {
	m := make(map[int]string)
	for _, f := range files {
		for _, t := range f.Tests {
			m[t.Index] = f.Path
		}
	}
	return m
}

// lookupBehaviorCache looks up cached behaviors for all tests in phase 1 output.
// Returns (cachedBehaviors map[hexHash]description, testHashMap map[testIndex]hexHash, error).
func (uc *GenerateSpecViewUseCase) lookupBehaviorCache(
	ctx context.Context,
	phase1Output *specview.Phase1Output,
	testIndexMap map[int]specview.TestInfo,
	testFilePathMap map[int]string,
	lang specview.Language,
	modelID string,
) (map[string]string, map[int]string, error) {
	testHashMap := uc.buildTestHashMap(phase1Output, testIndexMap, testFilePathMap, lang, modelID)

	// Collect all hashes for batch lookup
	var allHashes [][]byte
	hexToHash := make(map[string][]byte)
	for _, hexHash := range testHashMap {
		if _, exists := hexToHash[hexHash]; !exists {
			hash, err := hex.DecodeString(hexHash)
			if err != nil {
				continue // skip invalid hex (should not happen as we generate them)
			}
			allHashes = append(allHashes, hash)
			hexToHash[hexHash] = hash
		}
	}

	if len(allHashes) == 0 {
		return make(map[string]string), testHashMap, nil
	}

	// Batch lookup from repository
	cachedBehaviors, err := uc.repository.FindCachedBehaviors(ctx, allHashes)
	if err != nil {
		return nil, nil, err
	}

	return cachedBehaviors, testHashMap, nil
}

// buildTestHashMap generates hex-encoded cache key hashes for all tests.
func (uc *GenerateSpecViewUseCase) buildTestHashMap(
	phase1Output *specview.Phase1Output,
	testIndexMap map[int]specview.TestInfo,
	testFilePathMap map[int]string,
	lang specview.Language,
	modelID string,
) map[int]string {
	// Pre-calculate total tests for efficient map allocation
	totalTests := 0
	for _, domain := range phase1Output.Domains {
		for _, feature := range domain.Features {
			totalTests += len(feature.TestIndices)
		}
	}
	result := make(map[int]string, totalTests)

	for _, domain := range phase1Output.Domains {
		for _, feature := range domain.Features {
			for _, testIdx := range feature.TestIndices {
				testInfo, ok := testIndexMap[testIdx]
				if !ok {
					continue
				}
				filePath := testFilePathMap[testIdx]

				key := specview.BehaviorCacheKey{
					FilePath:  filePath,
					Language:  lang,
					ModelID:   modelID,
					SuitePath: testInfo.SuitePath,
					TestName:  testInfo.Name,
				}
				hash := specview.GenerateCacheKeyHash(key)
				result[testIdx] = hex.EncodeToString(hash)
			}
		}
	}
	return result
}

type featureTask struct {
	domainContext string
	domainIdx     int
	feature       specview.FeatureGroup
	featureIdx    int
}

// progressTracker tracks Phase 2 progress and handles batch logging.
type progressTracker struct {
	analysisID string
	completed  atomic.Int32
	failed     atomic.Int32
	lastLogTime atomic.Int64 // unix nano
	startTime  time.Time
	total      int32
}

func newProgressTracker(total int, analysisID string) *progressTracker {
	now := time.Now()
	pt := &progressTracker{
		analysisID: analysisID,
		startTime:  now,
		total:      int32(total),
	}
	pt.lastLogTime.Store(now.UnixNano())
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
		progressPct := float64(completed) / float64(pt.total) * 100
		elapsed := time.Since(pt.startTime)
		etaSeconds := int64(0)
		if completed > 0 {
			remaining := pt.total - completed
			etaSeconds = int64(elapsed.Seconds() / float64(completed) * float64(remaining))
		}

		slog.InfoContext(ctx, "phase 2 progress",
			"analysis_id", pt.analysisID,
			"completed", completed,
			"total", pt.total,
			"failed", pt.failed.Load(),
			"progress_pct", int(progressPct),
			"eta_seconds", etaSeconds,
		)
	}
}

// convertFeatureWithCache converts test names using AI, with behavior cache support.
// Returns: behaviors, token usage, failed count, new cache entries to save.
func (uc *GenerateSpecViewUseCase) convertFeatureWithCache(
	ctx context.Context,
	task featureTask,
	lang specview.Language,
	testIndexMap map[int]specview.TestInfo,
	testHashMap map[int]string,
	cachedBehaviors map[string]string,
) ([]specview.BehaviorSpec, *specview.TokenUsage, int, []specview.BehaviorCacheEntry) {
	featureCtx, cancel := context.WithTimeout(ctx, DefaultPhase2FeatureTimeout)
	defer cancel()

	// Separate cached vs uncached tests
	var cachedResults []specview.BehaviorSpec
	var uncachedTests []specview.TestForConversion

	for _, idx := range task.feature.TestIndices {
		testInfo, ok := testIndexMap[idx]
		if !ok {
			continue
		}

		hexHash, hasHash := testHashMap[idx]
		if hasHash {
			if cachedDesc, isCached := cachedBehaviors[hexHash]; isCached {
				// Use cached result
				cachedResults = append(cachedResults, specview.BehaviorSpec{
					Confidence:  1.0, // cached results are trusted
					Description: cachedDesc,
					TestIndex:   idx,
				})
				continue
			}
		}

		// Need AI call
		uncachedTests = append(uncachedTests, specview.TestForConversion{
			Index: idx,
			Name:  testInfo.Name,
		})
	}

	// If all tests are cached, return early (no AI call needed)
	if len(uncachedTests) == 0 {
		return cachedResults, nil, 0, nil
	}

	// AI call for uncached tests
	input := specview.Phase2Input{
		DomainContext: task.domainContext,
		FeatureName:   task.feature.Name,
		Language:      lang,
		Tests:         uncachedTests,
	}

	output, usage, err := uc.aiProvider.ConvertTestNames(featureCtx, input)
	if err != nil {
		slog.WarnContext(ctx, "feature conversion failed, using fallback",
			"feature", task.feature.Name,
			"error", err,
		)
		fallbackBehaviors := uc.generateFallbackBehaviors(uncachedTests)
		allBehaviors := append(cachedResults, fallbackBehaviors...)
		// Do not cache fallback behaviors (low quality)
		return allBehaviors, nil, 1, nil
	}

	// Prepare cache entries to save for successful AI conversions
	var newCacheEntries []specview.BehaviorCacheEntry
	for _, behavior := range output.Behaviors {
		if hexHash, ok := testHashMap[behavior.TestIndex]; ok {
			hash, err := hex.DecodeString(hexHash)
			if err != nil {
				continue // skip invalid hex (should not happen as we generate them)
			}
			newCacheEntries = append(newCacheEntries, specview.BehaviorCacheEntry{
				CacheKeyHash: hash,
				Description:  behavior.Description,
			})
		}
	}

	// Merge cached + AI results
	allBehaviors := append(cachedResults, output.Behaviors...)

	return allBehaviors, usage, 0, newCacheEntries
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

const (
	// DefaultPhase3Timeout is the timeout for Phase 3 executive summary generation.
	DefaultPhase3Timeout = 2 * time.Minute
)

// executePhase3 generates an executive summary from the assembled document.
// Phase 3 failure is non-fatal: logs a warning and continues without summary.
func (uc *GenerateSpecViewUseCase) executePhase3(
	ctx context.Context,
	analysisID string,
	doc *specview.SpecDocument,
) *specview.TokenUsage {
	phase3Start := time.Now()

	slog.InfoContext(ctx, "phase 3 started",
		"analysis_id", analysisID,
		"domain_count", len(doc.Domains),
	)

	phase3Ctx, cancel := context.WithTimeout(ctx, DefaultPhase3Timeout)
	defer cancel()

	input := specview.Phase3Input{
		Domains:  doc.Domains,
		Language: doc.Language,
	}

	output, usage, err := uc.aiProvider.GenerateSummary(phase3Ctx, input)
	if err != nil {
		slog.WarnContext(ctx, "phase 3 executive summary generation failed (non-fatal)",
			"analysis_id", analysisID,
			"duration_ms", time.Since(phase3Start).Milliseconds(),
			"error", err,
		)
		return nil
	}

	slog.InfoContext(ctx, "phase 3 complete",
		"analysis_id", analysisID,
		"duration_ms", time.Since(phase3Start).Milliseconds(),
		"summary_length", len(output.Summary),
	)

	doc.ExecutiveSummary = output.Summary
	return usage
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
	phase3Usage *specview.TokenUsage,
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

	var phase3Prompt, phase3Candidates, phase3Total int32
	var phase3Model string
	if phase3Usage != nil {
		phase3Prompt = phase3Usage.PromptTokens
		phase3Candidates = phase3Usage.CandidatesTokens
		phase3Total = phase3Usage.TotalTokens
		phase3Model = phase3Usage.Model
	}

	grandTotal := phase1Total + phase2Total + phase3Total

	slog.InfoContext(ctx, "specview_token_usage",
		"analysis_id", analysisID,
		"phase1_model", phase1Model,
		"phase1_prompt_tokens", phase1Prompt,
		"phase1_candidates_tokens", phase1Candidates,
		"phase1_total_tokens", phase1Total,
		"phase2_prompt_tokens", phase2Prompt,
		"phase2_candidates_tokens", phase2Candidates,
		"phase2_total_tokens", phase2Total,
		"phase3_model", phase3Model,
		"phase3_prompt_tokens", phase3Prompt,
		"phase3_candidates_tokens", phase3Candidates,
		"phase3_total_tokens", phase3Total,
		"grand_total_tokens", grandTotal,
	)
}
