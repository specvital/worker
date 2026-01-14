package specview

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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
	if err := req.Validate(); err != nil {
		return nil, err
	}

	modelID := req.ModelID
	if modelID == "" {
		modelID = uc.defaultModelID
	}

	files, err := uc.loadTestData(ctx, req.AnalysisID)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		slog.WarnContext(ctx, "no test files found",
			"analysis_id", req.AnalysisID,
		)
		return nil, fmt.Errorf("%w: no test files found for analysis", ErrLoadInventoryFailed)
	}

	contentHash := specview.GenerateContentHash(files, req.Language)

	existingDoc, err := uc.repository.FindDocumentByContentHash(ctx, contentHash, req.Language, modelID)
	if err != nil {
		return nil, fmt.Errorf("check cache: %w", err)
	}

	if existingDoc != nil {
		slog.InfoContext(ctx, "cache hit",
			"analysis_id", req.AnalysisID,
			"document_id", existingDoc.ID,
		)
		return &specview.SpecViewResult{
			CacheHit:    true,
			ContentHash: contentHash,
			DocumentID:  existingDoc.ID,
		}, nil
	}

	phase1Output, err := uc.executePhase1(ctx, files, req.Language)
	if err != nil {
		return nil, fmt.Errorf("%w: phase 1: %w", ErrAIProcessingFailed, err)
	}

	testIndexMap := buildTestIndexMap(files)

	phase2Results, err := uc.executePhase2(ctx, phase1Output, req.Language, testIndexMap)
	if err != nil {
		return nil, fmt.Errorf("%w: phase 2: %w", ErrAIProcessingFailed, err)
	}

	doc := uc.assembleDocument(req, modelID, contentHash, phase1Output, phase2Results, testIndexMap)

	if err := uc.repository.SaveDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSaveFailed, err)
	}

	slog.InfoContext(ctx, "document generated",
		"analysis_id", req.AnalysisID,
		"document_id", doc.ID,
		"domain_count", len(doc.Domains),
	)

	return &specview.SpecViewResult{
		CacheHit:    false,
		ContentHash: contentHash,
		DocumentID:  doc.ID,
	}, nil
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
) (*specview.Phase1Output, error) {
	phase1Ctx, cancel := context.WithTimeout(ctx, uc.config.Phase1Timeout)
	defer cancel()

	input := specview.Phase1Input{
		Files:    files,
		Language: lang,
	}

	output, err := uc.aiProvider.ClassifyDomains(phase1Ctx, input)
	if err != nil {
		return nil, err
	}

	slog.InfoContext(ctx, "phase 1 complete",
		"domain_count", len(output.Domains),
	)

	return output, nil
}

type phase2Result struct {
	domainIdx   int
	featureIdx  int
	behaviors   []specview.BehaviorSpec
	failedCount int
}

func (uc *GenerateSpecViewUseCase) executePhase2(
	ctx context.Context,
	phase1Output *specview.Phase1Output,
	lang specview.Language,
	testIndexMap map[int]specview.TestInfo,
) ([]phase2Result, error) {
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
		return nil, nil
	}

	var (
		results      = make([]phase2Result, len(featureTasks))
		resultsMu    sync.Mutex
		failedCount  int
		failedCountM sync.Mutex
	)

	g, gCtx := errgroup.WithContext(phase2Ctx)

	for i, task := range featureTasks {
		g.Go(func() error {
			if err := uc.phase2Sem.Acquire(gCtx, 1); err != nil {
				return err
			}
			defer uc.phase2Sem.Release(1)

			behaviors, failed := uc.convertFeature(gCtx, task, lang, testIndexMap)

			resultsMu.Lock()
			results[i] = phase2Result{
				domainIdx:   task.domainIdx,
				featureIdx:  task.featureIdx,
				behaviors:   behaviors,
				failedCount: failed,
			}
			resultsMu.Unlock()

			if failed > 0 {
				failedCountM.Lock()
				failedCount++
				failedCountM.Unlock()
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	failureRate := float64(failedCount) / float64(len(featureTasks))
	if failureRate > uc.config.FailureThreshold {
		return nil, fmt.Errorf("%w: %.0f%% features failed (threshold: %.0f%%)",
			ErrPartialFeatureFailure,
			failureRate*100,
			uc.config.FailureThreshold*100,
		)
	}

	slog.InfoContext(ctx, "phase 2 complete",
		"feature_count", len(featureTasks),
		"failed_count", failedCount,
	)

	return results, nil
}

type featureTask struct {
	domainContext string
	domainIdx     int
	feature       specview.FeatureGroup
	featureIdx    int
}

func (uc *GenerateSpecViewUseCase) convertFeature(
	ctx context.Context,
	task featureTask,
	lang specview.Language,
	testIndexMap map[int]specview.TestInfo,
) ([]specview.BehaviorSpec, int) {
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
		return nil, 0
	}

	input := specview.Phase2Input{
		DomainContext: task.domainContext,
		FeatureName:   task.feature.Name,
		Language:      lang,
		Tests:         tests,
	}

	output, err := uc.aiProvider.ConvertTestNames(featureCtx, input)
	if err != nil {
		slog.WarnContext(ctx, "feature conversion failed, using fallback",
			"feature", task.feature.Name,
			"error", err,
		)
		return uc.generateFallbackBehaviors(tests), 1
	}

	return output.Behaviors, 0
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
